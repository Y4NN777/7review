package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/pipeline"
	"github.com/Y4NN777/7review/agent/review"
	"github.com/Y4NN777/7review/agent/skills"
)

func TestHandleToolsReturnsToolCatalog(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/tools", nil)
	rec := httptest.NewRecorder()

	s.handleTools(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "get_run") || !strings.Contains(rec.Body.String(), "approve_run") {
		t.Fatalf("unexpected tools response: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "lifecycle_stage") || !strings.Contains(rec.Body.String(), "requires_approval") {
		t.Fatalf("tools response missing lifecycle metadata: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "implemented") || !strings.Contains(rec.Body.String(), "/tools/execute") {
		t.Fatalf("tools response missing implementation metadata: %s", rec.Body.String())
	}
}

func TestHandleToolExecuteListRuns(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	reqRun := review.Request{Provider: "github", ProjectID: "owner/repo", ChangeID: "7", MRIID: 7, Title: "Fix checkout"}
	if _, err := store.Start(context.Background(), reqRun); err != nil {
		t.Fatal(err)
	}
	s := &Server{pipeline: &pipeline.Pipeline{Jobs: store}}
	req := httptest.NewRequest(http.MethodPost, "/tools/execute", strings.NewReader(`{"name":"list_runs"}`))
	rec := httptest.NewRecorder()

	s.handleToolExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Name   string   `json:"name"`
		Result []runDTO `json:"result"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Name != "list_runs" || len(resp.Result) != 1 || resp.Result[0].ID != "owner/repo!7" || resp.Result[0].EventCount == 0 {
		t.Fatalf("unexpected tool response: %#v", resp)
	}
}

func TestHandleToolExecuteRunTimeline(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	reqRun := review.Request{Provider: "github", ProjectID: "owner/repo", ChangeID: "7", MRIID: 7, Title: "Fix checkout"}
	run, err := store.Start(context.Background(), reqRun)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(context.Background(), run.ID, pipeline.RunEvent{Type: "chat_message", Status: pipeline.StatusDrafted, Message: "explain F1"}); err != nil {
		t.Fatal(err)
	}
	s := &Server{pipeline: &pipeline.Pipeline{Jobs: store}}
	req := httptest.NewRequest(http.MethodPost, "/tools/execute", strings.NewReader(`{"name":"get_run_timeline","input":{"run":"owner/repo!7"}}`))
	rec := httptest.NewRecorder()

	s.handleToolExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Name   string         `json:"name"`
		Result runTimelineDTO `json:"result"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Name != "get_run_timeline" || resp.Result.Run != "owner/repo!7" || resp.Result.EventCount != 2 || resp.Result.Events[1].Type != "chat_message" {
		t.Fatalf("unexpected timeline response: %#v", resp)
	}
}

func TestHandleToolExecuteGetConfigStatusRedactsSecrets(t *testing.T) {
	s := &Server{
		cfg: &config.Config{
			ListenAddr:             ":8080",
			GitHubAPIURL:           "https://api.github.com",
			GitHubToken:            "github-token",
			GitHubWebhookSecret:    "github-secret",
			OpenRouterAPIKey:       "openrouter-token",
			DeepSeekAPIKey:         "deepseek-token",
			HeadroomURL:            "http://headroom:8787",
			MemPalaceURL:           "http://mempalace:8788",
			WebhookWorkers:         2,
			WebhookQueueSize:       10,
			OrchestratorConfigPath: "orchestrator.yaml",
		},
		pipeline: &pipeline.Pipeline{Jobs: pipeline.NewMemoryRunStore()},
		work:     make(chan workItem, 10),
	}
	req := httptest.NewRequest(http.MethodPost, "/tools/execute", strings.NewReader(`{"name":"get_config_status"}`))
	rec := httptest.NewRecorder()

	s.handleToolExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, secret := range []string{"github-token", "github-secret", "openrouter-token", "deepseek-token"} {
		if strings.Contains(body, secret) {
			t.Fatalf("config status leaked secret %q in %s", secret, body)
		}
	}
	if !strings.Contains(body, `"has_github":true`) || !strings.Contains(body, `"has_openrouter":true`) || !strings.Contains(body, `"has_deepseek":true`) {
		t.Fatalf("config status missing provider booleans: %s", body)
	}
}

func TestHandleToolExecuteListSkills(t *testing.T) {
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Jobs: pipeline.NewMemoryRunStore(),
			SkillLoader: &skills.Loader{Skills: []skills.Skill{{
				Name:          "methodology-review",
				Description:   "Use for review methodology.",
				License:       "Apache-2.0",
				Compatibility: "go",
				AllowedTools:  "bash",
				Metadata:      map[string]string{"version": "1.0.0"},
				Path:          "agent/skills/methodology-review/SKILL.md",
			}}},
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/tools/execute", strings.NewReader(`{"name":"list_skills"}`))
	rec := httptest.NewRecorder()

	s.handleToolExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Name   string           `json:"name"`
		Result []skillStatusDTO `json:"result"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Name != "list_skills" || len(resp.Result) != 1 {
		t.Fatalf("unexpected skill response: %#v", resp)
	}
	got := resp.Result[0]
	if got.Name != "methodology-review" || !got.Loaded || got.Metadata["version"] != "1.0.0" {
		t.Fatalf("unexpected skill metadata: %#v", got)
	}
}

func TestHandleToolExecuteRejectsUnimplementedTool(t *testing.T) {
	s := &Server{pipeline: &pipeline.Pipeline{Jobs: pipeline.NewMemoryRunStore()}}
	req := httptest.NewRequest(http.MethodPost, "/tools/execute", strings.NewReader(`{"name":"not_a_tool","input":{"run":"p!7"}}`))
	rec := httptest.NewRecorder()

	s.handleToolExecute(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unknown tool") {
		t.Fatalf("unexpected error: %s", rec.Body.String())
	}
}

func TestRoutesExposeToolExecute(t *testing.T) {
	s := &Server{
		cfg:      &config.Config{},
		pipeline: &pipeline.Pipeline{Jobs: pipeline.NewMemoryRunStore()},
		mux:      http.NewServeMux(),
		work:     make(chan workItem, 1),
		seen:     make(map[string]time.Time),
	}
	s.routes()
	req := httptest.NewRequest(http.MethodPost, "/tools/execute", strings.NewReader(`{"name":"list_runs"}`))
	rec := httptest.NewRecorder()

	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected routed tool execution, got %d: %s", rec.Code, rec.Body.String())
	}
}
