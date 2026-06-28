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

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/review"
	"github.com/Y4NN777/7review/agent/tools"
	"github.com/Y4NN777/7review/agent/ui"
	tea "github.com/charmbracelet/bubbletea"
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

func TestRemoteRunChatResponderStreamsOperatorChatWithoutRun(t *testing.T) {
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	responder := &remoteRunChatResponder{
		serverURL: "http://agent.test",
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/chat/stream" || req.URL.RawQuery != "" {
				t.Fatalf("unexpected request URL: %s", req.URL.String())
			}
			body := "data: {\"delta\":\"operator ready\"}\n\nevent: done\ndata: {}\n\n"
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})},
	}
	var chunks []string
	err := responder.StreamRespond(context.Background(), "hello", func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(chunks, ""); got != "operator ready" {
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

func TestChatCommandHandlerRendersProviderStatus(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/tools/execute" {
			t.Fatalf("unexpected provider request: %s %s", req.Method, req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"name":"list_provider_status"`) {
			t.Fatalf("unexpected provider request body: %s", string(body))
		}
		return jsonResponse(http.StatusOK, `{"name":"list_provider_status","result":{"mode":"orchestrator","active_provider":"openrouter","providers":[{"name":"openrouter","configured":true},{"name":"ollama","configured":false,"reason":"missing URL"}],"roles":[{"role":"reasoner","primary":"deepseek@openrouter","fallbacks":["qwen@ollama"],"parallel":true,"max_parallel":2}]}}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "", client)(context.Background(), "/providers", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"providers", "mode     orchestrator", "active   openrouter", "openrouter", "configured", "ollama", "missing URL", "reasoner", "deepseek@openrouter", "fallback=qwen@ollama", "parallel=2"} {
		if !handled || !strings.Contains(out.String(), want) {
			t.Fatalf("providers command output missing %q handled=%t:\n%s", want, handled, out.String())
		}
	}
}

func TestChatCommandHandlerRendersConfigStatus(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/tools/execute" {
			t.Fatalf("unexpected config request: %s %s", req.Method, req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"name":"get_config_status"`) {
			t.Fatalf("unexpected config request body: %s", string(body))
		}
		return jsonResponse(http.StatusOK, `{"name":"get_config_status","result":{"listen_addr":":8080","corpus_root":"/repo","max_supporting_corpus_sections":5,"memory_dir":"/data","hil_channel":"manual","provider":"openrouter","review_model":"deepseek-chat","small_model":"gpt-4o-mini","orchestrator_config":"./orchestrator.yaml","has_github":true,"has_gitlab":false,"has_openrouter":true,"has_deepseek":true,"headroom_url":"http://headroom:8787","mempalace_url":"http://mempalace:8788","webhook_workers":4,"webhook_queue_size":32}}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "", client)(context.Background(), "/config", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"config", "listen    :8080", "provider  openrouter", "review    deepseek-chat", "support  5", "headroom  http://headroom:8787", "mempalace http://mempalace:8788", "workers  4 queue=32", "github=true gitlab=false", "openrouter=true deepseek=true"} {
		if !handled || !strings.Contains(out.String(), want) {
			t.Fatalf("config command output missing %q handled=%t:\n%s", want, handled, out.String())
		}
	}
	for _, secret := range []string{"token", "secret", "api_key"} {
		if strings.Contains(strings.ToLower(out.String()), secret) {
			t.Fatalf("config command leaked secret-looking field %q:\n%s", secret, out.String())
		}
	}
}

func TestChatCommandHandlerRendersSkillStatus(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/tools/execute" {
			t.Fatalf("unexpected skills request: %s %s", req.Method, req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"name":"list_skills"`) {
			t.Fatalf("unexpected skills request body: %s", string(body))
		}
		return jsonResponse(http.StatusOK, `{"name":"list_skills","result":[{"name":"traceability-review","path":"agent/skills/traceability-review/SKILL.md","loaded":true},{"name":"framework-rules-review","path":"agent/skills/framework-rules-review/SKILL.md","loaded":false}]}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "", client)(context.Background(), "/skills", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"skills 2", "traceability-review", "loaded", "framework-rules-review", "off", "SKILL.md"} {
		if !handled || !strings.Contains(out.String(), want) {
			t.Fatalf("skills command output missing %q handled=%t:\n%s", want, handled, out.String())
		}
	}
}

func TestChatCommandHandlerRendersSessions(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/tools/execute" {
			t.Fatalf("unexpected sessions request: %s %s", req.Method, req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"name":"list_runs"`) {
			t.Fatalf("unexpected sessions request body: %s", string(body))
		}
		return jsonResponse(http.StatusOK, `{"name":"list_runs","result":[{"id":"owner/repo!6","provider":"github","project_id":"owner/repo","change_id":"6","title":"Older change","status":"finalized","updated_at":"2026-06-27T11:00:00Z","event_count":2,"hil_approved":true},{"id":"owner/repo!7","provider":"github","project_id":"owner/repo","change_id":"7","title":"Fix validation","status":"drafted","updated_at":"2026-06-27T12:00:00Z","event_count":3}]}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "", client)(context.Background(), "/sessions validation 1", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sessions 1/2", "query=validation", "limit=1", "owner/repo!7", "Fix validation", "history=3", "change owner/repo!7"} {
		if !handled || !strings.Contains(out.String(), want) {
			t.Fatalf("sessions command output missing %q handled=%t:\n%s", want, handled, out.String())
		}
	}
	if strings.Contains(out.String(), "owner/repo!6") {
		t.Fatalf("sessions filter included non-matching run:\n%s", out.String())
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

func TestChatCommandHandlerRendersMemoryProposal(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/tools/execute" {
			t.Fatalf("unexpected memory request: %s %s", req.Method, req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"name":"preview_memory_proposal"`) || !strings.Contains(string(body), `"run":"owner/repo!7"`) {
			t.Fatalf("unexpected memory request body: %s", string(body))
		}
		return jsonResponse(http.StatusOK, `{"name":"preview_memory_proposal","result":{"run":"owner/repo!7","approved":true,"final_bytes":42,"proposal":{"Conventions":["final report convention"],"Decisions":["human decision"],"Vectors":[{"ID":"v1","Text":"vector text"}]}}}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "owner/repo!7", client)(context.Background(), "/memory", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"memory owner/repo!7", "approved true", "final_bytes 42", "conventions 1 decisions 1 vectors 1", "final report convention", "human decision", "vector v1"} {
		if !handled || !strings.Contains(out.String(), want) {
			t.Fatalf("memory command output missing %q handled=%t:\n%s", want, handled, out.String())
		}
	}
}

func TestChatCommandHandlerRendersDiffSummary(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/tools/execute" {
			t.Fatalf("unexpected diff request: %s %s", req.Method, req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"name":"get_diff_summary"`) || !strings.Contains(string(body), `"run":"owner/repo!7"`) {
			t.Fatalf("unexpected diff request body: %s", string(body))
		}
		return jsonResponse(http.StatusOK, `{"name":"get_diff_summary","result":{"run":"owner/repo!7","file_count":2,"total_tokens":123,"additions":10,"deletions":3,"changed_files":[{"path":"api/orders.go","status":"modified","additions":8,"deletions":2,"has_patch":true},{"old_path":"old.go","path":"new.go","status":"renamed","additions":2,"deletions":1,"has_patch":false}]}}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "owner/repo!7", client)(context.Background(), "/diff", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"diff owner/repo!7", "files 2 tokens 123 +10 -3", "changed", "modified", "api/orders.go", "old.go -> new.go", "no-patch"} {
		if !handled || !strings.Contains(out.String(), want) {
			t.Fatalf("diff command output missing %q handled=%t:\n%s", want, handled, out.String())
		}
	}
}

func TestChatCommandHandlerRendersSelectedContext(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/tools/execute" {
			t.Fatalf("unexpected context request: %s %s", req.Method, req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		var call tools.ExecuteRequest
		if err := json.Unmarshal(body, &call); err != nil {
			t.Fatal(err)
		}
		if call.Name != "get_selected_context" || call.Input["run"] != "owner/repo!7" {
			t.Fatalf("unexpected context tool call: %#v", call)
		}
		return jsonResponse(http.StatusOK, `{"name":"get_selected_context","result":{"run":"owner/repo!7","corpus_sections":[{"path":"docs/SRS.md","title":"REQ-12","kind":"requirement","selection_reason":"seed: identifier REQ-12"},{"path":"docs/openapi.yaml","title":"schemas.Session","kind":"interface","selection_reason":"interface_trace: /sessions -> schemas.Session"}],"evidence_manifest":[{"source":"docs/SRS.md","heading_or_key":"REQ-12","kind":"requirement","authority":"requirement","matched_signals":["REQ-12"],"selection_reason":"seed: identifier REQ-12","score":920,"content_bytes":120},{"source":"docs/openapi.yaml","heading_or_key":"schemas.Session","kind":"interface","authority":"contract","matched_signals":["/sessions","Session"],"selection_reason":"interface_trace: /sessions -> schemas.Session","score":870,"content_bytes":220}],"skill_sections":[{"path":"agent/skills/api-contract-review/SKILL.md","title":"API contract review","kind":"skill","selection_reason":"language Go"}],"warnings":["selected context was compacted"]}}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "owner/repo!7", client)(context.Background(), "/context", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"context owner/repo!7",
		"corpus 2 evidence 2 skills 1",
		"docs/SRS.md#REQ-12",
		"seed: identifier REQ-12",
		"docs/openapi.yaml#schemas.Session",
		"interface_trace: /sessions -> schemas.Session",
		"signals /sessions, Session",
		"api-contract-review",
		"selected context was compacted",
	} {
		if !handled || !strings.Contains(out.String(), want) {
			t.Fatalf("context command output missing %q handled=%t:\n%s", want, handled, out.String())
		}
	}
}

func TestChatCommandHandlerAcceptsContextAlias(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		var call tools.ExecuteRequest
		if err := json.Unmarshal(body, &call); err != nil {
			t.Fatal(err)
		}
		if call.Name != "get_selected_context" {
			t.Fatalf("alias should dispatch to selected context tool, got %q", call.Name)
		}
		return jsonResponse(http.StatusOK, `{"name":"get_selected_context","result":{"run":"owner/repo!7"}}`), nil
	})}

	var out strings.Builder
	handled, err := chatCommandHandlerWithClient("http://agent", "owner/repo!7", client)(context.Background(), "/evidence", &out, ui.ChatContext{}, ui.ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	if !handled || !strings.Contains(out.String(), "context owner/repo!7") {
		t.Fatalf("context alias output unexpected handled=%t:\n%s", handled, out.String())
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

func TestRunSessionsUsesListRunsTool(t *testing.T) {
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.String() != "http://agent/tools/execute" {
			t.Fatalf("unexpected sessions request: %s %s", req.Method, req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer agent-token" {
			t.Fatalf("missing auth header: %#v", req.Header)
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"name":"list_runs"`) {
			t.Fatalf("unexpected sessions request body: %s", string(body))
		}
		return jsonResponse(http.StatusOK, `{"name":"list_runs","result":[{"id":"owner/repo!7","provider":"github","project_id":"owner/repo","change_id":"7","title":"Fix validation","status":"drafted","updated_at":"2026-06-27T12:00:00Z","event_count":3},{"id":"group/repo!8","provider":"gitlab","project_id":"group/repo","change_id":"8","title":"Ship report","status":"drafted","updated_at":"2026-06-27T12:05:00Z","event_count":1},{"id":"owner/repo!6","provider":"github","project_id":"owner/repo","change_id":"6","title":"Done","status":"finalized","updated_at":"2026-06-27T11:00:00Z","event_count":2}]}`), nil
	})}

	var out strings.Builder
	if err := runSessions([]string{"--server", "http://agent", "--status", "drafted", "--provider", "github", "--query", "validation", "--limit", "1"}, &out, client); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sessions 1/3", "status=drafted", "provider=github", "query=validation", "limit=1", "owner/repo!7", "github", "drafted", "Fix validation", "change owner/repo!7"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("sessions output missing %q:\n%s", want, out.String())
		}
	}
	for _, notWant := range []string{"group/repo!8", "owner/repo!6"} {
		if strings.Contains(out.String(), notWant) {
			t.Fatalf("sessions filter included %q:\n%s", notWant, out.String())
		}
	}
}

