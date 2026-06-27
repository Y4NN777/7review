package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Y4NN777/7review/agent/tools"
	"github.com/Y4NN777/7review/agent/ui"
)

func TestReadSSEEmitsDeltas(t *testing.T) {
	body := strings.NewReader("data: {\"delta\":\"hello \"}\n\ndata: {\"delta\":\"engineer\"}\n\nevent: done\ndata: {}\n\n")
	var chunks []string
	err := readSSE(body, func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(chunks, ""); got != "hello engineer" {
		t.Fatalf("unexpected chunks: %q", got)
	}
}

func TestReadSSEAcceptsLargeModelChunk(t *testing.T) {
	large := strings.Repeat("x", 128*1024)
	body := strings.NewReader(`data: {"delta":"` + large + `"}` + "\n\nevent: done\ndata: {}\n\n")
	var chunks []string
	err := readSSE(body, func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(chunks, ""); got != large {
		t.Fatalf("unexpected large chunk length: got %d want %d", len(got), len(large))
	}
}

func TestReadSSERejectsOversizedEvent(t *testing.T) {
	body := strings.NewReader("data: " + strings.Repeat("x", maxSSEEventBytes+1) + "\n\n")
	err := readSSE(body, func(string) error { return nil })
	if err == nil {
		t.Fatal("expected oversized SSE error")
	}
	if !strings.Contains(err.Error(), "read SSE stream") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoteRunChatResponderStreamsFromServer(t *testing.T) {
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	responder := &remoteRunChatResponder{
		serverURL: "http://agent.test",
		runID:     "project!7",
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/chat/stream" || req.URL.Query().Get("run") != "project!7" {
				t.Fatalf("unexpected request URL: %s", req.URL.String())
			}
			if req.Header.Get("Authorization") != "Bearer agent-token" {
				t.Fatalf("missing auth header: %#v", req.Header)
			}
			body := "data: {\"delta\":\"review \"}\n\ndata: {\"delta\":\"ready\"}\n\nevent: done\ndata: {}\n\n"
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})},
	}
	var chunks []string
	err := responder.StreamRespond(context.Background(), "explain finding", func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(chunks, ""); got != "review ready" {
		t.Fatalf("unexpected stream: %q", got)
	}
}

func TestParseChatArgsAcceptsPositionalRunAndServer(t *testing.T) {
	opts, serverURL, runID := parseChatArgs([]string{"owner/repo!7", "--server", "http://agent/", "--plain"})
	if !opts.Plain || serverURL != "http://agent" || runID != "owner/repo!7" {
		t.Fatalf("unexpected chat args: opts=%#v server=%q run=%q", opts, serverURL, runID)
	}
}

