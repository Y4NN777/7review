package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/Y4NN777/7review/agent/llm"
	"github.com/Y4NN777/7review/agent/review"
)

type streamingProvider struct{}

func (streamingProvider) Name() string { return "streaming" }

func (streamingProvider) Complete(context.Context, llm.LLMRequest) (string, error) {
	return "complete", nil
}

func (streamingProvider) Stream(_ context.Context, _ llm.LLMRequest, emit llm.StreamHandler) error {
	if err := emit("stream "); err != nil {
		return err
	}
	return emit("reply")
}

type completeOnlyProvider struct{}

func (completeOnlyProvider) Name() string { return "complete-only" }

func (completeOnlyProvider) Complete(context.Context, llm.LLMRequest) (string, error) {
	return "compat reply", nil
}

func TestStreamComplete_UsesNativeStreamingProvider(t *testing.T) {
	orch := NewOrchestrator(DefaultOrchestratorConfig("large", "small", "streaming"), map[string]LLMProvider{
		"streaming": streamingProvider{},
	})
	rc := review.NewContext(review.Request{Provider: "test"})
	var chunks []string
	out, err := orch.StreamComplete(context.Background(), rc, RoleFormatter, "chat", "system", "user", func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "stream reply" || strings.Join(chunks, "") != out {
		t.Fatalf("unexpected stream out=%q chunks=%q", out, strings.Join(chunks, ""))
	}
}

func TestStreamComplete_FallsBackToCompleteForNonStreamingProvider(t *testing.T) {
	orch := NewOrchestrator(DefaultOrchestratorConfig("large", "small", "complete-only"), map[string]LLMProvider{
		"complete-only": completeOnlyProvider{},
	})
	rc := review.NewContext(review.Request{Provider: "test"})
	var chunks []string
	out, err := orch.StreamComplete(context.Background(), rc, RoleFormatter, "chat", "system", "user", func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "compat reply" || strings.Join(chunks, "") != out {
		t.Fatalf("unexpected stream out=%q chunks=%q", out, strings.Join(chunks, ""))
	}
}
