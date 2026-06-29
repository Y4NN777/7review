package review

// Section contains project guidance relevant to a review.
type Section struct {
	Path    string
	Title   string
	Content string
	Kind    Kind
}

// EvidenceItem explains why one repository section was selected for review.
// It is operator-facing metadata; model-facing content stays in Section.
type EvidenceItem struct {
	Source            string   `json:"source"`
	HeadingOrKey      string   `json:"heading_or_key"`
	Kind              Kind     `json:"kind"`
	Authority         string   `json:"authority"`
	AuthorityLevel    string   `json:"authority_level,omitempty"`
	CanJustifyFinding bool     `json:"can_justify_finding,omitempty"`
	SupportsOnly      bool     `json:"supports_only,omitempty"`
	MatchedSignals    []string `json:"matched_signals,omitempty"`
	SelectionReason   string   `json:"selection_reason"`
	Score             int      `json:"score"`
	ContentBytes      int      `json:"content_bytes"`
}

// SkillActivation records why one review skill is active for a run.
type SkillActivation struct {
	Name           string   `json:"name"`
	Path           string   `json:"path,omitempty"`
	Category       string   `json:"category"`
	RiskTier       string   `json:"risk_tier,omitempty"`
	ReviewDomain   string   `json:"review_domain,omitempty"`
	AllowedTools   []string `json:"allowed_tools,omitempty"`
	RequiredChecks []string `json:"required_checks,omitempty"`
	Required       bool     `json:"required"`
	Reason         string   `json:"reason"`
}

// SkillCoverage is the model's auditable acknowledgement of an active skill.
type SkillCoverage struct {
	Name     string   `json:"name"`
	Status   string   `json:"status,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
	Tools    []string `json:"tools,omitempty"`
	Checks   []string `json:"checks,omitempty"`
	Notes    string   `json:"notes,omitempty"`
}

// ToolRequest is a model-proposed read-only tool call.
type ToolRequest struct {
	Name   string         `json:"name"`
	Input  map[string]any `json:"input,omitempty"`
	Reason string         `json:"reason,omitempty"`
	Round  int            `json:"round,omitempty"`
}

// ToolObservation records a governed read-only tool call result.
type ToolObservation struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Result  string `json:"result,omitempty"`
	Surface string `json:"surface,omitempty"`
	Round   int    `json:"round,omitempty"`
}

// Kind classifies rich review context so selection remains generic.
type Kind string

const (
	KindRules        Kind = "rules"
	KindPlanning     Kind = "planning"
	KindContract     Kind = "contract"
	KindArchitecture Kind = "architecture"
	KindAPI          Kind = "api"
	KindSecurity     Kind = "security"
	KindDesign       Kind = "design"
	KindDelivery     Kind = "delivery"
)

// DiffRefs identifies the Git refs used to compute a merge or pull request diff.
type DiffRefs struct {
	BaseSHA  string
	HeadSHA  string
	StartSHA string
}

// SCMContext is SCM-enriched review context normalized across providers.
type SCMContext struct {
	Provider    string
	ProjectID   string
	Repository  string
	ChangeID    string
	MRIID       int
	Title       string
	Description string
	Author      string
	WebURL      string
	Labels      []string
	DiffRefs    DiffRefs
	Commits     []Commit
	Files       []ChangedFile
	Discussions []Discussion
	Checks      []CheckRun
	Approvals   []Approval
}

type Commit struct {
	SHA     string
	Title   string
	Message string
	Author  string
}

type ChangedFile struct {
	OldPath   string
	NewPath   string
	Patch     string
	Status    string
	Additions int
	Deletions int
}

type Discussion struct {
	ID     string
	Author string
	Body   string
	URL    string
}

type CheckRun struct {
	Name       string
	Status     string
	Conclusion string
	URL        string
}

type Approval struct {
	Reviewer string
	State    string
}

// StructuredDiff is the normalized representation of a merge request diff.
type StructuredDiff struct {
	Files []FileDiff
}

// FileDiff describes one changed file and its estimated review complexity.
type FileDiff struct {
	Path       string
	Patch      string
	TokenCount int
}

type UpdateProposal struct {
	Conventions []string
	Decisions   []string
	Vectors     []Vector
}

type Vector struct {
	ID        string
	Text      string
	Embedding []float64
}
