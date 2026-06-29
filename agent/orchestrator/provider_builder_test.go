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
	t.Setenv("REVIEW_MODEL", "deepseek-coder-v2:16b")
	t.Setenv("SMALL_MODEL", "qwen2.5-coder-7b-16k:latest")
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
		ReviewModel:            "deepseek-coder-v2:16b",
		SmallModel:             "qwen2.5-coder-7b-16k:latest",
		OllamaBaseURL:          "http://127.0.0.1:11434",
	}

	orch, err := BuildOrchestrator(cfg)
	if err != nil {
		t.Fatalf("expected env-selected ollama provider to satisfy YAML config: %v", err)
	}
	reasoner := orch.cfg.Roles[RoleReasoner]
	if reasoner.Primary.Provider != "ollama" || reasoner.Primary.Model != "deepseek-coder-v2:16b" {
		t.Fatalf("reasoner primary was not overridden: %#v", reasoner.Primary)
	}
	formatter := orch.cfg.Roles[RoleFormatter]
	if formatter.Primary.Provider != "ollama" || formatter.Primary.Model != "qwen2.5-coder-7b-16k:latest" {
		t.Fatalf("formatter primary was not overridden: %#v", formatter.Primary)
	}
	if len(reasoner.Fallbacks) != 0 || len(formatter.Fallbacks) != 0 {
		t.Fatalf("expected env override to replace YAML fallback chain: reasoner=%#v formatter=%#v", reasoner.Fallbacks, formatter.Fallbacks)
	}
}

func TestBuildOrchestratorIgnoresEmptyComposeModelOverrides(t *testing.T) {
	t.Setenv("PROVIDER", "ollama")
	t.Setenv("REVIEW_MODEL", "")
	t.Setenv("SMALL_MODEL", "")
	cfgPath := filepath.Join(t.TempDir(), "orchestrator.yaml")
	if err := os.WriteFile(cfgPath, []byte(`
roles:
  reasoner:
    primary: "claude-sonnet@anthropic"
    max_tokens: 4096
  formatter:
    primary: "claude-haiku@anthropic"
    max_tokens: 2048
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		OrchestratorConfigPath: cfgPath,
		Provider:               "ollama",
		ReviewModel:            "deepseek-coder-v2:16b",
		SmallModel:             "qwen2.5-coder-7b-16k:latest",
		OllamaBaseURL:          "http://127.0.0.1:11434",
	}

	orch, err := BuildOrchestrator(cfg)
	if err != nil {
		t.Fatalf("expected provider-only override to use configured default models: %v", err)
	}
	if got := orch.cfg.Roles[RoleReasoner].Primary; got.Provider != "ollama" || got.Model != "deepseek-coder-v2:16b" {
		t.Fatalf("unexpected reasoner primary: %#v", got)
	}
	if got := orch.cfg.Roles[RoleFormatter].Primary; got.Provider != "ollama" || got.Model != "qwen2.5-coder-7b-16k:latest" {
		t.Fatalf("unexpected formatter primary: %#v", got)
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
