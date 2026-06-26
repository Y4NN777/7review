package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime settings, loaded from environment variables.
type Config struct {
	// ── Pipeline ──────────────────────────────────────────────────────────
	SkillsDir        string
	MemoryDir        string
	InstructionsPath string
	MaxDiffTokens    int
	HILChannel       string
	ListenAddr       string

	// ── GitLab ────────────────────────────────────────────────────────────
	GitLabURL     string
	GitLabToken   string
	WebhookSecret string

	// ── Multi-LLM Orchestration ───────────────────────────────────────────
	// OrchestratorConfigPath points to a YAML file defining role→provider chains.
	// If empty, DefaultOrchestratorConfig is used with the env vars below.
	OrchestratorConfigPath string

	// Fallback single-provider mode (used when no orchestrator config file exists).
	// PROVIDER sets which provider handles all roles.
	// Supported: anthropic | openai | mistral | gemini | ollama | openai_compat
	Provider        string
	ProviderAPIKey  string
	ProviderBaseURL string // optional override (e.g. for openai_compat or Ollama)

	// ReviewModel and SmallModel are the model strings for the fallback single-provider mode.
	ReviewModel string // used for RoleReasoner
	SmallModel  string // used for RoleFormatter and RoleCompactor

	// Per-provider API keys — used when the orchestrator config references
	// multiple providers. All optional; only needed if that provider is used.
	AnthropicAPIKey string
	OpenAIAPIKey    string
	MistralAPIKey   string
	GeminiAPIKey    string
	OllamaBaseURL   string // default: http://localhost:11434
}

// LoadConfig reads all required settings from the environment.
func LoadConfig() (*Config, error) {
	c := &Config{
		// Pipeline
		SkillsDir:        getEnv("SKILLS_DIR", "./agent/skills"),
		MemoryDir:        getEnv("MEMORY_DIR", "./agent/memory"),
		InstructionsPath: getEnv("INSTRUCTIONS_PATH", "./agent/instructions.md"),
		MaxDiffTokens:    getEnvInt("MAX_DIFF_TOKENS", 6000),
		HILChannel:       getEnv("HIL_CHANNEL", "gitlab_note"),
		ListenAddr:       getEnv("LISTEN_ADDR", ":8080"),

		// GitLab
		GitLabURL:     os.Getenv("GITLAB_URL"),
		GitLabToken:   os.Getenv("GITLAB_TOKEN"),
		WebhookSecret: os.Getenv("GITLAB_WEBHOOK_SECRET"),

		// Orchestration
		OrchestratorConfigPath: os.Getenv("ORCHESTRATOR_CONFIG"),

		// Fallback single-provider mode
		Provider:        getEnv("PROVIDER", "anthropic"),
		ProviderAPIKey:  os.Getenv("PROVIDER_API_KEY"),
		ProviderBaseURL: os.Getenv("PROVIDER_BASE_URL"),
		ReviewModel:     getEnv("REVIEW_MODEL", "claude-sonnet-4-6"),
		SmallModel:      getEnv("SMALL_MODEL", "claude-haiku-4-5-20251001"),

		// Per-provider keys (multi-provider orchestration)
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		MistralAPIKey:   os.Getenv("MISTRAL_API_KEY"),
		GeminiAPIKey:    os.Getenv("GEMINI_API_KEY"),
		OllamaBaseURL:   getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
	}

	var missing []string
	for _, req := range []struct{ name, val string }{
		{"GITLAB_URL", c.GitLabURL},
		{"GITLAB_TOKEN", c.GitLabToken},
		{"GITLAB_WEBHOOK_SECRET", c.WebhookSecret},
	} {
		if req.val == "" {
			missing = append(missing, req.name)
		}
	}

	// At least one LLM provider key must be present.
	hasProvider := c.AnthropicAPIKey != "" || c.OpenAIAPIKey != "" ||
		c.MistralAPIKey != "" || c.GeminiAPIKey != "" ||
		c.OllamaBaseURL != "" || c.ProviderAPIKey != ""
	if !hasProvider {
		missing = append(missing, "at least one of: ANTHROPIC_API_KEY, OPENAI_API_KEY, MISTRAL_API_KEY, GEMINI_API_KEY, OLLAMA_BASE_URL")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %v", missing)
	}

	return c, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
