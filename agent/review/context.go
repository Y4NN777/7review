package review

import (
	"sync"

	diffanalyzer "github.com/Y4NN777/7review/agent/subagents/diff_analyzer"
	"github.com/Y4NN777/7review/agent/tools"
)

// ReviewContext is assembled once per MR and passed through every pipeline step.
// Each step reads what it needs and writes its output back here.
// It is the single source of truth for in-flight review state.
//
// Steps are sequential by default. Step 5 (Review Agent) may populate
// Findings via multiple parallel calls — all writes go through AddFindings().
type Context struct {
	mu sync.Mutex

	Request Request

	// ── Inputs (populated by Step 1) ─────────────────────────────────────
	ProjectID string
	MRIID     int
	MRTitle   string
	MRAuthor  string
	WebURL    string
	DiffRefs  tools.DiffRefs

	// ── Structured diff (populated by Step 2) ────────────────────────────
	Diff *diffanalyzer.StructuredDiff

	// ── Retrieved context (populated by Step 3) ──────────────────────────
	Conventions string // formatted content of conventions.json
	Philosophy  string // content of philosophy.md

	// ── Contract sections (populated by Step 4) ──────────────────────────
	// Keyed by section identifier (e.g. "architecture/controller_rules.md").
	ContractSections []tools.ContractSection

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

	// ── Compaction output (populated by Step 7) ──────────────────────────
	NewConventions      []string
	PhilosophyAdditions []string

	// ── Execution metadata ────────────────────────────────────────────────
	// Which provider+model handled each step — written to the report footer.
	StepProviders map[string]string // e.g. {"step5": "anthropic/claude-sonnet-4-6"}
}

// NewReviewContext initialises a context for one MR review run.
func NewContext(req Request) *Context {
	return &Context{
		Request:       req,
		ProjectID:     req.ProjectID,
		MRIID:         req.MRIID,
		StepProviders: make(map[string]string),
	}
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
}

// ChangedPaths returns the list of file paths in the structured diff.
// Convenience method used by Steps 3 and 4.
func (rc *Context) ChangedPaths() []string {
	if rc.Diff == nil {
		return nil
	}
	paths := make([]string, len(rc.Diff.Files))
	for i, f := range rc.Diff.Files {
		paths[i] = f.Path
	}
	return paths
}
