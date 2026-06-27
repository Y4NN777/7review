package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Y4NN777/7review/agent/config"
)

func TestBuildOrchestratorFailsWhenDefaultProviderIsUnavailable(t *testing.T) {
	cfg := &config.Config{
		Provider:    "anthropic",
		ReviewModel: "review-model",
		SmallModel:  "small-model",
	}

	_, err := BuildOrchestrator(cfg)
	if err == nil {
		t.Fatal("expected unavailable provider error")
	}
	if !strings.Contains(err.Error(), "has no registered providers") || !strings.Contains(err.Error(), "anthropic") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildOrchestratorAcceptsConfiguredFallbackProvider(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "orchestrator.yaml")
	if err := os.WriteFile(cfgPath, []byte(`
roles:
  reasoner:
    primary: "claude-sonnet@anthropic"
    fallbacks:
      - "gpt-4o@openai"
    max_tokens: 4096
  formatter:
    primary: "claude-haiku@anthropic"
    fallbacks:
      - "gpt-4o-mini@openai"
    max_tokens: 2048
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		OrchestratorConfigPath: cfgPath,
		OpenAIAPIKey:           "openai-key",
	}

	if _, err := BuildOrchestrator(cfg); err != nil {
		t.Fatalf("expected fallback provider to satisfy each role: %v", err)
	}
}

func TestBuildOrchestratorUsesSingleProviderAPIKey(t *testing.T) {
	cfg := &config.Config{
		Provider:       "anthropic",
		ProviderAPIKey: "single-provider-key",
		ReviewModel:    "review-model",
		SmallModel:     "small-model",
	}

	if _, err := BuildOrchestrator(cfg); err != nil {
		t.Fatalf("expected PROVIDER_API_KEY to register the selected provider: %v", err)
	}
}

func TestBuildProvidersRegistersOpenRouterAndDeepSeek(t *testing.T) {
	providers := BuildProviders(&config.Config{
		OpenRouterAPIKey:  "openrouter-key",
		OpenRouterBaseURL: "http://openrouter.test",
		DeepSeekAPIKey:    "deepseek-key",
		DeepSeekBaseURL:   "http://deepseek.test",
	})
	for _, name := range []string{"openrouter", "deepseek"} {
		if providers[name] == nil {
			t.Fatalf("expected provider %q to be registered in %#v", name, providers)
		}
		if providers[name].Name() != name {
			t.Fatalf("expected provider name %q, got %q", name, providers[name].Name())
		}
	}
}

func TestBuildOrchestratorUsesDeepSeekSingleProvider(t *testing.T) {
	cfg := &config.Config{
		Provider:       "deepseek",
		ProviderAPIKey: "deepseek-key",
		ReviewModel:    "deepseek-chat",
		SmallModel:     "deepseek-chat",
	}

	if _, err := BuildOrchestrator(cfg); err != nil {
		t.Fatalf("expected deepseek single provider to register: %v", err)
	}
}
