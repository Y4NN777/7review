package operator

import (
	"encoding/json"
	"time"
)

type RunRow struct {
	ID               string    `json:"id"`
	Provider         string    `json:"provider"`
	ProjectID        string    `json:"project_id"`
	ChangeID         string    `json:"change_id"`
	Title            string    `json:"title"`
	Status           string    `json:"status"`
	Error            string    `json:"error"`
	WebURL           string    `json:"web_url"`
	UpdatedAt        time.Time `json:"updated_at"`
	EventCount       int       `json:"event_count"`
	Events           []any     `json:"events"`
	Findings         []any     `json:"findings"`
	ToolRequests     int       `json:"tool_requests"`
	ToolObservations int       `json:"tool_observations"`
	DraftReport      string    `json:"draft_report"`
	FinalReport      string    `json:"final_report"`
	HILApproved      bool      `json:"hil_approved"`
}

type RunEvent struct {
	At      time.Time         `json:"at"`
	Type    string            `json:"type"`
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Meta    map[string]string `json:"meta"`
}

type RunDetail struct {
	ID               string     `json:"id"`
	Provider         string     `json:"provider"`
	ProjectID        string     `json:"project_id"`
	ChangeID         string     `json:"change_id"`
	Status           string     `json:"status"`
	Title            string     `json:"title"`
	WebURL           string     `json:"web_url"`
	EventCount       int        `json:"event_count"`
	Events           []RunEvent `json:"events"`
	Findings         []any      `json:"findings"`
	ToolRequests     int        `json:"tool_requests"`
	ToolObservations int        `json:"tool_observations"`
	DraftReport      string     `json:"draft_report"`
	FinalReport      string     `json:"final_report"`
	HILApproved      bool       `json:"hil_approved"`
}

type ToolEnvelope struct {
	Name   string          `json:"name"`
	Result json.RawMessage `json:"result"`
}

type ProviderStatus struct {
	Mode               string              `json:"mode"`
	ActiveProvider     string              `json:"active_provider"`
	OrchestratorConfig string              `json:"orchestrator_config"`
	Providers          []ProviderStatusRow `json:"providers"`
	Roles              []RoleStatus        `json:"roles"`
}

type ProviderStatusRow struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	BaseURL    string `json:"base_url"`
	Reason     string `json:"reason"`
}

type RoleStatus struct {
	Role        string   `json:"role"`
	Primary     string   `json:"primary"`
	Fallbacks   []string `json:"fallbacks"`
	MaxTokens   int      `json:"max_tokens"`
	Parallel    bool     `json:"parallel"`
	MaxParallel int      `json:"max_parallel"`
}

type SkillStatus struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Loaded bool   `json:"loaded"`
}

type ConfigStatus struct {
	ListenAddr                  string `json:"listen_addr"`
	CorpusRoot                  string `json:"corpus_root"`
	MaxSupportingCorpusSections int    `json:"max_supporting_corpus_sections"`
	MemoryDir                   string `json:"memory_dir"`
	HILChannel                  string `json:"hil_channel"`
	Provider                    string `json:"provider"`
	ReviewModel                 string `json:"review_model"`
	SmallModel                  string `json:"small_model"`
	EmbeddingModel              string `json:"embedding_model"`
	Orchestrator                string `json:"orchestrator_config"`
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

type MemoryProposalStatus struct {
	Run        string         `json:"run"`
	Approved   bool           `json:"approved"`
	Proposal   UpdateProposal `json:"proposal"`
	FinalBytes int            `json:"final_bytes"`
}

type UpdateProposal struct {
	Conventions []string `json:"Conventions"`
	Decisions   []string `json:"Decisions"`
	Vectors     []Vector `json:"Vectors"`
}

type Vector struct {
	ID   string `json:"ID"`
	Text string `json:"Text"`
}

type SelectedContext struct {
	Run            string            `json:"run"`
	CorpusSections []ContextSection  `json:"corpus_sections"`
	SkillSections  []ContextSection  `json:"skill_sections"`
	Evidence       []ContextEvidence `json:"evidence_manifest"`
	Warnings       []string          `json:"warnings"`
}

type ContextSection struct {
	Path            string `json:"path"`
	Title           string `json:"title"`
	Kind            string `json:"kind"`
	ContentBytes    int    `json:"content_bytes"`
	ContentLines    int    `json:"content_lines"`
	SelectionReason string `json:"selection_reason"`
}

type ContextEvidence struct {
	Source          string   `json:"source"`
	HeadingOrKey    string   `json:"heading_or_key"`
	Kind            string   `json:"kind"`
	Authority       string   `json:"authority"`
	MatchedSignals  []string `json:"matched_signals"`
	SelectionReason string   `json:"selection_reason"`
	Score           int      `json:"score"`
	ContentBytes    int      `json:"content_bytes"`
}

type DiffSummary struct {
	Run          string        `json:"run"`
	FileCount    int           `json:"file_count"`
	TotalTokens  int           `json:"total_tokens"`
	Additions    int           `json:"additions"`
	Deletions    int           `json:"deletions"`
	Files        []FileDiff    `json:"files"`
	ChangedFiles []ChangedFile `json:"changed_files"`
}

type FileDiff struct {
	Path       string `json:"path"`
	TokenCount int    `json:"token_count"`
	PatchLines int    `json:"patch_lines"`
}

type ChangedFile struct {
	Path      string `json:"path"`
	OldPath   string `json:"old_path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	HasPatch  bool   `json:"has_patch"`
}
