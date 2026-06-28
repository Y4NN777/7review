package review

import (
	"sync"
	"time"
)

// Source is the single source of truth for one review run.
// The pipeline progressively enriches it from webhook input through SCM data,
// changed files, selected corpus, skills, memory, model findings, report, and
// run metadata.
type Source struct {
	Request Request
	SCM     *SCMContext

	ChangedFiles   []ChangedFile
	Diff           *StructuredDiff
	CorpusSections []Section
	Evidence       []EvidenceItem
	SkillSections  []Section
	Memory         MemoryRecall

	Model    ModelReview
	Findings []Finding
	Report   Report
	Run      RunMetadata
}

type ModelReview struct {
	RawResponses       []string `json:"raw_responses,omitempty"`
	ParseStatus        string   `json:"parse_status,omitempty"`
	ParseWarning       string   `json:"parse_warning,omitempty"`
	ParsedFindings     int      `json:"parsed_findings"`
	AcceptedFindings   int      `json:"accepted_findings"`
	RejectedFindings   int      `json:"rejected_findings"`
	ProviderTrace      string   `json:"provider_trace,omitempty"`
	RawResponseBytes   int      `json:"raw_response_bytes"`
	RawResponseExcerpt string   `json:"raw_response_excerpt,omitempty"`
}

type MemoryRecall struct {
	Conventions []string
	Decisions   []string
	History     []string
}

type Report struct {
	Draft string
	Final string
}

type RunMetadata struct {
	ID             string
	StartedAt      time.Time
	StepProviders  map[string]string
	AvailableTools []string
	Warnings       []string
}

// Context is kept as a compatibility alias for older package interfaces.
type Context struct {
	mu sync.Mutex

	Source
	Request Request

	// ── Inputs (populated by Step 1) ─────────────────────────────────────
	ProjectID string
	MRIID     int
	MRTitle   string
	MRAuthor  string
	WebURL    string
	DiffRefs  DiffRefs

	// ── Structured diff (populated by Step 2) ────────────────────────────
	Diff *StructuredDiff

	// ── Retrieved context (populated by Step 3) ──────────────────────────
	Conventions string // formatted content of conventions.json
	Philosophy  string // content of philosophy.md

	// ── Contract sections (populated by Step 4) ──────────────────────────
	// Keyed by section identifier (e.g. "architecture/controller_rules.md").
	ContractSections []Section
	SkillSections    []Section

	// ── Review findings (populated by Step 5, thread-safe) ───────────────
	// Each parallel batch appends its raw findings string here.
	rawFindings []string
	Findings    []Finding

	// ── Report (populated by Step 6) ─────────────────────────────────────
	DraftReport string
	FinalReport string

	// ── HIL decision (populated by HIL gate) ─────────────────────────────
	HILApproved bool
	// HILEdits are finding IDs the human marked as false positives or added.
	HILRejectedIDs []string
	HILAddedNotes  []string

	// ── Memory update proposal (populated after approval) ────────────────
	NewConventions      []string
	PhilosophyAdditions []string

	// ── Execution metadata ────────────────────────────────────────────────
	// Which provider+model handled each step — written to the report footer.
	StepProviders  map[string]string // e.g. {"step5": "ollama/deepseek-coder-v2:16b"}
	AvailableTools []string
}

// NewReviewContext initialises a context for one MR review run.
func NewContext(req Request) *Context {
	rc := &Context{
		Request:       req,
		ProjectID:     req.ProjectID,
		MRIID:         req.MRIID,
		StepProviders: make(map[string]string),
	}
	rc.Source = Source{
		Request: req,
		Run: RunMetadata{
			StartedAt:     time.Now().UTC(),
			StepProviders: rc.StepProviders,
		},
	}
	return rc
}

// AddFindings appends raw findings from one parallel batch.
// Thread-safe — called concurrently by Step 5 goroutines.
func (rc *Context) AddFindings(findings string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.rawFindings = append(rc.rawFindings, findings)
}

// AllFindings returns the merged findings from all batches.
func (rc *Context) AllFindings() []string {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	out := make([]string, len(rc.rawFindings))
	copy(out, rc.rawFindings)
	return out
}

// RecordProvider logs which provider handled a given step.
func (rc *Context) RecordProvider(step, providerAndModel string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.StepProviders[step] = providerAndModel
	rc.Run.StepProviders = rc.StepProviders
}

func (rc *Context) AddWarning(warning string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.Run.Warnings = append(rc.Run.Warnings, warning)
}

// ChangedPaths returns the list of file paths in the structured diff.
// Convenience method used by Steps 3 and 4.
func (rc *Context) ChangedPaths() []string {
	diff := rc.Diff
	if diff == nil {
		diff = rc.Source.Diff
	}
	if diff == nil {
		return nil
	}
	paths := make([]string, len(diff.Files))
	for i, f := range diff.Files {
		paths[i] = f.Path
	}
	return paths
}