func TestParseChatCommandFieldsSupportsQuotedValues(t *testing.T) {
	fields, err := parseChatCommandFields(`/approve --report-file "final report.md" --note 'lead approved' escaped\ value`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/approve", "--report-file", "final report.md", "--note", "lead approved", "escaped value"}
	if strings.Join(fields, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestParseChatCommandFieldsRejectsUnclosedQuote(t *testing.T) {
	if _, err := parseChatCommandFields(`/approve --report-file "final.md`); err == nil {
		t.Fatal("expected unclosed quote error")
	}
}

func TestChatCommandHandlerRendersRunHistory(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/run" || req.URL.Query().Get("id") != "owner/repo!7" {
			t.Fatalf("unexpected request: %s", req.URL.String())
		}
		return jsonResponse(http.StatusOK, `{"id":"owner/repo!7","status":"drafted","title":"Fix validation","events":[{"type":"chat_message","message":"first"},{"type":"chat_message","message":"second"},{"type":"status_changed","message":"drafted"}]}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "owner/repo!7", client)(context.Background(), "/history chat_message 1", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if !strings.Contains(out.String(), "history 1/3 events") || !strings.Contains(out.String(), "second") {
		t.Fatalf("history command output missing filtered event:\n%s", out.String())
	}
	if strings.Contains(out.String(), "first") || strings.Contains(out.String(), "status_changed") {
		t.Fatalf("history command output included excluded events:\n%s", out.String())
	}
}

func TestChatCommandHandlerRendersRunSummary(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"id":"owner/repo!7","provider":"github","project_id":"owner/repo","change_id":"7","status":"drafted","title":"Fix validation","web_url":"https://example.test/pr/7","events":[{"type":"run_started"}],"findings":[{"id":"F1"}],"draft_report":"draft","hil_approved":false}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "owner/repo!7", client)(context.Background(), "/run", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("expected command to be handled")
	}
	for _, want := range []string{"owner/repo!7  drafted", "provider github", "project  owner/repo", "findings 1", "history  1 events", "draft=5 bytes"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("run command output missing %q:\n%s", want, out.String())
		}
	}
}

func TestChatCommandHandlerRendersStatus(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/ready" {
			t.Fatalf("unexpected status request: %s %s", req.Method, req.URL.String())
		}
		return jsonResponse(http.StatusOK, `{"ready":true,"dependencies":{"headroom":"ok","mempalace":"ok","queue":"ok depth=0 capacity=8"},"queue":{"capacity":8,"available":8}}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "", client)(context.Background(), "/status", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"7review status http://agent", "agent", "headroom", "mempalace", "queue"} {
		if !handled || !strings.Contains(out.String(), want) {
			t.Fatalf("status command output missing %q handled=%t:\n%s", want, handled, out.String())
		}
	}
}

func TestChatCommandHandlerRendersStatusFailureView(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusServiceUnavailable, `{"ready":false,"dependencies":{"headroom":"down","mempalace":"ok"}}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "", client)(context.Background(), "/status", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	if !handled || !strings.Contains(out.String(), "headroom") || !strings.Contains(out.String(), "down") {
		t.Fatalf("status failure output missing dependency details handled=%t:\n%s", handled, out.String())
	}
}

func TestChatCommandHandlerRendersToolsCatalog(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/tools" {
			t.Fatalf("unexpected tools request: %s %s", req.Method, req.URL.String())
		}
		return jsonResponse(http.StatusOK, `[{"name":"list_runs","lifecycle_stage":"observe","implemented":true},{"name":"approve_run","lifecycle_stage":"hil","implemented":true,"requires_approval":true}]`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "", client)(context.Background(), "/tools", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"tools 2", "list_runs", "observe", "approve_run", "hil", "approval"} {
		if !handled || !strings.Contains(out.String(), want) {
			t.Fatalf("tools command output missing %q handled=%t:\n%s", want, handled, out.String())
		}
	}
}

func TestChatCommandHandlerPrintsDraftReport(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"id":"owner/repo!7","status":"drafted","draft_report":"draft body"}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "owner/repo!7", client)(context.Background(), "/draft", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	if !handled || !strings.Contains(out.String(), "agent: draft body") {
		t.Fatalf("unexpected draft command handled=%t out=%s", handled, out.String())
	}
}

func TestChatCommandHandlerWritesDraftReportToQuotedPath(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"id":"owner/repo!7","status":"drafted","draft_report":"draft body"}`), nil
	})}
	path := filepath.Join(t.TempDir(), "draft report.md")

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "owner/repo!7", client)(context.Background(), `/draft "`+path+`"`, &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !handled || string(data) != "draft body" || !strings.Contains(out.String(), "draft report written to") {
		t.Fatalf("unexpected draft write handled=%t data=%q out=%s", handled, string(data), out.String())
	}
}

