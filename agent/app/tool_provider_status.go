package app

import (
	"context"
	"strings"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/orchestrator"
)

type providerStatusDTO struct {
	Mode               string              `json:"mode"`
	ActiveProvider     string              `json:"active_provider"`
	OrchestratorConfig string              `json:"orchestrator_config,omitempty"`
	Providers          []providerStatusRow `json:"providers"`
	Roles              []roleStatusDTO     `json:"roles"`
}

type providerStatusRow struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	BaseURL    string `json:"base_url,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type roleStatusDTO struct {
	Role        string   `json:"role"`
	Primary     string   `json:"primary"`
	Fallbacks   []string `json:"fallbacks,omitempty"`
	MaxTokens   int      `json:"max_tokens"`
	Parallel    bool     `json:"parallel"`
	MaxParallel int      `json:"max_parallel,omitempty"`
}

func (r appToolRunner) ProviderStatus(context.Context) (any, error) {
	cfg := r.server.cfg
	status := providerStatusDTO{
		Mode:      "single-provider",
		Providers: configuredProviderDTOs(cfg),
		Roles:     configuredRoleDTOs(cfg, nil),
	}
	if cfg != nil {
		status.OrchestratorConfig = cfg.OrchestratorConfigPath
		if cfg.OrchestratorConfigPath != "" {
			status.Mode = "orchestrator"
		} else {
			status.ActiveProvider = cfg.Provider
		}
	}
	if r.server != nil && r.server.pipeline != nil && r.server.pipeline.Orchestrator != nil {
		status.Roles = configuredRoleDTOs(cfg, r.server.pipeline.Orchestrator.RoleStatus())
	}
	return status, nil
}

func configuredProviderDTOs(cfg *config.Config) []providerStatusRow {
	if cfg == nil {
		return nil
	}
	rows := []providerStatusRow{
		{Name: "anthropic", Configured: cfg.AnthropicAPIKey != "" || (cfg.Provider == "anthropic" && cfg.ProviderAPIKey != "")},
		{Name: "openai", Configured: cfg.OpenAIAPIKey != "" || (cfg.Provider == "openai" && cfg.ProviderAPIKey != "")},
		{Name: "openrouter", Configured: cfg.OpenRouterAPIKey != "" || (cfg.Provider == "openrouter" && cfg.ProviderAPIKey != ""), BaseURL: firstNonEmpty(cfg.OpenRouterBaseURL, cfg.ProviderBaseURL)},
		{Name: "deepseek", Configured: cfg.DeepSeekAPIKey != "" || (cfg.Provider == "deepseek" && cfg.ProviderAPIKey != ""), BaseURL: firstNonEmpty(cfg.DeepSeekBaseURL, cfg.ProviderBaseURL)},
		{Name: "mistral", Configured: cfg.MistralAPIKey != "" || (cfg.Provider == "mistral" && cfg.ProviderAPIKey != "")},
		{Name: "gemini", Configured: cfg.GeminiAPIKey != "" || (cfg.Provider == "gemini" && cfg.ProviderAPIKey != "")},
		{Name: "ollama", Configured: cfg.OllamaBaseURL != "", BaseURL: cfg.OllamaBaseURL},
		{Name: "openai_compat", Configured: cfg.Provider == "openai_compat" && cfg.ProviderBaseURL != "", BaseURL: cfg.ProviderBaseURL},
	}
	for i := range rows {
		if !rows[i].Configured {
			rows[i].Reason = "credentials or endpoint not configured"
		}
	}
	return rows
}

func configuredRoleDTOs(cfg *config.Config, roles []orchestrator.RoleStatus) []roleStatusDTO {
	if len(roles) > 0 {
		out := make([]roleStatusDTO, 0, len(roles))
		for _, role := range roles {
			out = append(out, roleStatusDTO{
				Role:        role.Role,
				Primary:     role.Primary,
				Fallbacks:   append([]string(nil), role.Fallbacks...),
				MaxTokens:   role.MaxTokens,
				Parallel:    role.Parallel,
				MaxParallel: role.MaxParallel,
			})
		}
		return out
	}
	if cfg == nil {
		return nil
	}
	return []roleStatusDTO{
		{
			Role:        "reasoner",
			Primary:     cfg.ReviewModel + "@" + cfg.Provider,
			MaxTokens:   4096,
			Parallel:    true,
			MaxParallel: 4,
		},
		{
			Role:      "formatter",
			Primary:   cfg.SmallModel + "@" + cfg.Provider,
			MaxTokens: 2048,
		},
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