func TestRunSessionRendersReadableRunDetail(t *testing.T) {
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.String() != "http://agent/run?id=owner%2Frepo%217" {
			t.Fatalf("unexpected session request: %s %s", req.Method, req.URL.String())
		}
		if req.Header.Get("Authorization") != "Bearer agent-token" {
			t.Fatalf("missing auth header: %#v", req.Header)
		}
		return jsonResponse(http.StatusOK, `{"id":"owner/repo!7","provider":"github","project_id":"owner/repo","change_id":"7","status":"drafted","title":"Fix validation","web_url":"https://example.test/pr/7","events":[{"type":"run_started"},{"type":"chat_message","message":"first"},{"type":"status_changed","message":"drafted"},{"type":"chat_message","message":"explain F1"}],"findings":[{"id":"F1"}],"draft_report":"draft"}`), nil
	})}

	var out strings.Builder
	if err := runSession([]string{"owner/repo!7", "--server", "http://agent", "--type", "chat_message", "--limit", "1"}, &out, client); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"owner/repo!7  drafted", "Fix validation", "provider github", "findings 1", "history  4 events", "draft=5 bytes", "recent 1/4 events", "chat_message  explain F1", "commands", "7review chat --run owner/repo!7 --server http://agent", "7review history owner/repo!7 --type chat_message --limit 20 --server http://agent", "7review approve --run owner/repo!7 --report-file final.md --server http://agent"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("session output missing %q:\n%s", want, out.String())
		}
	}
	for _, notWant := range []string{"run_started", "status_changed", "first"} {
		if strings.Contains(out.String(), notWant) {
			t.Fatalf("session event filter included %q:\n%s", notWant, out.String())
		}
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

func TestParseTUIArgsDefaultsToLiveAndSupportsOnce(t *testing.T) {
	live := parseTUIArgs(nil)
	if !live.watch || live.once || !live.clearOnRefresh {
		t.Fatalf("tui should default to live dashboard: %#v", live)
	}
	once := parseTUIArgs([]string{"--once"})
	if once.watch || !once.once || once.clearOnRefresh {
		t.Fatalf("tui --once should render one snapshot: %#v", once)
	}
}

func TestConsoleTUIModelHandlesInteractiveKeys(t *testing.T) {
	model := newConsoleTUIModel(nil, tuiCommandOptions{serverURL: "http://agent", refreshEvery: time.Second})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model = updated.(consoleTUIModel)
	if !model.help || cmd != nil || !strings.Contains(model.View(), "/history chat_message 20") {
		t.Fatalf("help key did not toggle help view: model=%#v cmd=%v view=%s", model, cmd, model.View())
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model = updated.(consoleTUIModel)
	if !model.loading || cmd == nil {
		t.Fatalf("refresh key did not schedule fetch: model=%#v cmd=%v", model, cmd)
	}

	updated, cmd = model.Update(consoleViewMsg{view: ui.ConsoleView{Server: "http://agent", Ready: true, Watch: true, RefreshEvery: time.Second}})
	model = updated.(consoleTUIModel)
	if model.loading || model.err != nil || cmd == nil || !strings.Contains(model.View(), "7review") {
		t.Fatalf("view update did not render dashboard and schedule tick: model=%#v cmd=%v view=%s", model, cmd, model.View())
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updated.(consoleTUIModel)
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model = updated.(consoleTUIModel)
	if model.input != "/s" || cmd != nil || !strings.Contains(model.View(), "> /s") {
		t.Fatalf("typing did not update command input: model=%#v cmd=%v view=%s", model, cmd, model.View())
	}
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	model = updated.(consoleTUIModel)
	if model.input != "/" {
		t.Fatalf("backspace did not edit command input: %q", model.input)
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(consoleTUIModel)
	if !model.commandRunning || cmd == nil || len(model.transcript) == 0 || model.transcript[len(model.transcript)-1].Text != "/help" {
		t.Fatalf("enter should schedule command and append user transcript: model=%#v cmd=%v", model, cmd)
	}
	updated, cmd = model.Update(consoleCommandMsg{input: "/", out: "agent: command output"})
	model = updated.(consoleTUIModel)
	if model.commandRunning || cmd == nil || !strings.Contains(model.View(), "agent> command output") || strings.Contains(model.View(), "agent> agent:") {
		t.Fatalf("command result should append to transcript and refresh: model=%#v cmd=%v view=%s", model, cmd, model.View())
	}

	streaming := consoleTUIModel{
		opts:           tuiCommandOptions{serverURL: "http://agent", refreshEvery: time.Second},
		view:           ui.ConsoleView{Server: "http://agent", Ready: true, Watch: true, RefreshEvery: time.Second},
		commandRunning: true,
		stream:         make(chan consoleStreamMsg),
		transcript: []ui.ConsoleTranscriptItem{
			{Role: "you", Text: "explain finding"},
			{Role: "agent"},
		},
	}
	updated, cmd = streaming.Update(consoleStreamMsg{delta: "streamed "})
	streaming = updated.(consoleTUIModel)
	if cmd == nil || !strings.Contains(streaming.View(), "agent> streamed") {
		t.Fatalf("stream delta should append to agent transcript: cmd=%v view=%s", cmd, streaming.View())
	}
	updated, cmd = streaming.Update(consoleStreamMsg{delta: "reply"})
	streaming = updated.(consoleTUIModel)
	if cmd == nil || !strings.Contains(streaming.View(), "streamed reply") {
		t.Fatalf("second stream delta should append to same transcript item: cmd=%v view=%s", cmd, streaming.View())
	}

	streaming.width = 92
	streaming.height = 12
	if view := streaming.View(); !strings.Contains(view, "streamed reply") || !strings.Contains(view, "state running") {
		t.Fatalf("short streaming view should show latest agent token immediately:\n%s", view)
	}

	updated, cmd = streaming.Update(consoleStreamMsg{done: true})
	streaming = updated.(consoleTUIModel)
	if streaming.commandRunning || cmd == nil {
		t.Fatalf("stream completion should clear running state and refresh: model=%#v cmd=%v", streaming, cmd)
	}

	streaming.commandRunning = true
	updated, cmd = streaming.Update(consoleTickMsg(time.Now()))
	streaming = updated.(consoleTUIModel)
	if !streaming.commandRunning || streaming.loading || cmd == nil {
		t.Fatalf("background refresh should pause while chat is streaming: model=%#v cmd=%v", streaming, cmd)
	}

	small := newConsoleTUIModel(nil, tuiCommandOptions{serverURL: "http://agent", refreshEvery: time.Second})
	updated, cmd = small.Update(consoleViewMsg{view: ui.ConsoleView{Server: "http://agent", Ready: true, Watch: true, RefreshEvery: time.Second}})
	small = updated.(consoleTUIModel)
	updated, cmd = small.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	small = updated.(consoleTUIModel)
	shortView := small.View()
	if cmd != nil || !strings.Contains(shortView, "message or / command") || !strings.Contains(shortView, "...") {
		t.Fatalf("small terminal view should keep command panel visible and clip dashboard: cmd=%v\n%s", cmd, shortView)
	}
}

func TestSlashCommandMatchingFiltersAndHighlights(t *testing.T) {
	all := matchSlashCommands("/")
	if len(all) != len(slashCommands) {
		t.Fatalf("bare slash should show all commands: got %d want %d", len(all), len(slashCommands))
	}

	matches := matchSlashCommands("/h")
	var names []string
	for _, match := range matches {
		names = append(names, match.Command.Name)
		if match.Command.Name == "/help" && len(match.Indices) == 0 {
			t.Fatalf("/help should include highlighted match indices")
		}
	}
	for _, want := range []string{"/help", "/history"} {
		if !containsString(names, want) {
			t.Fatalf("/h matches missing %s: %#v", want, names)
		}
	}
	if containsString(names, "/status") {
		t.Fatalf("nonmatching /status should be hidden: %#v", names)
	}
}

func TestConsolePaletteRowsAnnotateRunRequiredCommands(t *testing.T) {
	model := newConsoleTUIModel(nil, tuiCommandOptions{serverURL: "http://agent", refreshEvery: time.Second})
	model.input = "/d"
	model.syncPaletteState()
	rows := model.consolePaletteRows()
	if len(rows) == 0 {
		t.Fatal("expected palette rows")
	}
	var draft ui.ConsolePaletteRow
	for _, row := range rows {
		if row.Label == "/draft" {
			draft = row
			break
		}
	}
	if draft.Label == "" || !draft.Disabled || draft.Annotation != "needs run" {
		t.Fatalf("run-required command should be disabled without active run: %#v", draft)
	}
}

func TestConsoleTUIModelSlashPaletteKeys(t *testing.T) {
	model := newConsoleTUIModel(nil, tuiCommandOptions{serverURL: "http://agent", refreshEvery: time.Second})
	model.view = ui.ConsoleView{Server: "http://agent", Ready: true, Watch: true, RefreshEvery: time.Second}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updated.(consoleTUIModel)
	if cmd != nil || !model.paletteOpen || len(model.consolePaletteRows()) != len(slashCommands) {
		t.Fatalf("slash should open palette: model=%#v cmd=%v", model, cmd)
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(consoleTUIModel)
	if cmd != nil || model.paletteSelected != 1 {
		t.Fatalf("down should move palette selection: model=%#v cmd=%v", model, cmd)
	}
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(consoleTUIModel)
	if cmd != nil || model.paletteSelected != 0 {
		t.Fatalf("up should move palette selection: model=%#v cmd=%v", model, cmd)
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(consoleTUIModel)
	if cmd != nil || model.paletteOpen || model.input != "/" {
		t.Fatalf("esc should close palette without exiting or clearing input: model=%#v cmd=%v", model, cmd)
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model = updated.(consoleTUIModel)
	if cmd != nil || !model.paletteOpen || model.input != "/h" {
		t.Fatalf("typing after slash should reopen filtered palette: model=%#v cmd=%v", model, cmd)
	}
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(consoleTUIModel)
	if !model.commandRunning || cmd == nil || len(model.transcript) == 0 || model.transcript[len(model.transcript)-1].Text != "/help" {
		t.Fatalf("enter should execute selected palette command: model=%#v cmd=%v", model, cmd)
	}
}

func TestConsoleTUIModelKJTypeWhenInputActive(t *testing.T) {
	model := newConsoleTUIModel(nil, tuiCommandOptions{serverURL: "http://agent", refreshEvery: time.Second})
	model.view = ui.ConsoleView{Server: "http://agent", Ready: true, Watch: true, RefreshEvery: time.Second}

	for _, r := range []rune("ask j") {
		updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated.(consoleTUIModel)
		if cmd != nil {
			t.Fatalf("typing %q should not schedule a command: cmd=%v", r, cmd)
		}
	}
	if model.input != "ask j" {
		t.Fatalf("k/j should type when input is active, got %q", model.input)
	}

	model.input = ""
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(consoleTUIModel)
	if cmd != nil || model.transcriptScroll != 1 || model.input != "" {
		t.Fatalf("empty-input k should scroll instead of typing: model=%#v cmd=%v", model, cmd)
	}
}

func TestNormalizeConsoleCommandOutputStripsPlainChatPrefix(t *testing.T) {
	if got := normalizeConsoleCommandOutput("agent: sessions 0"); got != "sessions 0" {
		t.Fatalf("unexpected normalized output %q", got)
	}
	if got := normalizeConsoleCommandOutput("sessions 0"); got != "sessions 0" {
		t.Fatalf("output without prefix should stay unchanged: %q", got)
	}
}

func TestConsoleChatResponderUsesRemoteOperatorChatWhenNoRunIsActive(t *testing.T) {
	responder, err := consoleChatResponder(tuiCommandOptions{serverURL: "http://agent"}, "")
	if err != nil {
		t.Fatal(err)
	}
	remote, ok := responder.(*remoteRunChatResponder)
	if !ok {
		t.Fatalf("expected remote responder for no-run chat, got %T", responder)
	}
	if remote.runID != "" {
		t.Fatalf("expected empty run id for operator chat, got %q", remote.runID)
	}
}

func TestChatSystemPromptDefinesOperationalContract(t *testing.T) {
	prompt := chatSystemPrompt(&config.Config{
		InstructionsPath:       "../../agent/instructions.md",
		HeadroomURL:            "http://headroom:8787",
		MemPalaceURL:           "http://mempalace:8788",
		GitLabURL:              "https://gitlab.example.com",
		GitHubAPIURL:           "https://api.github.com",
		Provider:               "ollama",
		ReviewModel:            "deepseek-coder-v2:16b",
		SmallModel:             "qwen2.5-coder-7b-16k:latest",
		EmbeddingModel:         "nomic-embed-text:latest",
		OrchestratorConfigPath: "./orchestrator.yaml",
	})
	for _, want := range []string{
		"7review Agent Instructions",
		"Your product identity is 7review.",
		"Do not claim to be Codex, Claude, OpenCode, OpenAI",
		"do not describe diff hunks as a context window",
		"use the retrieved memory block",
		"Always separate known state from assumptions.",
		"Never invent runtime state",
		"Prefer one clear next command",
		"Do not claim final approval",
		"Headroom and MemPalace as required dependencies",
		"REVIEW_API_TOKEN",
		"7review status --server <agent-url>",
		"7review chat --run <run-id> --server <agent-url>",
		"7review approve",
		"7review publish-final",
		"curl <agent-url>/ready",
		"get_run",
		"approve_run",
		"Provider: ollama",
		"Review model: deepseek-coder-v2:16b",
		"Small/formatter model: qwen2.5-coder-7b-16k:latest",
		"Embedding model: nomic-embed-text:latest",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestDeterministicOperatorAnswerHandlesIdentityAndModelQuestions(t *testing.T) {
	cfg := &config.Config{
		Provider:               "ollama",
		ReviewModel:            "deepseek-coder-v2:16b",
		SmallModel:             "qwen2.5-coder-7b-16k:latest",
		EmbeddingModel:         "nomic-embed-text:latest",
		OrchestratorConfigPath: "./orchestrator.yaml",
	}
	identity, ok := deterministicOperatorAnswer(cfg, "who created you?")
	if !ok || !strings.Contains(identity, "I am 7review") || strings.Contains(identity, "I am Codex") || strings.Contains(identity, "OpenAI-powered") {
		t.Fatalf("bad identity answer ok=%t:\n%s", ok, identity)
	}
	model, ok := deterministicOperatorAnswer(cfg, "what kind of model are you?")
	for _, want := range []string{"Provider: ollama", "Review model: deepseek-coder-v2:16b", "Formatter/chat model: qwen2.5-coder-7b-16k:latest", "Embedding model: nomic-embed-text:latest"} {
		if !ok || !strings.Contains(model, want) {
			t.Fatalf("model answer missing %q ok=%t:\n%s", want, ok, model)
		}
	}
	context, ok := deterministicOperatorAnswer(cfg, "what about your context window?")
	if !ok || !strings.Contains(context, "does not treat a diff hunk as the model context window") {
		t.Fatalf("bad context answer ok=%t:\n%s", ok, context)
	}
}

func TestModelChatResponderUserMessageIncludesRetrievedMemory(t *testing.T) {
	responder := &modelChatResponder{}
	block := renderOperatorMemoryRecall(review.MemoryRecall{
		Conventions: []string{"Use bounded webhook workers."},
		Decisions:   []string{"Headroom and MemPalace are required sidecars."},
		History:     []string{"Previous review preferred explicit HIL approval."},
	})
	message := responder.userMessage("what should I check?", block)
	for _, want := range []string{
		"Retrieved memory and embedding-backed context for the small model:",
		"retrieval: mempalace",
		"conventions:",
		"- Use bounded webhook workers.",
		"Structured task input:",
		"User message:",
		"what should I check?",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message missing %q:\n%s", want, message)
		}
	}
}

func TestExecuteConsoleCommandRunsSlashCommandThroughAgentTools(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/tools/execute" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		body, _ := io.ReadAll(req.Body)
		var call tools.ExecuteRequest
		if err := json.Unmarshal(body, &call); err != nil {
			t.Fatal(err)
		}
		if call.Name != "list_runs" {
			t.Fatalf("unexpected tool call %q", call.Name)
		}
		return jsonResponse(http.StatusOK, `{"name":"list_runs","result":[{"id":"owner/repo!7","provider":"github","project_id":"owner/repo","change_id":"7","title":"Fix validation","status":"drafted","event_count":2}]}`), nil
	})}

	out, err := executeConsoleCommand(client, tuiCommandOptions{serverURL: "http://agent"}, "", "/sessions")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"agent:", "sessions 1", "owner/repo!7", "Fix validation"} {
		if !strings.Contains(out, want) {
			t.Fatalf("command output missing %q:\n%s", want, out)
		}
	}
}

func TestExecuteConsoleCommandRunsContextCommandThroughAgentTools(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/tools/execute" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		body, _ := io.ReadAll(req.Body)
		var call tools.ExecuteRequest
		if err := json.Unmarshal(body, &call); err != nil {
			t.Fatal(err)
		}
		if call.Name != "get_selected_context" || call.Input["run"] != "owner/repo!7" {
			t.Fatalf("unexpected tool call %#v", call)
		}
		return jsonResponse(http.StatusOK, `{"name":"get_selected_context","result":{"run":"owner/repo!7","evidence_manifest":[{"source":"docs/CONTRACT.md","heading_or_key":"Session invariant","kind":"constraint","authority":"contract","selection_reason":"constraint_trace: INV-9 shared with docs/SRS.md#FR-MSG-52","score":910}]}}`), nil
	})}

	out, err := executeConsoleCommand(client, tuiCommandOptions{serverURL: "http://agent"}, "owner/repo!7", "/context")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"agent:", "context owner/repo!7", "constraint_trace: INV-9 shared with docs/SRS.md#FR-MSG-52"} {
		if !strings.Contains(out, want) {
			t.Fatalf("context command output missing %q:\n%s", want, out)
		}
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
	for _, want := range []string{"7review", "owner/repo!7", "Fix validation", "history    3 events", "latest     status_changed drafted", "openrouter", "traceability-review", "tools     2", "refresh 5s", "live refresh 5s"} {
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
