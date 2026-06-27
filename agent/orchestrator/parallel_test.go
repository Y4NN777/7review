package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/Y4NN777/7review/agent/llm"
	"github.com/Y4NN777/7review/agent/review"
)

type recordingProvider struct {
	prefix string
}

func (p recordingProvider) Name() string { return p.prefix }

func (p recordingProvider) Complete(_ context.Context, req llm.LLMRequest) (string, error) {
	return p.prefix + ":" + req.Model, nil
}

func TestCompleteParallelUsesRegisteredFallbackWhenPrimaryUnavailable(t *testing.T) {
	orch := NewOrchestrator(&OrchestratorConfig{
		Roles: map[ModelRole]RoleConfig{
			RoleReasoner: {
				Primary:   ModelSpec{Model: "claude-sonnet", Provider: "anthropic"},
				Fallbacks: []ModelSpec{{Model: "gpt-4o", Provider: "openai"}},
				MaxTokens: 2048,
				Parallel:  true,
			},
		},
	}, map[string]LLMProvider{
		"openai": recordingProvider{prefix: "openai"},
	})
	rc := review.NewContext(review.Request{Provider: "test"})
	err := orch.CompleteParallel(
		context.Background(),
		rc,
		"system",
		[][]review.FileDiff{
			{{Path: "a.go", TokenCount: 3000}},
			{{Path: "b.go", TokenCount: 100}},
		},
		func(batch []review.FileDiff) string {
			return batch[0].Path
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(rc.AllFindings(), "\n"); strings.Count(got, "openai:gpt-4o") != 2 {
		t.Fatalf("expected both batches to use registered fallback, got %q", got)
	}
}
