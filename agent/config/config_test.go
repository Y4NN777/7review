package config

import (
	"strings"
	"testing"
)

func TestLoadConfig_RequiresHeadroomAndMemPalace(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	t.Setenv("HEADROOM_URL", "")
	t.Setenv("MEMPALACE_URL", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected missing sidecar config error")
	}
	if !strings.Contains(err.Error(), "HEADROOM_URL") || !strings.Contains(err.Error(), "MEMPALACE_URL") {
		t.Fatalf("expected sidecar names in error, got %v", err)
	}
}

func TestLoadConfig_SidecarTimeoutDefaults(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	t.Setenv("HEADROOM_URL", "http://headroom")
	t.Setenv("MEMPALACE_URL", "http://mempalace")
	t.Setenv("OLLAMA_BASE_URL", "http://ollama:11434")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HeadroomTimeout != 5000 || cfg.MemPalaceTimeout != 5000 {
		t.Fatalf("unexpected timeouts: headroom=%d mempalace=%d", cfg.HeadroomTimeout, cfg.MemPalaceTimeout)
	}
	if cfg.WebhookJobTimeout != 15*60*1000 {
		t.Fatalf("unexpected webhook job timeout: %d", cfg.WebhookJobTimeout)
	}
	if cfg.ReadHeaderTimeout != 5000 || cfg.ReadTimeout != 30000 || cfg.WriteTimeout != 120000 || cfg.IdleTimeout != 120000 {
		t.Fatalf("unexpected HTTP timeout defaults: header=%d read=%d write=%d idle=%d", cfg.ReadHeaderTimeout, cfg.ReadTimeout, cfg.WriteTimeout, cfg.IdleTimeout)
	}
	if cfg.APIToken != "agent-token" {
		t.Fatalf("unexpected API token")
	}
	if cfg.MemoryDir != "./.7review" {
		t.Fatalf("unexpected memory dir %q", cfg.MemoryDir)
	}
	if cfg.CorpusRoot != "." {
		t.Fatalf("unexpected corpus root %q", cfg.CorpusRoot)
	}
	if cfg.MaxSupportingCorpusSections != 3 {
		t.Fatalf("unexpected supporting corpus cap %d", cfg.MaxSupportingCorpusSections)
	}
	if cfg.EmbeddingModel != "nomic-embed-text:latest" {
		t.Fatalf("unexpected embedding model %q", cfg.EmbeddingModel)
	}
}

func TestLoadConfig_CorpusRootOverride(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	t.Setenv("HEADROOM_URL", "http://headroom")
	t.Setenv("MEMPALACE_URL", "http://mempalace")
	t.Setenv("OLLAMA_BASE_URL", "http://ollama:11434")
	t.Setenv("CORPUS_ROOT", "/workspace/repo")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CorpusRoot != "/workspace/repo" {
		t.Fatalf("unexpected corpus root %q", cfg.CorpusRoot)
	}
}

func TestLoadConfig_MaxSupportingCorpusSectionsOverride(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	t.Setenv("HEADROOM_URL", "http://headroom")
	t.Setenv("MEMPALACE_URL", "http://mempalace")
	t.Setenv("OLLAMA_BASE_URL", "http://ollama:11434")
	t.Setenv("MAX_SUPPORTING_CORPUS_SECTIONS", "5")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxSupportingCorpusSections != 5 {
		t.Fatalf("unexpected supporting corpus cap %d", cfg.MaxSupportingCorpusSections)
	}
}

func TestLoadConfig_OllamaDefaultsUseHarnessRouting(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	t.Setenv("HEADROOM_URL", "http://headroom")
	t.Setenv("MEMPALACE_URL", "http://mempalace")
	t.Setenv("PROVIDER", "ollama")
	t.Setenv("OLLAMA_BASE_URL", "http://ollama:11434")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReviewModel != "deepseek-coder-v2:16b" || cfg.SmallModel != "qwen2.5-coder-7b-16k:latest" {
		t.Fatalf("unexpected ollama harness defaults: review=%q small=%q", cfg.ReviewModel, cfg.SmallModel)
	}
	if cfg.EmbeddingModel != "nomic-embed-text:latest" {
		t.Fatalf("unexpected embedding model %q", cfg.EmbeddingModel)
	}
}

