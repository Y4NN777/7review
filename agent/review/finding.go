package review

// Severity is the normalized impact level for a review finding.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Location points to a changed file and optional line number.
type Location struct {
	Path string
	Line int
}

// EvidenceCitation ties a finding to an exact selected repository evidence item.
type EvidenceCitation struct {
	Source       string `json:"source"`
	HeadingOrKey string `json:"heading_or_key,omitempty"`
	Rule         string `json:"rule"`
	Violation    string `json:"violation"`
}

// Finding is the structured form expected from the review agent.
type Finding struct {
	ID                string
	Severity          Severity
	Title             string
	Description       string
	Suggestion        string
	Location          Location
	Confidence        float64
	FindingType       string             `json:"finding_type,omitempty"`
	Strength          string             `json:"strength,omitempty"`
	EvidenceAuthority string             `json:"evidence_authority,omitempty"`
	Citations         []EvidenceCitation `json:"citations,omitempty"`
	ValidationStatus  string             `json:"validation_status,omitempty"`
	ValidationReason  string             `json:"validation_reason,omitempty"`
}

// InlineComment records provider-neutral inline publishing state for a finding.
type InlineComment struct {
	FindingID  string `json:"finding_id"`
	Path       string `json:"path"`
	OldPath    string `json:"old_path,omitempty"`
	NewPath    string `json:"new_path,omitempty"`
	Line       int    `json:"line"`
	Side       string `json:"side,omitempty"`
	Body       string `json:"body,omitempty"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	ProviderID string `json:"provider_id,omitempty"`
	URL        string `json:"url,omitempty"`
}

// InlinePosition describes provider metadata needed to place an inline comment.
type InlinePosition struct {
	Path     string   `json:"path"`
	OldPath  string   `json:"old_path,omitempty"`
	NewPath  string   `json:"new_path,omitempty"`
	Line     int      `json:"line"`
	Side     string   `json:"side"`
	Provider string   `json:"provider,omitempty"`
	DiffRefs DiffRefs `json:"diff_refs"`
	Valid    bool     `json:"valid"`
	Reason   string   `json:"reason,omitempty"`
}
