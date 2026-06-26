package knowledge

// Section contains project guidance relevant to a review.
type Section struct {
	Path    string
	Title   string
	Content string
	Kind    Kind
}

// Kind classifies rich review context so selection remains generic across projects.
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
