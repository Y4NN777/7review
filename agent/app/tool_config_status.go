package app

import (
	"context"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/tools"
)

type configStatusDTO struct {
	ListenAddr                  string `json:"listen_addr"`
	CorpusRoot                  string `json:"corpus_root"`
	MaxSupportingCorpusSections int    `json:"max_supporting_corpus_sections"`
	MemoryDir                   string `json:"memory_dir"`
	HILChannel                  string `json:"hil_channel"`
	Provider                    string `json:"provider"`
	ReviewModel                 string `json:"review_model"`
	SmallModel                  string `json:"small_model"`
	EmbeddingModel              string `json:"embedding_model,omitempty"`
	Orchestrator                string `json:"orchestrator_config,omitempty"`
	HasGitLab                   bool   `json:"has_gitlab"`
	HasGitHub                   bool   `json:"has_github"`
	HasOpenAI                   bool   `json:"has_openai"`
	HasOpenRouter               bool   `json:"has_openrouter"`
	HasDeepSeek                 bool   `json:"has_deepseek"`
	HasAnthropic                bool   `json:"has_anthropic"`
	HasMistral                  bool   `json:"has_mistral"`
	HasGemini                   bool   `json:"has_gemini"`
	HasOllama                   bool   `json:"has_ollama"`
	HeadroomURL                 string `json:"headroom_url"`
	MemPalaceURL                string `json:"mempalace_url"`
	WebhookWorkers              int    `json:"webhook_workers"`
	WebhookQueueSize            int    `json:"webhook_queue_size"`
}

type skillStatusDTO struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	AllowedTools  string            `json:"allowed_tools,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Path          string            `json:"path"`
	Loaded        bool              `json:"loaded"`
}

func (r appToolRunner) ConfigStatus(context.Context) (any, error) {
	return configStatus(r.server.cfg), nil
}

func (r appToolRunner) ListSkills(context.Context) (any, error) {
	if r.server == nil || r.server.pipeline == nil || r.server.pipeline.SkillLoader == nil {
		return []skillStatusDTO{}, nil
	}
	out := make([]skillStatusDTO, 0, len(r.server.pipeline.SkillLoader.Skills))
	for _, skill := range r.server.pipeline.SkillLoader.Skills {
		out = append(out, skillStatusDTO{
			Name:          skill.Name,
			Description:   skill.Description,
			License:       skill.License,
			Compatibility: skill.Compatibility,
			AllowedTools:  skill.AllowedTools,
			Metadata:      skill.Metadata,
			Path:          skill.Path,
			Loaded:        true,
		})
	}
	return out, nil
}

func configStatus(cfg *config.Config) configStatusDTO {
	if cfg == nil {
		return configStatusDTO{}
	}
	return configStatusDTO{
		ListenAddr:                  cfg.ListenAddr,
		CorpusRoot:                  cfg.CorpusRoot,
		MaxSupportingCorpusSections: cfg.MaxSupportingCorpusSections,
		MemoryDir:                   cfg.MemoryDir,
		HILChannel:                  cfg.HILChannel,
		Provider:                    cfg.Provider,
		ReviewModel:                 cfg.ReviewModel,
		SmallModel:                  cfg.SmallModel,
		EmbeddingModel:              cfg.EmbeddingModel,
		Orchestrator:                cfg.OrchestratorConfigPath,
		HasGitLab:                   cfg.GitLabURL != "" && cfg.GitLabToken != "" && cfg.WebhookSecret != "",
		HasGitHub:                   cfg.GitHubAPIURL != "" && cfg.GitHubToken != "" && cfg.GitHubWebhookSecret != "",
		HasOpenAI:                   cfg.OpenAIAPIKey != "",
		HasOpenRouter:               cfg.OpenRouterAPIKey != "",
		HasDeepSeek:                 cfg.DeepSeekAPIKey != "",
		HasAnthropic:                cfg.AnthropicAPIKey != "",
		HasMistral:                  cfg.MistralAPIKey != "",
		HasGemini:                   cfg.GeminiAPIKey != "",
		HasOllama:                   cfg.OllamaBaseURL != "",
		HeadroomURL:                 cfg.HeadroomURL,
		MemPalaceURL:                cfg.MemPalaceURL,
		WebhookWorkers:              cfg.WebhookWorkers,
		WebhookQueueSize:            cfg.WebhookQueueSize,
	}
}

var _ tools.ConfigStatusReader = appToolRunner{}
var _ tools.SkillLister = appToolRunner{}
