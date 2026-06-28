//go:build live_smoke

package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/orchestrator"
	"github.com/Y4NN777/7review/agent/review"
)

func TestLiveSmokeReviewPipelineWithConfiguredOllamaModels(t *testing.T) {
	if os.Getenv("RUN_LIVE_SMOKE") != "1" {
		t.Skip("set RUN_LIVE_SMOKE=1 to run the live Ollama review pipeline smoke test")
	}

	ollamaURL := firstNonEmpty(os.Getenv("OLLAMA_BASE_URL"), "http://127.0.0.1:11434")
	orchestratorConfig := resolveLiveSmokePath(t, firstNonEmpty(os.Getenv("ORCHESTRATOR_CONFIG"), filepath.Join("..", "..", "orchestrator.yaml")))

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	orch, err := orchestrator.BuildOrchestrator(&config.Config{
		Provider:               "ollama",
		OllamaBaseURL:          ollamaURL,
		ReviewModel:            "deepseek-coder-v2:16b",
		SmallModel:             "qwen2.5-coder-7b-16k:latest",
		OrchestratorConfigPath: orchestratorConfig,
	})
	if err != nil {
		t.Fatal(err)
	}

	store := NewMemoryRunStore()
	publisher := &draftRecordingPublisher{}
	req := review.Request{
		Provider:   "github",
		ProjectID:  "smoke/repo",
		Repository: "smoke/repo",
		ChangeID:   "101",
		Title:      "Smoke review pipeline",
	}
	p := &Pipeline{
		Config: &config.Config{
			CorpusRoot:    t.TempDir(),
			MaxDiffTokens: 6000,
		},
		Orchestrator:     orch,
		Jobs:             store,
		Policy:           DefaultPolicyFilter{},
		FindingValidator: DefaultFindingValidator{},
		Memory:           NoopMemoryStore{},
		SCM: staticSCM{context: &review.SCMContext{
			Provider:    "github",
			ProjectID:   "smoke/repo",
			Repository:  "smoke/repo",
			ChangeID:    "101",
			Title:       "Smoke review pipeline",
			Description: "Real model smoke test for the complete review pipeline.",
			WebURL:      "https://github.example.com/smoke/repo/pull/101",
			Files: []review.ChangedFile{{
				NewPath: "internal/smoke/handler.go",
				Patch: strings.Join([]string{
					"@@ -1,5 +1,9 @@",
					"+package smoke",
					"+",
					"+func Handle(user string) string {",
					"+\treturn \"hello \" + user",
					"+}",
				}, "\n"),
			}},
		}},
		SCMPublisher:   publisher,
		ContextReducer: NoopContextReducer{},
	}

	if err := p.Run(ctx, req); err != nil {
		t.Fatal(err)
	}
	run, err := store.Get(ctx, "smoke/repo!101")
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != StatusDrafted || strings.TrimSpace(run.DraftReport) == "" {
		t.Fatalf("expected drafted run with report, got status=%s report=%q", run.Status, run.DraftReport)
	}
	if publisher.draftSource == nil || publisher.draftSource.Provider != "github" || strings.TrimSpace(publisher.draftReport) == "" {
		t.Fatalf("draft was not published through publisher: %#v report=%q", publisher.draftSource, publisher.draftReport)
	}
	for _, eventType := range []string{
		"webhook_received",
		"scm_enriched",
		"skills_selected",
		"repository_knowledge_selected",
		"memory_recalled",
		"context_assembled",
		"model_review_completed",
		"findings_validated",
		"draft_published",
	} {
		if !hasRunEvent(run.Events, eventType) {
			t.Fatalf("run missing trace event %q: %#v", eventType, run.Events)
		}
	}
	if !eventMetaContains(run.Events, "model_review_completed", "providers", "ollama/deepseek-coder-v2:16b") {
		t.Fatalf("model review did not use configured Ollama reasoner route: %#v", run.Events)
	}
}

func resolveLiveSmokePath(t *testing.T, path string) string {
	t.Helper()
	if filepath.IsAbs(path) {
		return path
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}
	for _, prefix := range []string{filepath.Join("..", ".."), filepath.Join("..", "..", "..")} {
		candidate := filepath.Join(prefix, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return path
}
