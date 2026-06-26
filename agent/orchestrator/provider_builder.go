package orchestrator

import (
	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/llm/providers"
)

// BuildProviders instantiates every provider that has credentials in cfg.
// Returns a map keyed by provider name, ready for the Orchestrator.
//
// Only providers with non-empty credentials are registered.
// This means you can safely configure a multi-provider orchestrator config
// and only the providers you have keys for will be available.
func BuildProviders(cfg *config.Config) map[string]LLMProvider {
	p := make(map[string]LLMProvider)

	if cfg.AnthropicAPIKey != "" {
		p["anthropic"] = providers.NewAnthropic(cfg.AnthropicAPIKey, "")
	}
	if cfg.OpenAIAPIKey != "" {
		p["openai"] = providers.NewOpenAI(cfg.OpenAIAPIKey, "")
	}
	if cfg.MistralAPIKey != "" {
		p["mistral"] = providers.NewMistral(cfg.MistralAPIKey, "")
	}
	if cfg.GeminiAPIKey != "" {
		p["gemini"] = providers.NewGemini(cfg.GeminiAPIKey, "")
	}
	// Ollama is always available if a base URL is set (no auth needed).
	if cfg.OllamaBaseURL != "" {
		p["ollama"] = providers.NewOllama(cfg.OllamaBaseURL)
	}
	// openai_compat covers Together AI, Groq, vLLM, LM Studio, etc.
	if cfg.ProviderBaseURL != "" && cfg.Provider == "openai_compat" {
		p["openai_compat"] = providers.NewOpenAICompat(cfg.ProviderAPIKey, cfg.ProviderBaseURL)
	}

	return p
}

// BuildOrchestrator creates the Orchestrator from config.
// If OrchestratorConfigPath is set, it loads role→provider chains from YAML.
// Otherwise falls back to single-provider mode using Provider/ReviewModel/SmallModel.
func BuildOrchestrator(cfg *config.Config) (*Orchestrator, error) {
	providerMap := BuildProviders(cfg)

	var orchCfg *OrchestratorConfig
	if cfg.OrchestratorConfigPath != "" {
		var err error
		orchCfg, err = loadOrchestratorConfigFromFile(cfg.OrchestratorConfigPath)
		if err != nil {
			return nil, err
		}
	} else {
		orchCfg = DefaultOrchestratorConfig(cfg.ReviewModel, cfg.SmallModel, cfg.Provider)
	}

	return NewOrchestrator(orchCfg, providerMap), nil
}