func TestLoadConfig_HTTPTimeoutOverrides(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	t.Setenv("HEADROOM_URL", "http://headroom")
	t.Setenv("MEMPALACE_URL", "http://mempalace")
	t.Setenv("OLLAMA_BASE_URL", "http://ollama:11434")
	t.Setenv("HTTP_READ_HEADER_TIMEOUT_MS", "7000")
	t.Setenv("HTTP_READ_TIMEOUT_MS", "31000")
	t.Setenv("HTTP_WRITE_TIMEOUT_MS", "121000")
	t.Setenv("HTTP_IDLE_TIMEOUT_MS", "122000")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReadHeaderTimeout != 7000 || cfg.ReadTimeout != 31000 || cfg.WriteTimeout != 121000 || cfg.IdleTimeout != 122000 {
		t.Fatalf("unexpected HTTP timeout overrides: %#v", cfg)
	}
}

func TestLoadConfig_RejectsInvalidNumericRuntimeSettings(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	t.Setenv("HEADROOM_URL", "http://headroom")
	t.Setenv("MEMPALACE_URL", "http://mempalace")
	t.Setenv("OLLAMA_BASE_URL", "http://ollama:11434")
	t.Setenv("WEBHOOK_WORKERS", "0")
	t.Setenv("WEBHOOK_QUEUE_SIZE", "-1")
	t.Setenv("MAX_SUPPORTING_CORPUS_SECTIONS", "0")
	t.Setenv("HEADROOM_TIMEOUT_MS", "not-a-number")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected invalid numeric config error")
	}
	for _, want := range []string{"WEBHOOK_WORKERS must be greater than zero", "WEBHOOK_QUEUE_SIZE must be greater than zero", "MAX_SUPPORTING_CORPUS_SECTIONS must be greater than zero", "HEADROOM_TIMEOUT_MS must be an integer"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in %v", want, err)
		}
	}
}

func TestLoadConfig_RequiresExplicitModelProvider(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	t.Setenv("HEADROOM_URL", "http://headroom")
	t.Setenv("MEMPALACE_URL", "http://mempalace")
	t.Setenv("OLLAMA_BASE_URL", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected missing provider error")
	}
	if !strings.Contains(err.Error(), "OLLAMA_BASE_URL") {
		t.Fatalf("expected provider guidance in error, got %v", err)
	}
}

func TestLoadConfig_OpenRouterAndDeepSeek(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	t.Setenv("HEADROOM_URL", "http://headroom")
	t.Setenv("MEMPALACE_URL", "http://mempalace")
	t.Setenv("OPENROUTER_API_KEY", "openrouter-key")
	t.Setenv("DEEPSEEK_API_KEY", "deepseek-key")
	t.Setenv("PROVIDER", "openrouter")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenRouterAPIKey != "openrouter-key" || cfg.OpenRouterBaseURL != "https://openrouter.ai/api" {
		t.Fatalf("unexpected openrouter config: %#v", cfg)
	}
	if cfg.DeepSeekAPIKey != "deepseek-key" || cfg.DeepSeekBaseURL != "https://api.deepseek.com" {
		t.Fatalf("unexpected deepseek config: %#v", cfg)
	}
	if cfg.ReviewModel != "openai/gpt-4o" || cfg.SmallModel != "openai/gpt-4o-mini" {
		t.Fatalf("unexpected openrouter default models: review=%q small=%q", cfg.ReviewModel, cfg.SmallModel)
	}
}

func TestLoadConfig_EmbeddingModelOverride(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("REVIEW_API_TOKEN", "agent-token")
	t.Setenv("HEADROOM_URL", "http://headroom")
	t.Setenv("MEMPALACE_URL", "http://mempalace")
	t.Setenv("OLLAMA_BASE_URL", "http://ollama:11434")
	t.Setenv("EMBEDDING_MODEL", "custom-embed")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EmbeddingModel != "custom-embed" {
		t.Fatalf("unexpected embedding model %q", cfg.EmbeddingModel)
	}
}