func TestChatCommandHandlerApprovesRunFromReportFile(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "final report.md")
	if err := os.WriteFile(reportPath, []byte("approved final"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotBody string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/approve" || req.URL.Query().Get("run") != "owner/repo!7" {
			t.Fatalf("unexpected approval request: %s %s", req.Method, req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		gotBody = string(body)
		return jsonResponse(http.StatusAccepted, `{"accepted":true}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "owner/repo!7", client)(context.Background(), `/approve --report-file "`+reportPath+`"`, &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	if !handled || gotBody != "approved final" || !strings.Contains(out.String(), "approval queued for owner/repo!7") {
		t.Fatalf("unexpected approval command handled=%t body=%q out=%s", handled, gotBody, out.String())
	}
}

func TestChatCommandHandlerPublishFinalFromReportFile(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "final.md")
	if err := os.WriteFile(reportPath, []byte("approved final retry"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotBody string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/publish/final" || req.URL.Query().Get("run") != "owner/repo!7" {
			t.Fatalf("unexpected publish request: %s %s", req.Method, req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		gotBody = string(body)
		return jsonResponse(http.StatusAccepted, `{"accepted":true}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "owner/repo!7", client)(context.Background(), "/publish-final --report-file "+reportPath, &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	if !handled || gotBody != "approved final retry" || !strings.Contains(out.String(), "final publish queued for owner/repo!7") {
		t.Fatalf("unexpected publish command handled=%t body=%q out=%s", handled, gotBody, out.String())
	}
}

func TestChatCommandHandlerRequiresReportFileForHILCommands(t *testing.T) {
	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "owner/repo!7", operatorRequestHTTPClient())(context.Background(), "/approve", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if !handled || err == nil || !strings.Contains(err.Error(), "--report-file") {
		t.Fatalf("expected report-file error handled=%t err=%v", handled, err)
	}
}

func TestParseApprovalArgsAcceptsSpaceAndEqualsFlags(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "final.md")
	if err := os.WriteFile(reportPath, []byte("approved final"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts, err := parseApprovalArgs([]string{
		"--server", "http://agent",
		"--project=p",
		"--mr", "7",
		"--report-file", reportPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.serverURL != "http://agent" || opts.projectID != "p" || opts.mrIID != "7" || opts.report != "approved final" {
		t.Fatalf("unexpected approval options: %#v", opts)
	}
}

func TestParseApprovalArgsAcceptsRunID(t *testing.T) {
	opts, err := parseApprovalArgs([]string{
		"--server", "http://agent",
		"owner/repo!7",
		"--report=approved final",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.serverURL != "http://agent" || opts.runID != "owner/repo!7" || opts.approvalTarget() != "owner/repo!7" || opts.report != "approved final" {
		t.Fatalf("unexpected approval options: %#v", opts)
	}
}

func TestParseApprovalArgsAcceptsRunFlag(t *testing.T) {
	opts, err := parseApprovalArgs([]string{
		"--run=owner/repo!7",
		"--report=approved final",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.runID != "owner/repo!7" || opts.projectID != "" || opts.mrIID != "" {
		t.Fatalf("unexpected approval options: %#v", opts)
	}
}

func TestParsePublishArgsAcceptsPositionalRunAndInlineReport(t *testing.T) {
	opts, err := parsePublishArgs([]string{
		"--server", "http://agent",
		"p!7",
		"--report=final",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.serverURL != "http://agent" || opts.runID != "p!7" || opts.report != "final" {
		t.Fatalf("unexpected publish options: %#v", opts)
	}
}

func TestRequestAgentSendsMethodAndBody(t *testing.T) {
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/approve" || req.URL.Query().Get("project") != "p" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer agent-token" || req.Header.Get("X-7review-Token") != "agent-token" {
			t.Fatalf("missing auth headers: %#v", req.Header)
		}
		body, _ := io.ReadAll(req.Body)
		if string(body) != "final" {
			t.Fatalf("unexpected body %q", string(body))
		}
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Status:     "202 Accepted",
			Body:       io.NopCloser(strings.NewReader("queued")),
		}, nil
	})}
	out, err := requestAgent(client, http.MethodPost, "http://agent/approve?project=p", strings.NewReader("final"))
	if err != nil {
		t.Fatal(err)
	}
	if out != "queued" {
		t.Fatalf("unexpected response %q", out)
	}
}

func TestRunHealthcheckUsesDefaultHealthEndpoint(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.String() != "http://127.0.0.1:8080/health" {
			t.Fatalf("unexpected healthcheck request: %s %s", req.Method, req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})}
	if err := runHealthcheck(nil, client); err != nil {
		t.Fatal(err)
	}
}

func TestRunHealthcheckAcceptsServerOverride(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "http://agent:9090/health" {
			t.Fatalf("unexpected healthcheck URL: %s", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})}
	if err := runHealthcheck([]string{"--server", "http://agent:9090"}, client); err != nil {
		t.Fatal(err)
	}
}

func TestRunHealthcheckFailsOnUnhealthyResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Status:     "503 Service Unavailable",
			Body:       io.NopCloser(strings.NewReader("down")),
		}, nil
	})}
	err := runHealthcheck([]string{"--url", "http://agent/health"}, client)
	if err == nil || !strings.Contains(err.Error(), "503 Service Unavailable") {
		t.Fatalf("expected unhealthy response error, got %v", err)
	}
}

func TestOperatorHTTPClientsHaveBoundedTimeouts(t *testing.T) {
	if operatorRequestHTTPClient().Timeout != operatorRequestTimeout {
		t.Fatalf("unexpected request timeout: %s", operatorRequestHTTPClient().Timeout)
	}
	if operatorStreamHTTPClient().Timeout != operatorStreamTimeout {
		t.Fatalf("unexpected stream timeout: %s", operatorStreamHTTPClient().Timeout)
	}
	if operatorRequestTimeout <= 0 || operatorStreamTimeout <= operatorRequestTimeout {
		t.Fatalf("unexpected timeout ordering: request=%s stream=%s", operatorRequestTimeout, operatorStreamTimeout)
	}
}

func TestRemoteRunChatResponderDefaultsToBoundedClient(t *testing.T) {
	responder := &remoteRunChatResponder{serverURL: "://bad-url", runID: "run"}
	responder.httpClient = nil
	err := responder.StreamRespond(context.Background(), "hello", func(string) error { return nil })
	if err == nil {
		t.Fatal("expected request error for fake host")
	}
	if responder.httpClient == nil || responder.httpClient.Timeout != operatorStreamTimeout {
		t.Fatalf("expected bounded default stream client, got %#v", responder.httpClient)
	}
}

func TestParseStatusArgsEnablesRemoteStatus(t *testing.T) {
	opts := parseStatusArgs([]string{"--plain", "--server", "http://agent"})
	if !opts.remote || !opts.plain || opts.serverURL != "http://agent" {
		t.Fatalf("unexpected status options: %#v", opts)
	}
}

func TestRemoteStatusViewRendersReadyDependenciesAndQueue(t *testing.T) {
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.String() != "http://agent/ready" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer agent-token" {
			t.Fatalf("missing auth header: %#v", req.Header)
		}
		body := `{"ready":true,"dependencies":{"headroom":"ok","mempalace":"ok","queue":"ok depth=1 capacity=8"},"queue":{"depth":1,"capacity":8,"available":7,"enqueued":4,"completed":3,"failed":1}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
	view, ready, err := remoteStatusView(client, statusCommandOptions{serverURL: "http://agent", plain: true, remote: true})
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Fatalf("expected ready status: %#v", view)
	}
	out := ui.RenderStatus(view)
	for _, want := range []string{"7review status http://agent", "agent", "http=200", "headroom", "mempalace", "queue", "available=7", "failed=1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestRemoteStatusViewRendersDependencyFailureBody(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"ready":false,"dependencies":{"headroom":"headroom check failed","mempalace":"ok","queue":"ok depth=0 capacity=2"},"queue":{"capacity":2,"available":2}}`
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Status:     "503 Service Unavailable",
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
	view, ready, err := remoteStatusView(client, statusCommandOptions{serverURL: "http://agent", plain: true, remote: true})
	if err == nil {
		t.Fatal("expected non-ready HTTP error")
	}
	if ready {
		t.Fatalf("expected non-ready status: %#v", view)
	}
	out := ui.RenderStatus(view)
	for _, want := range []string{"agent", "down", "http=503", "headroom", "headroom check failed", "mempalace", "ok"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestRunStatusRemoteReturnsErrorWhenAgentNotReady(t *testing.T) {
	var out bytes.Buffer
	err := runStatus([]string{"--server", "://bad-url", "--plain"}, &out)
	if err == nil {
		t.Fatal("expected status error")
	}
	if !strings.Contains(out.String(), "agent") || !strings.Contains(out.String(), "down") {
		t.Fatalf("expected rendered down status, got:\n%s", out.String())
	}
}

func TestParseTUIArgsAcceptsRunServerAndPlain(t *testing.T) {
	opts := parseTUIArgs([]string{"owner/repo!7", "--server", "http://agent", "--plain", "--watch", "--refresh", "5s"})
	if opts.runID != "owner/repo!7" || opts.serverURL != "http://agent" || !opts.plain || !opts.watch || opts.refreshEvery != 5*time.Second || opts.clearOnRefresh {
		t.Fatalf("unexpected tui options: %#v", opts)
	}
}

func TestParseHistoryArgsAcceptsRunServerAndPositional(t *testing.T) {
	opts := parseHistoryArgs([]string{"owner/repo!7", "--server", "http://agent", "--type", "chat_message", "--limit", "3"})
	if opts.runID != "owner/repo!7" || opts.serverURL != "http://agent" || opts.eventType != "chat_message" || opts.limit != 3 {
		t.Fatalf("unexpected history options: %#v", opts)
	}
}

func TestRenderRunHistoryFormatsTimeline(t *testing.T) {
	out := renderRunHistory(remoteRunDetail{
		ID:     "owner/repo!7",
		Status: "drafted",
		Title:  "Fix checkout",
		Events: []remoteRunEvent{{
			At:      time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
			Type:    "chat_message",
			Status:  "drafted",
			Message: "explain F1",
			Meta:    map[string]string{"role": "engineer"},
		}},
	}, historyCommandOptions{})
	for _, want := range []string{"owner/repo!7  drafted", "Fix checkout", "history 1/1 events", "2026-06-27T12:00:00Z", "chat_message", "role=engineer"} {
		if !strings.Contains(out, want) {
			t.Fatalf("history output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderRunHistoryFiltersAndLimitsEvents(t *testing.T) {
	out := renderRunHistory(remoteRunDetail{
		ID:     "owner/repo!7",
		Status: "drafted",
		Events: []remoteRunEvent{
			{Type: "run_started", Message: "start"},
			{Type: "chat_message", Message: "first"},
			{Type: "chat_message", Message: "second"},
		},
	}, historyCommandOptions{eventType: "chat_message", limit: 1})
	if !strings.Contains(out, "history 1/3 events") || !strings.Contains(out, "second") {
		t.Fatalf("filtered history missing latest chat event:\n%s", out)
	}
	if strings.Contains(out, "run_started") || strings.Contains(out, "first") {
		t.Fatalf("filtered history included excluded events:\n%s", out)
	}
}

func TestParseRefreshIntervalAcceptsSecondsAndDuration(t *testing.T) {
	if got := parseRefreshInterval("3", time.Second); got != 3*time.Second {
		t.Fatalf("unexpected seconds interval: %s", got)
	}
	if got := parseRefreshInterval("250ms", time.Second); got != 250*time.Millisecond {
		t.Fatalf("unexpected duration interval: %s", got)
	}
	if got := parseRefreshInterval("bad", 2*time.Second); got != 2*time.Second {
		t.Fatalf("expected fallback interval, got %s", got)
	}
}

func TestRemoteConsoleViewUsesAgentEndpoints(t *testing.T) {
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	var toolNames []string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer agent-token" {
			t.Fatalf("missing auth header on %s", req.URL.String())
		}
		switch req.URL.Path {
		case "/ready":
			return jsonResponse(http.StatusOK, `{"ready":true,"dependencies":{"headroom":"ok","mempalace":"ok","queue":"ok depth=1 capacity=8"},"queue":{"depth":1,"capacity":8,"available":7,"enqueued":4,"completed":3,"failed":0}}`), nil
		case "/tools":
			return jsonResponse(http.StatusOK, `[{"name":"list_runs","lifecycle_stage":"observe","implemented":true},{"name":"approve_run","lifecycle_stage":"hil","implemented":true,"requires_approval":true}]`), nil
		case "/tools/execute":
			body, _ := io.ReadAll(req.Body)
			var call tools.ExecuteRequest
			if err := json.Unmarshal(body, &call); err != nil {
				t.Fatal(err)
			}
			toolNames = append(toolNames, call.Name)
			switch call.Name {
			case "list_runs":
				return jsonResponse(http.StatusOK, `{"name":"list_runs","result":[{"id":"owner/repo!7","provider":"github","project_id":"owner/repo","change_id":"7","title":"Fix validation","status":"drafted","updated_at":"2026-06-27T12:00:00Z"}]}`), nil
			case "list_provider_status":
				return jsonResponse(http.StatusOK, `{"name":"list_provider_status","result":{"mode":"orchestrator","providers":[{"name":"openrouter","configured":true}],"roles":[{"role":"reasoner","primary":"openrouter/deepseek","max_tokens":4096}]}}`), nil
			case "list_skills":
				return jsonResponse(http.StatusOK, `{"name":"list_skills","result":[{"name":"traceability-review","path":"agent/skills/traceability-review/SKILL.md","loaded":true}]}`), nil
			case "get_run":
				return jsonResponse(http.StatusOK, `{"name":"get_run","result":{"id":"owner/repo!7","provider":"github","project_id":"owner/repo","change_id":"7","title":"Fix validation","status":"drafted","updated_at":"2026-06-27T12:00:00Z","event_count":3,"events":[{"type":"run_started","status":"running"},{"type":"status_changed","status":"drafted"}],"findings":[{"id":"F-1"}],"draft_report":"draft"}}`), nil
			default:
				t.Fatalf("unexpected tool call %q", call.Name)
			}
		}
		t.Fatalf("unexpected request path %s", req.URL.Path)
		return nil, nil
	})}
	view, err := remoteConsoleView(client, tuiCommandOptions{serverURL: "http://agent", plain: true, watch: true, refreshEvery: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	out := ui.RenderConsole(view)
	for _, want := range []string{"owner/repo!7", "Fix validation", "history    3 events", "latest     status_changed drafted", "openrouter", "traceability-review", "tools     2", "refresh 5s", "watch every 5s"} {
		if !strings.Contains(out, want) {
			t.Fatalf("console output missing %q:\n%s", want, out)
		}
	}
	for _, want := range []string{"list_runs", "list_provider_status", "list_skills", "get_run"} {
		if !containsString(toolNames, want) {
			t.Fatalf("missing tool call %q in %#v", want, toolNames)
		}
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
