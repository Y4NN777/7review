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

func TestBuildOrchestratorAppliesEnvRoleOverridesOnConfigFile(t *testing.T) {
	t.Setenv("PROVIDER", "ollama")
	t.Setenv("REVIEW_MODEL", "qwen2.5-coder-7b-16k:latest")
	t.Setenv("SMALL_MODEL", "qwen2.5-coder:1.5b")
	cfgPath := filepath.Join(t.TempDir(), "orchestrator.yaml")
	if err := os.WriteFile(cfgPath, []byte(`
roles:
  reasoner:
    primary: "claude-sonnet@anthropic"
    fallbacks:
      - "gpt-4o@openai"
    max_tokens: 4096
    parallel: true
    max_parallel: 4
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
		Provider:               "ollama",
		ReviewModel:            "qwen2.5-coder-7b-16k:latest",
		SmallModel:             "qwen2.5-coder:1.5b",
		OllamaBaseURL:          "http://127.0.0.1:11434",
	}

	orch, err := BuildOrchestrator(cfg)
	if err != nil {
		t.Fatalf("expected env-selected ollama provider to satisfy YAML config: %v", err)
	}
	reasoner := orch.cfg.Roles[RoleReasoner]
	if reasoner.Primary.Provider != "ollama" || reasoner.Primary.Model != "qwen2.5-coder-7b-16k:latest" {
		t.Fatalf("reasoner primary was not overridden: %#v", reasoner.Primary)
	}
	formatter := orch.cfg.Roles[RoleFormatter]
	if formatter.Primary.Provider != "ollama" || formatter.Primary.Model != "qwen2.5-coder:1.5b" {
		t.Fatalf("formatter primary was not overridden: %#v", formatter.Primary)
	}
	if len(reasoner.Fallbacks) != 1 || reasoner.Fallbacks[0].Provider != "openai" {
		t.Fatalf("expected YAML fallbacks to remain available: %#v", reasoner.Fallbacks)
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
