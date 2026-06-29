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

func TestHandleToolExecuteSelectedContextIncludesEvidenceManifest(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	reqRun := review.Request{
		Provider:     "github",
		ProjectID:    "owner/repo",
		ChangeID:     "11",
		Title:        "Update session API",
		Description:  "Touch REQ-12 and POST /sessions.",
		ChangedPaths: []string{"api/sessions.go"},
	}
	run, err := store.Start(context.Background(), reqRun)
	if err != nil {
		t.Fatal(err)
	}
	rc := review.NewContext(reqRun)
	rc.Source.CorpusSections = []review.Section{
		{Path: "docs/openapi.yaml", Title: "paths./sessions", Kind: review.KindAPI, Content: "post: {}"},
		{Path: "docs/openapi.yaml", Title: "schemas.Session", Kind: review.KindAPI, Content: "type: object"},
	}
	rc.Source.Evidence = []review.EvidenceItem{
		{
			Source:          "docs/openapi.yaml",
			HeadingOrKey:    "paths./sessions",
			Kind:            review.KindAPI,
			Authority:       "api_contract",
			MatchedSignals:  []string{"/sessions"},
			SelectionReason: "seed: API route /sessions",
			Score:           198,
			ContentBytes:    len("post: {}"),
		},
		{
			Source:          "docs/openapi.yaml",
			HeadingOrKey:    "schemas.Session",
			Kind:            review.KindAPI,
			Authority:       "api_contract",
			MatchedSignals:  []string{"/sessions"},
			SelectionReason: "interface_trace: session -> docs/openapi.yaml#schemas.Session",
			Score:           115,
			ContentBytes:    len("type: object"),
		},
	}
	rc.Source.SkillSections = []review.Section{{Path: "agent/skills/api-contract-review/SKILL.md", Title: "api-contract-review", Kind: review.KindRules, Content: "contract rules"}}
	rc.Source.Findings = []review.Finding{{ID: "F1", Severity: review.SeverityHigh, Title: "confirmed", Strength: "confirmed"}}
	rc.Source.HumanCheck = []review.Finding{{ID: "H1", Severity: review.SeverityMedium, Title: "likely", Strength: "likely", ValidationStatus: "needs_human_check"}}
	rc.Source.Notes = []review.Finding{{ID: "N1", Severity: review.SeverityInfo, Title: "note", FindingType: "note"}}
	rc.Source.Questions = []review.Finding{{ID: "Q1", Severity: review.SeverityInfo, Title: "question", FindingType: "question"}}
	rc.Source.Model = review.ModelReview{
		ParseStatus:        "empty_findings",
		ParseWarning:       "model returned an explicit empty findings list",
		AcceptedFindings:   1,
		HumanCheckFindings: 1,
		NoteFindings:       1,
		QuestionFindings:   1,
		RawResponseExcerpt: "[]",
		ProviderTrace:      "step5=openrouter/openrouter/free",
	}
	if err := store.SaveContext(context.Background(), run.ID, rc); err != nil {
		t.Fatal(err)
	}
	s := &Server{pipeline: &pipeline.Pipeline{Jobs: store}}
	req := httptest.NewRequest(http.MethodPost, "/tools/execute", strings.NewReader(`{"name":"get_selected_context","input":{"run":"owner/repo!11"}}`))
	rec := httptest.NewRecorder()

	s.handleToolExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Name   string                   `json:"name"`
		Result selectedContextStatusDTO `json:"result"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Name != "get_selected_context" || resp.Result.Run != "owner/repo!11" {
		t.Fatalf("unexpected selected context response: %#v", resp)
	}
	if len(resp.Result.CorpusSections) != 2 || len(resp.Result.Evidence) != 2 {
		t.Fatalf("context/evidence length mismatch: %#v", resp.Result)
	}
	assertEvidenceDTO(t, resp.Result.Evidence[0], "docs/openapi.yaml", "paths./sessions", "seed: API route /sessions", "/sessions", 198)
	assertEvidenceDTO(t, resp.Result.Evidence[1], "docs/openapi.yaml", "schemas.Session", "interface_trace: session -> docs/openapi.yaml#schemas.Session", "/sessions", 115)
	if len(resp.Result.SkillSections) != 1 || !strings.Contains(resp.Result.SkillSections[0].SelectionReason, "matched title + description + changed paths") {
		t.Fatalf("skill selection reason missing request signal explanation: %#v", resp.Result.SkillSections)
	}
	if resp.Result.Model.ParseStatus != "empty_findings" || resp.Result.Model.RawResponseExcerpt != "[]" {
		t.Fatalf("model audit missing from selected context: %#v", resp.Result.Model)
	}
	if resp.Result.Model.AcceptedFindings != 1 || resp.Result.Model.HumanCheckFindings != 1 || resp.Result.Model.NoteFindings != 1 || resp.Result.Model.QuestionFindings != 1 {
		t.Fatalf("model quality counters missing from selected context: %#v", resp.Result.Model)
	}
	if len(resp.Result.Findings) != 1 || len(resp.Result.HumanCheck) != 1 || len(resp.Result.Notes) != 1 || len(resp.Result.Questions) != 1 {
		t.Fatalf("selected context missing review quality categories: %#v", resp.Result)
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

func assertEvidenceDTO(t *testing.T, item evidenceStatusDTO, source, heading, reason, signal string, score int) {
	t.Helper()
	if item.Source != source || item.HeadingOrKey != heading || item.SelectionReason != reason || item.Score != score {
		t.Fatalf("unexpected evidence item: %#v", item)
	}
	if item.Authority == "" || item.ContentBytes == 0 {
		t.Fatalf("evidence item missing authority/content bytes: %#v", item)
	}
	for _, got := range item.MatchedSignals {
		if got == signal {
			return
		}
	}
	t.Fatalf("evidence item missing matched signal %q: %#v", signal, item)
}
