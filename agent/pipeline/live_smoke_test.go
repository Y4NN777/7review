//go:build live_smoke

package pipeline

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/orchestrator"
	"github.com/Y4NN777/7review/agent/review"
)

func TestLiveSmokeReviewPipelineWithOpenRouter(t *testing.T) {
	if os.Getenv("RUN_LIVE_SMOKE") != "1" {
		t.Skip("set RUN_LIVE_SMOKE=1 to run the live OpenRouter review pipeline smoke test")
	}

	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if apiKey == "" {
		t.Fatal("OPENROUTER_API_KEY is required when RUN_LIVE_SMOKE=1")
	}
	reviewModel := firstNonEmpty(os.Getenv("REVIEW_MODEL"), "openrouter/free")
	smallModel := firstNonEmpty(os.Getenv("SMALL_MODEL"), "openrouter/free")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	orch, err := orchestrator.BuildOrchestrator(&config.Config{
		Provider:          "openrouter",
		OpenRouterAPIKey:  apiKey,
		OpenRouterBaseURL: firstNonEmpty(os.Getenv("OPENROUTER_BASE_URL"), "https://openrouter.ai/api"),
		ReviewModel:       reviewModel,
		SmallModel:        smallModel,
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
	expectedProvider := fmt.Sprintf("openrouter/%s", reviewModel)
	if !eventMetaContains(run.Events, "model_review_completed", "providers", expectedProvider) {
		t.Fatalf("model review did not use configured OpenRouter reasoner route %q: %#v", expectedProvider, run.Events)
	}
}
