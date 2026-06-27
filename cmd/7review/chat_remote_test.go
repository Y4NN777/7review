package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
