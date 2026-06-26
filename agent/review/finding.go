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

// Finding is the structured form expected from the review agent.
type Finding struct {
	ID          string
	Severity    Severity
	Title       string
	Description string
	Suggestion  string
	Location    Location
	Confidence  float64
}
