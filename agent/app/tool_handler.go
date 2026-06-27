package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/tools"
)

const toolExecuteMaxBodyBytes int64 = 64 * 1024

type appToolRunner struct {
	server *Server
}

func (r appToolRunner) ListRuns(ctx context.Context) (any, error) {
	runs, err := r.server.pipeline.Jobs.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]runDTO, 0, len(runs))
	for _, run := range runs {
		out = append(out, toRunDTO(run, false))
	}
	return out, nil
}

func (r appToolRunner) GetRun(ctx context.Context, id string) (any, error) {
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return toRunDTO(*run, true), nil
}

func (r appToolRunner) ApproveRun(ctx context.Context, run string, report string) error {
	return r.server.pipeline.ApproveRun(ctx, run, report)
}

func (r appToolRunner) PublishFinal(ctx context.Context, run string, report string) error {
	return r.server.pipeline.PublishFinal(ctx, run, report)
}

func (r appToolRunner) CheckReady(ctx context.Context) (any, error) {
	return r.server.readiness(ctx), nil
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

func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s == nil || s.pipeline == nil || s.pipeline.Jobs == nil {
		http.Error(w, "pipeline is not configured", http.StatusServiceUnavailable)
		return
	}
	body, err := readBoundedBody(r.Body, toolExecuteMaxBodyBytes)
	if err != nil {
		http.Error(w, "tool request too large", http.StatusRequestEntityTooLarge)
		return
	}
	var req tools.ExecuteRequest
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.UseNumber()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid tool request", http.StatusBadRequest)
		return
	}
	resp, err := (tools.ToolExecutor{
		Runs:    appToolRunner{server: s},
		Actions: appToolRunner{server: s},
		Ready:   appToolRunner{server: s},
		Config:  appToolRunner{server: s},
		Skills:  appToolRunner{server: s},
	}).Execute(r.Context(), req)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not configured") {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) readiness(ctx context.Context) readinessStatus {
	status := readinessStatus{
		Ready:        true,
		Dependencies: make(map[string]string),
	}
	if s == nil || s.pipeline == nil {
		status.markDown("pipeline", "pipeline is not configured")
	} else {
		status.Dependencies["pipeline"] = "ok"
	}
	if s == nil || s.pipeline == nil || s.pipeline.Orchestrator == nil {
		status.markDown("orchestrator", "orchestrator is not configured")
	} else {
		status.Dependencies["orchestrator"] = "ok"
	}
	if s == nil || s.work == nil {
		status.markDown("queue", "worker queue is not configured")
	} else {
		status.Queue = s.queueStatus()
		status.Dependencies["queue"] = fmt.Sprintf("ok depth=%d capacity=%d", status.Queue.Depth, status.Queue.Capacity)
	}
	if s == nil || s.pipeline == nil || s.pipeline.Jobs == nil {
		status.markDown("run_store", "run store is not configured")
	} else if _, err := s.pipeline.Jobs.List(ctx); err != nil {
		status.markDown("run_store", err.Error())
	} else {
		status.Dependencies["run_store"] = "ok"
	}
	if s == nil || s.pipeline == nil || s.pipeline.ContextReducer == nil {
		status.markDown("headroom", "headroom reducer is not configured")
	} else if err := s.pipeline.ContextReducer.Check(ctx); err != nil {
		status.Ready = false
		status.Dependencies["headroom"] = err.Error()
	} else {
		status.Dependencies["headroom"] = "ok"
	}
	if s == nil || s.pipeline == nil || s.pipeline.Memory == nil {
		status.markDown("mempalace", "mempalace store is not configured")
	} else if err := s.pipeline.Memory.Check(ctx); err != nil {
		status.markDown("mempalace", err.Error())
	} else {
		status.Dependencies["mempalace"] = "ok"
	}
	return status
}

type configStatusDTO struct {
	ListenAddr       string `json:"listen_addr"`
	CorpusRoot       string `json:"corpus_root"`
	MemoryDir        string `json:"memory_dir"`
	HILChannel       string `json:"hil_channel"`
	Provider         string `json:"provider"`
	ReviewModel      string `json:"review_model"`
	SmallModel       string `json:"small_model"`
	Orchestrator     string `json:"orchestrator_config,omitempty"`
	HasGitLab        bool   `json:"has_gitlab"`
	HasGitHub        bool   `json:"has_github"`
	HasOpenAI        bool   `json:"has_openai"`
	HasOpenRouter    bool   `json:"has_openrouter"`
	HasDeepSeek      bool   `json:"has_deepseek"`
	HasAnthropic     bool   `json:"has_anthropic"`
	HasMistral       bool   `json:"has_mistral"`
	HasGemini        bool   `json:"has_gemini"`
	HasOllama        bool   `json:"has_ollama"`
	HeadroomURL      string `json:"headroom_url"`
	MemPalaceURL     string `json:"mempalace_url"`
	WebhookWorkers   int    `json:"webhook_workers"`
	WebhookQueueSize int    `json:"webhook_queue_size"`
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

func configStatus(cfg *config.Config) configStatusDTO {
	if cfg == nil {
		return configStatusDTO{}
	}
	return configStatusDTO{
		ListenAddr:       cfg.ListenAddr,
		CorpusRoot:       cfg.CorpusRoot,
		MemoryDir:        cfg.MemoryDir,
		HILChannel:       cfg.HILChannel,
		Provider:         cfg.Provider,
		ReviewModel:      cfg.ReviewModel,
		SmallModel:       cfg.SmallModel,
		Orchestrator:     cfg.OrchestratorConfigPath,
		HasGitLab:        cfg.GitLabURL != "" && cfg.GitLabToken != "" && cfg.WebhookSecret != "",
		HasGitHub:        cfg.GitHubAPIURL != "" && cfg.GitHubToken != "" && cfg.GitHubWebhookSecret != "",
		HasOpenAI:        cfg.OpenAIAPIKey != "",
		HasOpenRouter:    cfg.OpenRouterAPIKey != "",
		HasDeepSeek:      cfg.DeepSeekAPIKey != "",
		HasAnthropic:     cfg.AnthropicAPIKey != "",
		HasMistral:       cfg.MistralAPIKey != "",
		HasGemini:        cfg.GeminiAPIKey != "",
		HasOllama:        cfg.OllamaBaseURL != "",
		HeadroomURL:      cfg.HeadroomURL,
		MemPalaceURL:     cfg.MemPalaceURL,
		WebhookWorkers:   cfg.WebhookWorkers,
		WebhookQueueSize: cfg.WebhookQueueSize,
	}
}

var _ tools.RunReader = appToolRunner{}
var _ tools.RunActions = appToolRunner{}
var _ tools.ReadyChecker = appToolRunner{}
var _ tools.ConfigStatusReader = appToolRunner{}
var _ tools.SkillLister = appToolRunner{}
