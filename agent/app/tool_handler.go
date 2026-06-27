package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/orchestrator"
	"github.com/Y4NN777/7review/agent/pipeline"
	"github.com/Y4NN777/7review/agent/review"
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

func (r appToolRunner) SuppressFinding(ctx context.Context, run string, findingID string, reason string) error {
	return r.server.pipeline.SuppressFinding(ctx, run, findingID, reason)
}

func (r appToolRunner) ReviseDraft(ctx context.Context, run string, request string) error {
	return r.server.pipeline.ReviseDraft(ctx, run, request)
}

func (r appToolRunner) RerunReview(ctx context.Context, run string, reason string) error {
	return r.server.pipeline.RerunReview(ctx, run, reason)
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

func (r appToolRunner) SelectedContext(ctx context.Context, id string) (any, error) {
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	source := sourceForRun(run)
	return selectedContextStatusDTO{
		Run:            run.ID,
		CorpusSections: sectionDTOs(source.CorpusSections),
		SkillSections:  sectionDTOs(source.SkillSections),
		Memory:         source.Memory,
		Warnings:       append([]string(nil), source.Run.Warnings...),
	}, nil
}

func (r appToolRunner) DiffSummary(ctx context.Context, id string) (any, error) {
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	source := sourceForRun(run)
	diff := source.Diff
	if diff == nil && run.Context != nil {
		diff = run.Context.Diff
	}
	out := diffSummaryDTO{Run: run.ID}
	for _, changed := range source.ChangedFiles {
		out.Additions += changed.Additions
		out.Deletions += changed.Deletions
		out.ChangedFiles = append(out.ChangedFiles, changedFileDTO{
			Path:      changed.NewPath,
			OldPath:   changed.OldPath,
			Status:    changed.Status,
			Additions: changed.Additions,
			Deletions: changed.Deletions,
			HasPatch:  strings.TrimSpace(changed.Patch) != "",
		})
	}
	if diff != nil {
		for _, file := range diff.Files {
			out.TotalTokens += file.TokenCount
			out.Files = append(out.Files, fileDiffDTO{
				Path:       file.Path,
				TokenCount: file.TokenCount,
				PatchLines: countLines(file.Patch),
			})
		}
	}
	out.FileCount = len(out.Files)
	if out.FileCount == 0 {
		out.FileCount = len(out.ChangedFiles)
	}
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	sort.Slice(out.ChangedFiles, func(i, j int) bool { return out.ChangedFiles[i].Path < out.ChangedFiles[j].Path })
	return out, nil
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

func (r appToolRunner) PublishStatus(ctx context.Context, id string) (any, error) {
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	source := sourceForRun(run)
	return publishStatusDTO{
		Run:             run.ID,
		Status:          run.Status,
		WebURL:          run.WebURL,
		HILApproved:     run.HILApproved,
		HasDraftReport:  strings.TrimSpace(run.DraftReport) != "",
		HasFinalReport:  strings.TrimSpace(run.FinalReport) != "",
		DraftBytes:      len(run.DraftReport),
		FinalBytes:      len(run.FinalReport),
		Error:           run.Error,
		Provider:        sourceProvider(source, run),
		ProjectID:       sourceProjectID(source, run),
		ChangeID:        sourceChangeID(source, run),
		SCMDiscussions:  len(sourceSCM(source).Discussions),
		SCMChecks:       len(sourceSCM(source).Checks),
		SCMApprovals:    len(sourceSCM(source).Approvals),
		UpdatedAtUnixMS: run.UpdatedAt.UnixMilli(),
	}, nil
}

func (r appToolRunner) MemoryProposal(ctx context.Context, id string) (any, error) {
	if r.server == nil || r.server.pipeline == nil || r.server.pipeline.Memory == nil {
		return nil, fmt.Errorf("memory store is not configured")
	}
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	rc := contextForRunPreview(run)
	proposal, err := r.server.pipeline.Memory.ProposeUpdate(ctx, rc)
	if err != nil {
		return nil, err
	}
	return memoryProposalStatusDTO{
		Run:        run.ID,
		Approved:   rc.HILApproved,
		Proposal:   proposal,
		FinalBytes: len(rc.FinalReport),
	}, nil
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
		Runs:        appToolRunner{server: s},
		Actions:     appToolRunner{server: s},
		Ready:       appToolRunner{server: s},
		Config:      appToolRunner{server: s},
		Skills:      appToolRunner{server: s},
		Observatory: appToolRunner{server: s},
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

type selectedContextStatusDTO struct {
	Run            string              `json:"run"`
	CorpusSections []sectionStatusDTO  `json:"corpus_sections"`
	SkillSections  []sectionStatusDTO  `json:"skill_sections"`
	Memory         review.MemoryRecall `json:"memory"`
	Warnings       []string            `json:"warnings,omitempty"`
}

type sectionStatusDTO struct {
	Path         string      `json:"path"`
	Title        string      `json:"title"`
	Kind         review.Kind `json:"kind"`
	ContentBytes int         `json:"content_bytes"`
	ContentLines int         `json:"content_lines"`
}

type diffSummaryDTO struct {
	Run          string           `json:"run"`
	FileCount    int              `json:"file_count"`
	TotalTokens  int              `json:"total_tokens"`
	Additions    int              `json:"additions"`
	Deletions    int              `json:"deletions"`
	Files        []fileDiffDTO    `json:"files"`
	ChangedFiles []changedFileDTO `json:"changed_files"`
}

type fileDiffDTO struct {
	Path       string `json:"path"`
	TokenCount int    `json:"token_count"`
	PatchLines int    `json:"patch_lines"`
}

type changedFileDTO struct {
	Path      string `json:"path"`
	OldPath   string `json:"old_path,omitempty"`
	Status    string `json:"status,omitempty"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	HasPatch  bool   `json:"has_patch"`
}

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

type publishStatusDTO struct {
	Run             string             `json:"run"`
	Status          pipeline.RunStatus `json:"status"`
	WebURL          string             `json:"web_url,omitempty"`
	HILApproved     bool               `json:"hil_approved"`
	HasDraftReport  bool               `json:"has_draft_report"`
	HasFinalReport  bool               `json:"has_final_report"`
	DraftBytes      int                `json:"draft_bytes"`
	FinalBytes      int                `json:"final_bytes"`
	Error           string             `json:"error,omitempty"`
	Provider        string             `json:"provider,omitempty"`
	ProjectID       string             `json:"project_id,omitempty"`
	ChangeID        string             `json:"change_id,omitempty"`
	SCMDiscussions  int                `json:"scm_discussions"`
	SCMChecks       int                `json:"scm_checks"`
	SCMApprovals    int                `json:"scm_approvals"`
	UpdatedAtUnixMS int64              `json:"updated_at_unix_ms"`
}

type memoryProposalStatusDTO struct {
	Run        string                `json:"run"`
	Approved   bool                  `json:"approved"`
	Proposal   review.UpdateProposal `json:"proposal"`
	FinalBytes int                   `json:"final_bytes"`
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
var _ tools.Observatory = appToolRunner{}

func sourceForRun(run *pipeline.Run) review.Source {
	if run == nil {
		return review.Source{}
	}
	if run.Source != nil {
		return *run.Source
	}
	if run.Context != nil {
		return run.Context.Source
	}
	return review.Source{Request: run.Request}
}

func sourceSCM(source review.Source) *review.SCMContext {
	if source.SCM != nil {
		return source.SCM
	}
	return &review.SCMContext{}
}

func sectionDTOs(sections []review.Section) []sectionStatusDTO {
	out := make([]sectionStatusDTO, 0, len(sections))
	for _, section := range sections {
		out = append(out, sectionStatusDTO{
			Path:         section.Path,
			Title:        section.Title,
			Kind:         section.Kind,
			ContentBytes: len(section.Content),
			ContentLines: countLines(section.Content),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].Path < out[j].Path
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func countLines(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
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

func sourceProvider(source review.Source, run *pipeline.Run) string {
	if source.SCM != nil && source.SCM.Provider != "" {
		return source.SCM.Provider
	}
	if run != nil {
		return run.Request.Provider
	}
	return ""
}

func sourceProjectID(source review.Source, run *pipeline.Run) string {
	if source.SCM != nil && source.SCM.ProjectID != "" {
		return source.SCM.ProjectID
	}
	if run != nil {
		return run.Request.ProjectID
	}
	return ""
}

func sourceChangeID(source review.Source, run *pipeline.Run) string {
	if source.SCM != nil && source.SCM.ChangeID != "" {
		return source.SCM.ChangeID
	}
	if run != nil {
		return run.Request.ChangeID
	}
	return ""
}

func contextForRunPreview(run *pipeline.Run) *review.Context {
	if run == nil {
		return review.NewContext(review.Request{})
	}
	if run.Context != nil {
		return run.Context
	}
	rc := review.NewContext(run.Request)
	if run.Source != nil {
		rc.Source = *run.Source
	}
	rc.Request = run.Request
	rc.DraftReport = run.DraftReport
	rc.FinalReport = run.FinalReport
	rc.HILApproved = run.HILApproved
	rc.Findings = append([]review.Finding(nil), run.Findings...)
	rc.WebURL = run.WebURL
	return rc
}
