package orchestrator

import (
	"fmt"
	"strings"

	"github.com/Y4NN777/7review/agent/config"
	llmproviders "github.com/Y4NN777/7review/agent/llm/providers"
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
		p["anthropic"] = llmproviders.NewAnthropic(cfg.AnthropicAPIKey, "")
	}
	if cfg.OpenAIAPIKey != "" {
		p["openai"] = llmproviders.NewOpenAI(cfg.OpenAIAPIKey, "")
	}
	if cfg.OpenRouterAPIKey != "" {
		p["openrouter"] = llmproviders.NewOpenRouter(cfg.OpenRouterAPIKey, cfg.OpenRouterBaseURL)
	}
	if cfg.DeepSeekAPIKey != "" {
		p["deepseek"] = llmproviders.NewDeepSeek(cfg.DeepSeekAPIKey, cfg.DeepSeekBaseURL)
	}
	if cfg.MistralAPIKey != "" {
		p["mistral"] = llmproviders.NewMistral(cfg.MistralAPIKey, "")
	}
	if cfg.GeminiAPIKey != "" {
		p["gemini"] = llmproviders.NewGemini(cfg.GeminiAPIKey, "")
	}
	if cfg.ProviderAPIKey != "" {
		registerSingleProvider(p, cfg.Provider, cfg.ProviderAPIKey, cfg.ProviderBaseURL)
	}
	// Ollama is available when the deployment explicitly provides a base URL.
	if cfg.OllamaBaseURL != "" {
		p["ollama"] = llmproviders.NewOllama(cfg.OllamaBaseURL)
	}
	// openai_compat covers Together AI, Groq, vLLM, LM Studio, etc.
	if cfg.ProviderBaseURL != "" && cfg.Provider == "openai_compat" {
		p["openai_compat"] = llmproviders.NewOpenAICompat(cfg.ProviderAPIKey, cfg.ProviderBaseURL)
	}

	return p
}

func registerSingleProvider(providers map[string]LLMProvider, provider, apiKey, baseURL string) {
	if _, exists := providers[provider]; exists {
		return
	}
	switch provider {
	case "anthropic":
		providers["anthropic"] = llmproviders.NewAnthropic(apiKey, baseURL)
	case "openai":
		providers["openai"] = llmproviders.NewOpenAI(apiKey, baseURL)
	case "openrouter":
		providers["openrouter"] = llmproviders.NewOpenRouter(apiKey, baseURL)
	case "deepseek":
		providers["deepseek"] = llmproviders.NewDeepSeek(apiKey, baseURL)
	case "mistral":
		providers["mistral"] = llmproviders.NewMistral(apiKey, baseURL)
	case "gemini":
		providers["gemini"] = llmproviders.NewGemini(apiKey, baseURL)
	case "openai_compat":
		if baseURL != "" {
			providers["openai_compat"] = llmproviders.NewOpenAICompat(apiKey, baseURL)
		}
	}
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

	if err := validateConfiguredProviders(orchCfg, providerMap); err != nil {
		return nil, err
	}

	return NewOrchestrator(orchCfg, providerMap), nil
}

func validateConfiguredProviders(cfg *OrchestratorConfig, providers map[string]LLMProvider) error {
	for role, roleCfg := range cfg.Roles {
		chain := append([]ModelSpec{roleCfg.Primary}, roleCfg.Fallbacks...)
		if len(chain) == 0 {
			return fmt.Errorf("orchestrator: role %q has no provider chain", role)
		}
		if !chainHasRegisteredProvider(chain, providers) {
			return fmt.Errorf("orchestrator: role %q has no registered providers from chain %s", role, describeProviderChain(chain))
		}
	}
	return nil
}

func chainHasRegisteredProvider(chain []ModelSpec, providers map[string]LLMProvider) bool {
	for _, spec := range chain {
		if _, ok := providers[spec.Provider]; ok {
			return true
		}
	}
	return false
}

func describeProviderChain(chain []ModelSpec) string {
	parts := make([]string, 0, len(chain))
	for _, spec := range chain {
		parts = append(parts, fmt.Sprintf("%s@%s", spec.Model, spec.Provider))
	}
	return strings.Join(parts, ", ")
}
