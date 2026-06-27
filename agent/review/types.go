package review

// Section contains project guidance relevant to a review.
type Section struct {
	Path    string
	Title   string
	Content string
	Kind    Kind
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
