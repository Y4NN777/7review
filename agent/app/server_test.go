package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/orchestrator"
	"github.com/Y4NN777/7review/agent/pipeline"
	"github.com/Y4NN777/7review/agent/review"
	"github.com/Y4NN777/7review/agent/skills"
)

func TestHandleReadyReportsRequiredDependencies(t *testing.T) {
	s := &Server{
		pipeline: &pipeline.Pipeline{
			ContextReducer: fakeReducer{err: errors.New("headroom down")},
			Memory:         fakeMemory{},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	s.handleReady(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	var status readinessStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.Ready || status.Dependencies["headroom"] == "ok" || status.Dependencies["mempalace"] != "ok" {
		t.Fatalf("unexpected readiness: %#v", status)
	}
}

func TestHandleReadyReportsHealthyRuntime(t *testing.T) {
	orch := orchestrator.NewOrchestrator(orchestrator.DefaultOrchestratorConfig("large", "small", "fake"), map[string]orchestrator.LLMProvider{
		"fake": streamingProvider{},
	})
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Orchestrator:     orch,
			Jobs:             pipeline.NewMemoryRunStore(),
			ContextReducer:   fakeReducer{},
			Memory:           fakeMemory{},
			SCM:              fakeAppSCM{},
			FindingValidator: pipeline.DefaultFindingValidator{},
		},
		work: make(chan workItem, 3),
	}
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	s.handleReady(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var status readinessStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	for _, dep := range []string{"pipeline", "orchestrator", "queue", "run_store", "headroom", "mempalace"} {
		if status.Dependencies[dep] == "" {
			t.Fatalf("missing dependency %q in %#v", dep, status)
		}
	}
	if status.Queue.Depth != 0 || status.Queue.Capacity != 3 || status.Queue.Available != 3 {
		t.Fatalf("unexpected queue status: %#v", status.Queue)
	}
	if !status.Ready {
		t.Fatalf("expected ready status, got %#v", status)
	}
}

func TestHandleReadyReportsMissingCoreDependencies(t *testing.T) {
	s := &Server{pipeline: &pipeline.Pipeline{}}
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	s.handleReady(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
	var status readinessStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	for _, dep := range []string{"orchestrator", "queue", "run_store", "headroom", "mempalace"} {
		if status.Dependencies[dep] == "" || status.Dependencies[dep] == "ok" {
			t.Fatalf("expected dependency %q down in %#v", dep, status)
		}
	}
}

func TestHandleReadyMethod(t *testing.T) {
	s := &Server{pipeline: &pipeline.Pipeline{}}
	req := httptest.NewRequest(http.MethodPost, "/ready", nil)
	rec := httptest.NewRecorder()

	s.handleReady(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHTTPServerUsesConfiguredTimeouts(t *testing.T) {
	s := &Server{
		cfg: &config.Config{
			ListenAddr:        ":9090",
			ReadHeaderTimeout: 7000,
			ReadTimeout:       31000,
			WriteTimeout:      121000,
			IdleTimeout:       122000,
		},
		mux: http.NewServeMux(),
	}

	server := s.httpServer()

	if server.Addr != ":9090" || server.Handler != s.mux {
		t.Fatalf("unexpected server wiring: %#v", server)
	}
	if server.ReadHeaderTimeout != 7*time.Second ||
		server.ReadTimeout != 31*time.Second ||
		server.WriteTimeout != 121*time.Second ||
		server.IdleTimeout != 122*time.Second {
		t.Fatalf("unexpected timeouts: header=%s read=%s write=%s idle=%s", server.ReadHeaderTimeout, server.ReadTimeout, server.WriteTimeout, server.IdleTimeout)
	}
}

func TestRunWorkItemCancelsAtConfiguredTimeout(t *testing.T) {
	s := &Server{cfg: &config.Config{WebhookJobTimeout: 1}}

	err := s.runWorkItem(1, workItem{
		name: "timeout",
		run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestRunWorkItemConvertsPanicToError(t *testing.T) {
	s := &Server{cfg: &config.Config{WebhookJobTimeout: 1000}}

	err := s.runWorkItem(1, workItem{
		name: "panic-job",
		run: func(ctx context.Context) error {
			panic("broken adapter")
		},
	})

	if err == nil || !strings.Contains(err.Error(), "panic processing panic-job") {
		t.Fatalf("expected panic error, got %v", err)
	}
}

func TestQueueStatusTracksWorkerOutcomes(t *testing.T) {
	s := &Server{
		cfg:  &config.Config{WebhookJobTimeout: 1000},
		work: make(chan workItem, 2),
	}
	if err := s.enqueue(workItem{name: "ok", run: func(context.Context) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	if got := s.queueStatus(); got.Depth != 1 || got.Capacity != 2 || got.Available != 1 || got.Enqueued != 1 {
		t.Fatalf("unexpected queued status: %#v", got)
	}
	item := <-s.work
	if err := s.runWorkItem(1, item); err != nil {
		t.Fatal(err)
	}
	s.stats.completed.Add(1)
	if err := s.runWorkItem(1, workItem{name: "bad", run: func(context.Context) error { return errors.New("boom") }}); err == nil {
		t.Fatal("expected worker error")
	}
	s.stats.failed.Add(1)

	got := s.queueStatus()
	if got.Depth != 0 || got.Completed != 1 || got.Failed != 1 || got.Enqueued != 1 {
		t.Fatalf("unexpected final queue status: %#v", got)
	}
}

func TestRequireAuthRejectsMissingOperatorToken(t *testing.T) {
	s := &Server{cfg: &config.Config{APIToken: "secret"}}
	req := httptest.NewRequest(http.MethodGet, "/runs", nil)
	rec := httptest.NewRecorder()

	s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthAcceptsBearerToken(t *testing.T) {
	s := &Server{cfg: &config.Config{APIToken: "secret"}}
	req := httptest.NewRequest(http.MethodGet, "/runs", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestClaimDeliveryDeduplicatesAndCanRelease(t *testing.T) {
	s := &Server{}
	if !s.claimDelivery("github:delivery-1") {
		t.Fatal("first delivery should be accepted")
	}
	if s.claimDelivery("github:delivery-1") {
		t.Fatal("duplicate delivery should be rejected")
	}
	s.releaseDelivery("github:delivery-1")
	if !s.claimDelivery("github:delivery-1") {
		t.Fatal("released delivery should be accepted again")
	}
}

func TestClaimDeliveryPurgesExpiredEntries(t *testing.T) {
	s := &Server{
		seen: map[string]time.Time{
			"github:old": time.Now().UTC().Add(-deliveryRetention - time.Minute),
		},
	}

	if !s.claimDelivery("github:old") {
		t.Fatal("expired delivery should be accepted again")
	}
	if len(s.seen) != 1 {
		t.Fatalf("expired delivery map was not compacted: %#v", s.seen)
	}
}

func TestDeliveryKeyReleasedWhenQueuedWebhookRunFails(t *testing.T) {
	s := &Server{
		cfg:      &config.Config{GitLabURL: "https://gitlab.example.com", GitLabToken: "token", WebhookSecret: "secret"},
		pipeline: &pipeline.Pipeline{},
		mux:      http.NewServeMux(),
		work:     make(chan workItem, 1),
		seen:     make(map[string]time.Time),
	}
	s.routes()
	body := `{
		"object_kind":"merge_request",
		"event_type":"merge_request",
		"project":{"id":42},
		"object_attributes":{"iid":7,"action":"update","title":"Fix auth"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(body))
	req.Header.Set("X-Gitlab-Token", "secret")
	req.Header.Set("X-Gitlab-Event-UUID", "delivery-fail")
	rec := httptest.NewRecorder()

	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	item := <-s.work
	if err := item.run(context.Background()); err == nil {
		t.Fatal("expected pipeline failure")
	}
	if !s.claimDelivery("gitlab:delivery-fail") {
		t.Fatal("failed delivery should be retryable")
	}
}

func TestRoutesDisableUnconfiguredProviderWebhooks(t *testing.T) {
	s := &Server{
		cfg:  &config.Config{GitHubAPIURL: "https://api.github.com", GitHubToken: "token", GitHubWebhookSecret: "github-secret"},
		mux:  http.NewServeMux(),
		work: make(chan workItem, 1),
		seen: make(map[string]time.Time),
	}
	s.routes()
	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected inactive GitLab route to return 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(s.work) != 0 {
		t.Fatal("inactive webhook should not enqueue work")
	}
}

func TestRoutesEnableOnlyConfiguredGitHubWebhook(t *testing.T) {
	body := `{
		"action":"opened",
		"number":7,
		"repository":{"full_name":"o/r"},
		"pull_request":{"title":"Fix","head":{"sha":"abc"},"base":{"sha":"def"}}
	}`
	s := &Server{
		cfg:      &config.Config{GitHubAPIURL: "https://api.github.com", GitHubToken: "token", GitHubWebhookSecret: "github-secret"},
		pipeline: &pipeline.Pipeline{},
		mux:      http.NewServeMux(),
		work:     make(chan workItem, 1),
		seen:     make(map[string]time.Time),
	}
	s.routes()
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", githubSignature("github-secret", body))
	rec := httptest.NewRecorder()

	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected configured GitHub route to accept, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(s.work) != 1 {
		t.Fatal("configured GitHub route should enqueue work")
	}
}

func TestHandleRunEndpointsExposeStoredReviewContext(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	req := review.Request{Provider: "gitlab", ProjectID: "p", MRIID: 7, ChangeID: "7", Title: "Fix checkout"}
	run, err := store.Start(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	rc := review.NewContext(req)
	rc.DraftReport = "draft body"
	rc.Findings = []review.Finding{{ID: "F1", Severity: review.SeverityHigh, Title: "bug"}}
	if err := store.SaveContext(context.Background(), run.ID, rc); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), run.ID, pipeline.StatusDrafted, nil); err != nil {
		t.Fatal(err)
	}
	s := &Server{pipeline: &pipeline.Pipeline{Jobs: store}}

	listReq := httptest.NewRequest(http.MethodGet, "/runs", nil)
	listRec := httptest.NewRecorder()
	s.handleRuns(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var listed []runDTO
	if err := json.NewDecoder(listRec.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != "p!7" || len(listed[0].Findings) != 0 {
		t.Fatalf("unexpected list response: %#v", listed)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/run?id=p!7", nil)
	getRec := httptest.NewRecorder()
	s.handleRun(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected get 200, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var detail runDTO
	if err := json.NewDecoder(getRec.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail.DraftReport != "draft body" || len(detail.Findings) != 1 {
		t.Fatalf("unexpected detail response: %#v", detail)
	}
}

func TestHandleChatStreamStreamsAgainstStoredRun(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	req := review.Request{Provider: "gitlab", ProjectID: "p", MRIID: 7, ChangeID: "7", Title: "Fix checkout"}
	run, err := store.Start(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	rc := review.NewContext(req)
	rc.DraftReport = "draft body"
	rc.Findings = []review.Finding{{ID: "F1", Severity: review.SeverityHigh, Title: "bug"}}
	if err := store.SaveContext(context.Background(), run.ID, rc); err != nil {
		t.Fatal(err)
	}
	orch := orchestrator.NewOrchestrator(orchestrator.DefaultOrchestratorConfig("large", "small", "fake"), map[string]orchestrator.LLMProvider{
		"fake": streamingProvider{},
	})
	s := &Server{pipeline: &pipeline.Pipeline{Jobs: store, Orchestrator: orch}}

	reqHTTP := httptest.NewRequest(http.MethodPost, "/chat/stream?run=p!7", strings.NewReader(`{"message":"explain F1"}`))
	rec := httptest.NewRecorder()
	s.handleChatStream(rec, reqHTTP)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	if !strings.Contains(out, `event: done`) || !strings.Contains(out, `"delta":"stream "`) || !strings.Contains(out, `"delta":"reply"`) {
		t.Fatalf("unexpected stream response:\n%s", out)
	}
}

func TestHandleChatStreamRejectsOversizedMessage(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	reqRun := review.Request{Provider: "gitlab", ProjectID: "p", MRIID: 7, ChangeID: "7", Title: "Fix checkout"}
	if _, err := store.Start(context.Background(), reqRun); err != nil {
		t.Fatal(err)
	}
	s := &Server{pipeline: &pipeline.Pipeline{Jobs: store}}
	reqHTTP := httptest.NewRequest(http.MethodPost, "/chat/stream?run=p!7", strings.NewReader(strings.Repeat("x", int(chatMaxBodyBytes)+1)))
	rec := httptest.NewRecorder()

	s.handleChatStream(rec, reqHTTP)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
	}
}

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
	if resp.Name != "list_runs" || len(resp.Result) != 1 || resp.Result[0].ID != "owner/repo!7" {
		t.Fatalf("unexpected tool response: %#v", resp)
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

func TestHandleToolExecuteObservabilityTools(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	reqRun := review.Request{Provider: "github", ProjectID: "owner/repo", ChangeID: "9", Title: "Update API"}
	run, err := store.Start(context.Background(), reqRun)
	if err != nil {
		t.Fatal(err)
	}
	rc := review.NewContext(reqRun)
	rc.Source.SCM = &review.SCMContext{
		Provider:    "github",
		ProjectID:   "owner/repo",
		ChangeID:    "9",
		WebURL:      "https://github.example.com/owner/repo/pull/9",
		Discussions: []review.Discussion{{ID: "d1"}},
		Checks:      []review.CheckRun{{Name: "ci", Status: "completed"}},
		Approvals:   []review.Approval{{Reviewer: "lead", State: "approved"}},
	}
	rc.Source.ChangedFiles = []review.ChangedFile{{
		NewPath:   "api/users.go",
		Patch:     "@@ -1 +1\n+change",
		Status:    "modified",
		Additions: 3,
		Deletions: 1,
	}}
	rc.Diff = &review.StructuredDiff{Files: []review.FileDiff{{
		Path:       "api/users.go",
		Patch:      "@@ -1 +1\n+change",
		TokenCount: 42,
	}}}
	rc.Source.Diff = rc.Diff
	rc.CorpusSections = []review.Section{{Path: "PRD.md", Title: "PRD", Kind: review.KindPlanning, Content: "feature\nrule"}}
	rc.Source.CorpusSections = rc.CorpusSections
	rc.SkillSections = []review.Section{{Path: "agent/skills/api-contract-review/SKILL.md", Title: "api-contract-review", Kind: review.KindRules, Content: "skill body"}}
	rc.Source.SkillSections = rc.SkillSections
	rc.Source.Memory = review.MemoryRecall{Conventions: []string{"return typed errors"}}
	rc.DraftReport = "draft report"
	rc.FinalReport = "final report"
	rc.HILApproved = true
	rc.WebURL = rc.Source.SCM.WebURL
	if err := store.SaveContext(context.Background(), run.ID, rc); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), run.ID, pipeline.StatusFinalized, nil); err != nil {
		t.Fatal(err)
	}
	s := &Server{
		cfg: &config.Config{
			Provider:          "openrouter",
			ReviewModel:       "openai/gpt-4o",
			SmallModel:        "openai/gpt-4o-mini",
			OpenRouterAPIKey:  "secret",
			OpenRouterBaseURL: "https://openrouter.ai/api",
			DeepSeekBaseURL:   "https://api.deepseek.com",
		},
		pipeline: &pipeline.Pipeline{Jobs: store, Memory: proposalMemory{}},
	}

	cases := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "get_selected_context",
			body: `{"name":"get_selected_context","input":{"run":"owner/repo!9"}}`,
			want: []string{`"corpus_sections"`, `"PRD.md"`, `"skill_sections"`, `"return typed errors"`},
		},
		{
			name: "get_diff_summary",
			body: `{"name":"get_diff_summary","input":{"run":"owner/repo!9"}}`,
			want: []string{`"total_tokens":42`, `"additions":3`, `"deletions":1`, `"api/users.go"`},
		},
		{
			name: "get_publish_status",
			body: `{"name":"get_publish_status","input":{"run":"owner/repo!9"}}`,
			want: []string{`"status":"finalized"`, `"hil_approved":true`, `"has_final_report":true`, `"scm_discussions":1`},
		},
		{
			name: "list_provider_status",
			body: `{"name":"list_provider_status"}`,
			want: []string{`"active_provider":"openrouter"`, `"name":"openrouter"`, `"configured":true`, `"reasoner"`},
		},
		{
			name: "preview_memory_proposal",
			body: `{"name":"preview_memory_proposal","input":{"run":"owner/repo!9"}}`,
			want: []string{`"approved":true`, `"Conventions":["final report"]`, `"final_bytes":12`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/tools/execute", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			s.handleToolExecute(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			for _, want := range tc.want {
				if !strings.Contains(rec.Body.String(), want) {
					t.Fatalf("response missing %q:\n%s", want, rec.Body.String())
				}
			}
		})
	}
}

func TestHandleToolExecuteProviderStatusUsesLoadedOrchestrator(t *testing.T) {
	orch := orchestrator.NewOrchestrator(&orchestrator.OrchestratorConfig{
		Roles: map[orchestrator.ModelRole]orchestrator.RoleConfig{
			orchestrator.RoleReasoner: {
				Primary:   orchestrator.ModelSpec{Model: "claude-sonnet", Provider: "anthropic"},
				Fallbacks: []orchestrator.ModelSpec{{Model: "qwen2.5-coder:32b", Provider: "ollama"}},
				MaxTokens: 4096,
				Parallel:  true,
			},
		},
	}, map[string]orchestrator.LLMProvider{"ollama": staticResponseProvider{response: "ok"}})
	s := &Server{
		cfg: &config.Config{
			Provider:               "anthropic",
			OrchestratorConfigPath: "/app/orchestrator.yaml",
			OllamaBaseURL:          "http://ollama:11434",
		},
		pipeline: &pipeline.Pipeline{Jobs: pipeline.NewMemoryRunStore(), Orchestrator: orch},
	}
	req := httptest.NewRequest(http.MethodPost, "/tools/execute", strings.NewReader(`{"name":"list_provider_status"}`))
	rec := httptest.NewRecorder()

	s.handleToolExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"mode":"orchestrator"`, `"active_provider":""`, `"primary":"claude-sonnet@anthropic"`, `"fallbacks":["qwen2.5-coder:32b@ollama"]`, `"name":"ollama"`, `"configured":true`} {
		if !strings.Contains(body, want) {
			t.Fatalf("provider status missing %q:\n%s", want, body)
		}
	}
}

func TestHandleToolExecuteSuppressFinding(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	reqRun := review.Request{Provider: "github", ProjectID: "owner/repo", ChangeID: "7"}
	run, err := store.Start(context.Background(), reqRun)
	if err != nil {
		t.Fatal(err)
	}
	rc := review.NewContext(reqRun)
	rc.Findings = []review.Finding{
		{ID: "F1", Severity: review.SeverityHigh, Title: "Keep", Confidence: 0.9},
		{ID: "F2", Severity: review.SeverityLow, Title: "Suppress", Confidence: 0.8},
	}
	rc.Source.Findings = rc.Findings
	rc.DraftReport = "draft with Suppress"
	rc.Source.Report.Draft = rc.DraftReport
	if err := store.SaveContext(context.Background(), run.ID, rc); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), run.ID, pipeline.StatusDrafted, nil); err != nil {
		t.Fatal(err)
	}
	s := &Server{pipeline: &pipeline.Pipeline{Jobs: store}}
	body := `{"name":"suppress_finding","input":{"run":"owner/repo!7","finding_id":"F2","reason":"false positive"}}`
	req := httptest.NewRequest(http.MethodPost, "/tools/execute", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleToolExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	updated, err := store.Get(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Findings) != 1 || updated.Findings[0].ID != "F1" {
		t.Fatalf("finding was not suppressed: %#v", updated.Findings)
	}
	if updated.Context == nil || len(updated.Context.HILRejectedIDs) != 1 || updated.Context.HILRejectedIDs[0] != "F2" {
		t.Fatalf("suppression was not persisted: %#v", updated.Context)
	}
	if !strings.Contains(rec.Body.String(), `"accepted":true`) || !strings.Contains(rec.Body.String(), `"finding_id":"F2"`) {
		t.Fatalf("unexpected suppress response: %s", rec.Body.String())
	}
}

func TestHandleToolExecuteReviseDraft(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	reqRun := review.Request{Provider: "github", ProjectID: "owner/repo", ChangeID: "7"}
	run, err := store.Start(context.Background(), reqRun)
	if err != nil {
		t.Fatal(err)
	}
	rc := review.NewContext(reqRun)
	rc.DraftReport = "old draft"
	rc.Source.Report.Draft = rc.DraftReport
	if err := store.SaveContext(context.Background(), run.ID, rc); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), run.ID, pipeline.StatusDrafted, nil); err != nil {
		t.Fatal(err)
	}
	orch := orchestrator.NewOrchestrator(orchestrator.DefaultOrchestratorConfig("large", "small", "fake"), map[string]orchestrator.LLMProvider{
		"fake": staticResponseProvider{response: "new draft"},
	})
	s := &Server{pipeline: &pipeline.Pipeline{Jobs: store, Orchestrator: orch}}
	body := `{"name":"revise_draft","input":{"run":"owner/repo!7","request":"tighten wording"}}`
	req := httptest.NewRequest(http.MethodPost, "/tools/execute", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleToolExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	updated, err := store.Get(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DraftReport != "new draft" || updated.Context.Source.Report.Draft != "new draft" {
		t.Fatalf("draft was not revised: %#v", updated)
	}
}

func TestHandleToolExecuteRerunReview(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	reqRun := review.Request{Provider: "github", ProjectID: "owner/repo", ChangeID: "7", Title: "Fix retry"}
	run, err := store.Start(context.Background(), reqRun)
	if err != nil {
		t.Fatal(err)
	}
	rc := review.NewContext(reqRun)
	rc.DraftReport = "old draft"
	if err := store.SaveContext(context.Background(), run.ID, rc); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), run.ID, pipeline.StatusFailed, errors.New("old failure")); err != nil {
		t.Fatal(err)
	}
	orch := orchestrator.NewOrchestrator(orchestrator.DefaultOrchestratorConfig("large", "small", "fake"), map[string]orchestrator.LLMProvider{
		"fake": staticResponseProvider{response: `[]`},
	})
	s := &Server{pipeline: &pipeline.Pipeline{
		Config:           &config.Config{MaxDiffTokens: 6000, CorpusRoot: t.TempDir()},
		Jobs:             store,
		Orchestrator:     orch,
		SCM:              fakeAppSCM{},
		SCMPublisher:     &fakeAppPublisher{},
		Memory:           fakeMemory{},
		ContextReducer:   fakeReducer{},
		Policy:           pipeline.DefaultPolicyFilter{},
		FindingValidator: pipeline.DefaultFindingValidator{},
	}}
	body := `{"name":"rerun_review","input":{"run":"owner/repo!7","reason":"new commits"}}`
	req := httptest.NewRequest(http.MethodPost, "/tools/execute", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.handleToolExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	updated, err := store.Get(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != pipeline.StatusDrafted || updated.Error != "" || !strings.Contains(updated.DraftReport, "No validated findings") {
		t.Fatalf("run was not rerun: %#v", updated)
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

func TestHandlePublishFinalEnqueuesRunPublish(t *testing.T) {
	called := make(chan struct{}, 1)
	store := pipeline.NewMemoryRunStore()
	reqRun := review.Request{Provider: "gitlab", ProjectID: "p", MRIID: 7, ChangeID: "7"}
	run, err := store.Start(context.Background(), reqRun)
	if err != nil {
		t.Fatal(err)
	}
	rc := review.NewContext(reqRun)
	rc.HILApproved = true
	rc.FinalReport = "old final"
	rc.Source.SCM = &review.SCMContext{Provider: "gitlab", ProjectID: "p", MRIID: 7, ChangeID: "7"}
	if err := store.SaveContext(context.Background(), run.ID, rc); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), run.ID, pipeline.StatusDrafted, nil); err != nil {
		t.Fatal(err)
	}
	publisher := &fakeAppPublisher{}
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Jobs:         store,
			SCMPublisher: publisher,
			Memory:       fakeMemory{},
		},
		work: make(chan workItem, 1),
	}
	s.pipeline.SCM = fakeAppSCM{}
	req := httptest.NewRequest(http.MethodPost, "/publish/final?run=p!7", strings.NewReader("final report"))
	rec := httptest.NewRecorder()

	s.handlePublishFinal(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	item := <-s.work
	go func() {
		_ = item.run(context.Background())
		called <- struct{}{}
	}()
	<-called
	if publisher.finalReport != "final report" {
		t.Fatalf("final report was not published: %#v", publisher)
	}
}

func TestHandlePublishFinalRejectsOversizedReport(t *testing.T) {
	s := &Server{
		pipeline: &pipeline.Pipeline{Jobs: pipeline.NewMemoryRunStore()},
		work:     make(chan workItem, 1),
	}
	req := httptest.NewRequest(http.MethodPost, "/publish/final?run=p!7", strings.NewReader(strings.Repeat("x", int(reportMaxBodyBytes)+1)))
	rec := httptest.NewRecorder()

	s.handlePublishFinal(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(s.work) != 0 {
		t.Fatal("oversized final report should not enqueue work")
	}
}

func TestHandleApproveAcceptsRunID(t *testing.T) {
	called := make(chan struct{}, 1)
	store := pipeline.NewMemoryRunStore()
	reqRun := review.Request{Provider: "github", ProjectID: "owner/repo", MRIID: 7, ChangeID: "7"}
	run, err := store.Start(context.Background(), reqRun)
	if err != nil {
		t.Fatal(err)
	}
	rc := review.NewContext(reqRun)
	rc.Source.SCM = &review.SCMContext{Provider: "github", Repository: "owner/repo", ProjectID: "owner/repo", MRIID: 7, ChangeID: "7"}
	rc.DraftReport = "draft"
	if err := store.SaveContext(context.Background(), run.ID, rc); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(context.Background(), run.ID, pipeline.StatusDrafted, nil); err != nil {
		t.Fatal(err)
	}
	publisher := &fakeAppPublisher{}
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Jobs:         store,
			SCM:          fakeAppSCM{},
			SCMPublisher: publisher,
			Memory:       fakeMemory{},
		},
		work: make(chan workItem, 1),
	}
	req := httptest.NewRequest(http.MethodPost, "/approve?run=owner%2Frepo%217", strings.NewReader("approved report"))
	rec := httptest.NewRecorder()

	s.handleApprove(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	item := <-s.work
	go func() {
		_ = item.run(context.Background())
		called <- struct{}{}
	}()
	<-called
	if publisher.finalReport != "approved report" {
		t.Fatalf("approved report was not published: %#v", publisher)
	}
	updated, err := store.Get(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.HILApproved || updated.Status != pipeline.StatusFinalized {
		t.Fatalf("run was not approved through run id: %#v", updated)
	}
}

func TestHandleApproveDoesNotPublishUndraftedRun(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	reqRun := review.Request{Provider: "github", ProjectID: "owner/repo", MRIID: 7, ChangeID: "7"}
	run, err := store.Start(context.Background(), reqRun)
	if err != nil {
		t.Fatal(err)
	}
	rc := review.NewContext(reqRun)
	rc.Source.SCM = &review.SCMContext{Provider: "github", Repository: "owner/repo", ProjectID: "owner/repo", MRIID: 7, ChangeID: "7"}
	if err := store.SaveContext(context.Background(), run.ID, rc); err != nil {
		t.Fatal(err)
	}
	publisher := &fakeAppPublisher{}
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Jobs:         store,
			SCM:          fakeAppSCM{},
			SCMPublisher: publisher,
			Memory:       fakeMemory{},
		},
		work: make(chan workItem, 1),
	}
	req := httptest.NewRequest(http.MethodPost, "/approve?run=owner%2Frepo%217", strings.NewReader("approved report"))
	rec := httptest.NewRecorder()

	s.handleApprove(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected queued approval, got %d: %s", rec.Code, rec.Body.String())
	}
	item := <-s.work
	if err := item.run(context.Background()); err == nil || !strings.Contains(err.Error(), "draft report required") {
		t.Fatalf("expected draft requirement error, got %v", err)
	}
	if publisher.finalReport != "" {
		t.Fatalf("publisher should not run for undrafted approval: %#v", publisher)
	}
}

func TestHandleApproveRejectsOversizedReport(t *testing.T) {
	s := &Server{
		pipeline: &pipeline.Pipeline{Jobs: pipeline.NewMemoryRunStore()},
		work:     make(chan workItem, 1),
	}
	req := httptest.NewRequest(http.MethodPost, "/approve?run=owner%2Frepo%217", strings.NewReader(strings.Repeat("x", int(reportMaxBodyBytes)+1)))
	rec := httptest.NewRecorder()

	s.handleApprove(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(s.work) != 0 {
		t.Fatal("oversized approval report should not enqueue work")
	}
}

type fakeReducer struct {
	err error
}

func (f fakeReducer) Reduce(context.Context, *review.Context) error {
	return f.err
}

func (f fakeReducer) Check(context.Context) error {
	return f.err
}

type fakeMemory struct{}

func (fakeMemory) Recall(context.Context, review.Request) (pipeline.Recall, error) {
	return pipeline.Recall{}, nil
}

func (fakeMemory) ProposeUpdate(context.Context, *review.Context) (pipeline.UpdateProposal, error) {
	return pipeline.UpdateProposal{}, nil
}

func (fakeMemory) Write(context.Context, pipeline.UpdateProposal) error {
	return nil
}

func (fakeMemory) Check(context.Context) error {
	return nil
}

type proposalMemory struct{}

func (proposalMemory) Recall(context.Context, review.Request) (pipeline.Recall, error) {
	return pipeline.Recall{}, nil
}

func (proposalMemory) ProposeUpdate(_ context.Context, rc *review.Context) (pipeline.UpdateProposal, error) {
	if rc == nil || !rc.HILApproved {
		return pipeline.UpdateProposal{}, errors.New("approval required")
	}
	return pipeline.UpdateProposal{Conventions: []string{rc.FinalReport}}, nil
}

func (proposalMemory) Write(context.Context, pipeline.UpdateProposal) error {
	return nil
}

func (proposalMemory) Check(context.Context) error {
	return nil
}

type streamingProvider struct{}

func (streamingProvider) Name() string { return "fake" }

func (streamingProvider) Complete(context.Context, orchestrator.LLMRequest) (string, error) {
	return "complete", nil
}

func (streamingProvider) Stream(_ context.Context, _ orchestrator.LLMRequest, emit orchestrator.StreamHandler) error {
	if err := emit("stream "); err != nil {
		return err
	}
	return emit("reply")
}

type staticResponseProvider struct {
	response string
}

func (p staticResponseProvider) Name() string { return "fake" }

func (p staticResponseProvider) Complete(context.Context, orchestrator.LLMRequest) (string, error) {
	return p.response, nil
}

type fakeAppSCM struct{}

func (fakeAppSCM) Enrich(context.Context, review.Request) (*review.SCMContext, error) {
	return &review.SCMContext{Provider: "gitlab", ProjectID: "p", MRIID: 7, ChangeID: "7"}, nil
}

type fakeAppPublisher struct {
	finalReport string
}

func (fakeAppPublisher) PublishDraft(context.Context, *review.SCMContext, string) error {
	return nil
}

func (p *fakeAppPublisher) PublishFinal(_ context.Context, _ *review.SCMContext, report string) error {
	p.finalReport = report
	return nil
}
