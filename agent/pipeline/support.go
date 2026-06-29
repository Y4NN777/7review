package pipeline

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Y4NN777/7review/agent/review"
)

type RunStatus string

const (
	StatusRunning    RunStatus = "running"
	StatusDrafted    RunStatus = "drafted"
	StatusFinalizing RunStatus = "finalizing"
	StatusFinalized  RunStatus = "finalized"
	StatusFailed     RunStatus = "failed"
)

type Run struct {
	ID          string
	Request     review.Request
	Status      RunStatus
	Error       string
	Context     *review.Context
	Source      *review.Source
	DraftReport string
	FinalReport string
	HILApproved bool
	Findings    []review.Finding
	WebURL      string
	UpdatedAt   time.Time
	Events      []RunEvent
}

type RunEvent struct {
	At      time.Time         `json:"at"`
	Type    string            `json:"type"`
	Status  RunStatus         `json:"status,omitempty"`
	Message string            `json:"message,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

type RunStore interface {
	Start(context.Context, review.Request) (*Run, error)
	Update(context.Context, string, RunStatus, error) error
	SaveContext(context.Context, string, *review.Context) error
	AppendEvent(context.Context, string, RunEvent) error
	Get(context.Context, string) (*Run, error)
	List(context.Context) ([]Run, error)
}

type MemoryRunStore struct {
	mu   sync.Mutex
	runs map[string]*Run
}

func NewMemoryRunStore() *MemoryRunStore {
	return &MemoryRunStore{runs: make(map[string]*Run)}
}

func (s *MemoryRunStore) Start(_ context.Context, req review.Request) (*Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := requestRunID(req)
	run := &Run{ID: id, Request: req, Status: StatusRunning, UpdatedAt: time.Now().UTC()}
	appendRunEvent(run, "run_started", StatusRunning, "", map[string]string{
		"provider": req.Provider,
		"project":  req.ProjectID,
		"change":   firstNonEmpty(req.ChangeID, strconv.Itoa(req.MRIID)),
	})
	s.runs[id] = run
	return run, nil
}

func (s *MemoryRunStore) Update(_ context.Context, id string, status RunStatus, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.runs[id]
	if !ok {
		run = &Run{ID: id}
		s.runs[id] = run
	}
	run.Status = status
	if err != nil {
		run.Error = err.Error()
	} else {
		run.Error = ""
	}
	run.UpdatedAt = time.Now().UTC()
	appendRunEvent(run, "status_changed", status, run.Error, nil)
	return nil
}

func (s *MemoryRunStore) SaveContext(_ context.Context, id string, rc *review.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.runs[id]
	if !ok {
		run = &Run{ID: id}
		s.runs[id] = run
	}
	run.Context = rc
	if rc != nil {
		source := rc.Source
		run.Source = &source
		run.Request = rc.Request
		run.DraftReport = rc.DraftReport
		run.FinalReport = rc.FinalReport
		run.HILApproved = rc.HILApproved
		run.Findings = append([]review.Finding(nil), rc.Findings...)
		run.WebURL = rc.WebURL
	}
	run.UpdatedAt = time.Now().UTC()
	appendContextSavedEvent(run)
	return nil
}

func (s *MemoryRunStore) AppendEvent(_ context.Context, id string, event RunEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.runs[id]
	if !ok {
		return fmt.Errorf("run %q not found", id)
	}
	appendPreparedRunEvent(run, event)
	run.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *MemoryRunStore) Get(_ context.Context, id string) (*Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.runs[id]
	if !ok {
		return nil, fmt.Errorf("run %q not found", id)
	}
	copy := *run
	copy.Findings = append([]review.Finding(nil), run.Findings...)
	copy.Events = copyRunEvents(run.Events)
	return &copy, nil
}

func (s *MemoryRunStore) List(_ context.Context) ([]Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Run, 0, len(s.runs))
	for _, run := range s.runs {
		copy := *run
		copy.Findings = append([]review.Finding(nil), run.Findings...)
		copy.Events = copyRunEvents(run.Events)
		out = append(out, copy)
	}
	return out, nil
}

// FileRunStore persists run state as JSON files so drafts, HIL approvals, and
// final reports survive process and container restarts.
type FileRunStore struct {
	mu  sync.Mutex
	dir string
}

func NewFileRunStore(dir string) *FileRunStore {
	return &FileRunStore{dir: dir}
}

func (s *FileRunStore) Start(ctx context.Context, req review.Request) (*Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureDir(); err != nil {
		return nil, err
	}
	id := requestRunID(req)
	run := &Run{ID: id, Request: req, Status: StatusRunning, UpdatedAt: time.Now().UTC()}
	appendRunEvent(run, "run_started", StatusRunning, "", map[string]string{
		"provider": req.Provider,
		"project":  req.ProjectID,
		"change":   firstNonEmpty(req.ChangeID, strconv.Itoa(req.MRIID)),
	})
	if err := s.writeLocked(run); err != nil {
		return nil, err
	}
	return s.copyRun(run), nil
}

func (s *FileRunStore) Update(_ context.Context, id string, status RunStatus, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, readErr := s.readLocked(id)
	if readErr != nil {
		run = &Run{ID: id}
	}
	run.Status = status
	if err != nil {
		run.Error = err.Error()
	} else {
		run.Error = ""
	}
	run.UpdatedAt = time.Now().UTC()
	appendRunEvent(run, "status_changed", status, run.Error, nil)
	return s.writeLocked(run)
}

func (s *FileRunStore) SaveContext(_ context.Context, id string, rc *review.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, err := s.readLocked(id)
	if err != nil {
		run = &Run{ID: id}
	}
	if rc != nil {
		run.Context = nil
		run.Request = rc.Request
		run.DraftReport = rc.DraftReport
		run.FinalReport = rc.FinalReport
		run.HILApproved = rc.HILApproved
		run.Findings = append([]review.Finding(nil), rc.Findings...)
		run.WebURL = rc.WebURL
		run.Source = &rc.Source
	}
	run.UpdatedAt = time.Now().UTC()
	appendContextSavedEvent(run)
	return s.writeLocked(run)
}

func (s *FileRunStore) AppendEvent(_ context.Context, id string, event RunEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, err := s.readLocked(id)
	if err != nil {
		return err
	}
	appendPreparedRunEvent(run, event)
	run.UpdatedAt = time.Now().UTC()
	return s.writeLocked(run)
}

func (s *FileRunStore) Get(_ context.Context, id string) (*Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, err := s.readLocked(id)
	if err != nil {
		return nil, err
	}
	return s.copyRun(run), nil
}

func (s *FileRunStore) List(_ context.Context) ([]Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var out []Run
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		run, err := s.readFileLocked(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, *s.copyRun(run))
	}
	return out, nil
}

func (s *FileRunStore) ensureDir() error {
	if s == nil || s.dir == "" {
		return fmt.Errorf("run store: missing directory")
	}
	return os.MkdirAll(s.dir, 0o700)
}

func (s *FileRunStore) readLocked(id string) (*Run, error) {
	if err := s.ensureDir(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		data, err = os.ReadFile(s.legacyPath(id))
	}
	if err != nil {
		return nil, fmt.Errorf("run %q not found", id)
	}
	return decodeRun(id, data)
}

func (s *FileRunStore) readFileLocked(path string) (*Run, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeRun(filepath.Base(path), data)
}

func decodeRun(id string, data []byte) (*Run, error) {
	var run Run
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("run store: decode %s: %w", id, err)
	}
	return &run, nil
}

func (s *FileRunStore) writeLocked(run *Run) error {
	if run == nil {
		return nil
	}
	if err := s.ensureDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("run store: encode %s: %w", run.ID, err)
	}
	tmp := s.path(run.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("run store: write %s: %w", run.ID, err)
	}
	if err := os.Rename(tmp, s.path(run.ID)); err != nil {
		return fmt.Errorf("run store: commit %s: %w", run.ID, err)
	}
	return nil
}

func (s *FileRunStore) path(id string) string {
	return filepath.Join(s.dir, safeRunFilename(id)+".json")
}

func (s *FileRunStore) legacyPath(id string) string {
	return filepath.Join(s.dir, legacySafeRunFilename(id)+".json")
}

func (s *FileRunStore) copyRun(run *Run) *Run {
	if run == nil {
		return nil
	}
	copy := *run
	copy.Context = contextForPersistedRun(&copy)
	copy.Findings = append([]review.Finding(nil), run.Findings...)
	copy.Events = copyRunEvents(run.Events)
	return &copy
}

func contextForPersistedRun(run *Run) *review.Context {
	if run == nil {
		return nil
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

func safeRunFilename(id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

func legacySafeRunFilename(id string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "!", "!")
	return replacer.Replace(id)
}

func requestRunID(req review.Request) string {
	changeID := strings.TrimSpace(req.ChangeID)
	if changeID == "" && req.MRIID != 0 {
		changeID = strconv.Itoa(req.MRIID)
	}
	return req.ProjectID + "!" + changeID
}

func appendContextSavedEvent(run *Run) {
	if run == nil {
		return
	}
	appendRunEvent(run, "context_saved", run.Status, "", map[string]string{
		"draft_bytes": strconv.Itoa(len(run.DraftReport)),
		"final_bytes": strconv.Itoa(len(run.FinalReport)),
		"findings":    strconv.Itoa(len(run.Findings)),
		"hil":         strconv.FormatBool(run.HILApproved),
	})
}

func appendRunEvent(run *Run, eventType string, status RunStatus, message string, meta map[string]string) {
	if run == nil {
		return
	}
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		eventType = "event"
	}
	event := RunEvent{
		At:      time.Now().UTC(),
		Type:    eventType,
		Status:  status,
		Message: strings.TrimSpace(message),
		Meta:    cleanEventMeta(meta),
	}
	run.Events = append(run.Events, event)
}

func appendPreparedRunEvent(run *Run, event RunEvent) {
	if run == nil {
		return
	}
	event.Type = strings.TrimSpace(event.Type)
	if event.Type == "" {
		event.Type = "event"
	}
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	} else {
		event.At = event.At.UTC()
	}
	event.Message = strings.TrimSpace(event.Message)
	event.Meta = cleanEventMeta(event.Meta)
	run.Events = append(run.Events, event)
}

func cleanEventMeta(meta map[string]string) map[string]string {
	out := make(map[string]string)
	for key, value := range meta {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func copyRunEvents(events []RunEvent) []RunEvent {
	out := make([]RunEvent, 0, len(events))
	for _, event := range events {
		copy := event
		copy.Meta = cleanEventMeta(event.Meta)
		out = append(out, copy)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "0" {
			return value
		}
	}
	return ""
}

type Recall = review.MemoryRecall
type UpdateProposal = review.UpdateProposal
type Vector = review.Vector

type MemoryStore interface {
	Recall(context.Context, review.Request) (Recall, error)
	ProposeUpdate(context.Context, *review.Context) (UpdateProposal, error)
	Write(context.Context, UpdateProposal) error
	Check(context.Context) error
}

type ContextReducer interface {
	Reduce(context.Context, *review.Context) error
	Check(context.Context) error
}

type NoopMemoryStore struct{}

func (NoopMemoryStore) Recall(context.Context, review.Request) (Recall, error) {
	return Recall{}, nil
}

func (NoopMemoryStore) ProposeUpdate(context.Context, *review.Context) (UpdateProposal, error) {
	return UpdateProposal{}, nil
}

func (NoopMemoryStore) Write(context.Context, UpdateProposal) error {
	return nil
}

func (NoopMemoryStore) Check(context.Context) error {
	return nil
}

type NoopContextReducer struct{}

func (NoopContextReducer) Reduce(context.Context, *review.Context) error {
	return nil
}

func (NoopContextReducer) Check(context.Context) error {
	return nil
}

type PolicyDecision struct {
	SkippedPaths []string
	ReviewPaths  []string
}

type PolicyFilter interface {
	Apply(context.Context, *review.Context) (PolicyDecision, error)
}

type DefaultPolicyFilter struct{}

func (DefaultPolicyFilter) Apply(_ context.Context, rc *review.Context) (PolicyDecision, error) {
	var decision PolicyDecision
	for _, path := range rc.Request.ChangedPaths {
		if shouldSkip(path) {
			decision.SkippedPaths = append(decision.SkippedPaths, path)
			continue
		}
		decision.ReviewPaths = append(decision.ReviewPaths, path)
	}
	return decision, nil
}

func shouldSkip(path string) bool {
	clean := filepath.ToSlash(path)
	if strings.Contains(clean, "/vendor/") || strings.Contains(clean, "/node_modules/") {
		return true
	}
	switch filepath.Base(clean) {
	case "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "go.sum":
		return true
	}
	return strings.HasSuffix(clean, ".generated.go") || strings.HasSuffix(clean, ".min.js")
}

type ValidationReport struct {
	Accepted []review.Finding
	Rejected []RejectedFinding
}

type RejectedFinding struct {
	Finding review.Finding
	Reason  string
}

type FindingValidator interface {
	Validate(context.Context, *review.Context, []review.Finding) (ValidationReport, error)
}

type DefaultFindingValidator struct {
	MinConfidence float64
}

func (v DefaultFindingValidator) Validate(_ context.Context, rc *review.Context, findings []review.Finding) (ValidationReport, error) {
	minConfidence := v.MinConfidence
	if minConfidence == 0 {
		minConfidence = 0.45
	}

	changed := make(map[string]bool)
	for _, path := range rc.ChangedPaths() {
		changed[path] = true
	}
	for _, path := range rc.Request.ChangedPaths {
		changed[path] = true
	}
	changedLines := changedNewLinesByPath(rc)

	var report ValidationReport
	seen := make(map[string]bool)
	for _, finding := range findings {
		if finding.ID != "" && seen[finding.ID] {
			report.Rejected = append(report.Rejected, RejectedFinding{Finding: finding, Reason: "duplicate finding id"})
			continue
		}
		if finding.ID != "" {
			seen[finding.ID] = true
		}
		if !validSeverity(finding.Severity) {
			report.Rejected = append(report.Rejected, RejectedFinding{Finding: finding, Reason: fmt.Sprintf("invalid severity %q", finding.Severity)})
			continue
		}
		if finding.Confidence < minConfidence {
			report.Rejected = append(report.Rejected, RejectedFinding{Finding: finding, Reason: "confidence below threshold"})
			continue
		}
		if strings.TrimSpace(finding.Location.Path) == "" {
			report.Rejected = append(report.Rejected, RejectedFinding{Finding: finding, Reason: "missing changed-file location"})
			continue
		}
		if finding.Location.Path != "" && len(changed) > 0 && !changed[finding.Location.Path] {
			report.Rejected = append(report.Rejected, RejectedFinding{Finding: finding, Reason: "location is not in changed paths"})
			continue
		}
		if finding.Location.Line <= 0 {
			report.Rejected = append(report.Rejected, RejectedFinding{Finding: finding, Reason: "missing changed-line location"})
			continue
		}
		if len(changedLines) > 0 && !findingLineAddressable(rc, finding.Location.Path, finding.Location.Line, changedLines) {
			report.Rejected = append(report.Rejected, RejectedFinding{Finding: finding, Reason: "location line is not an added or changed line"})
			continue
		}
		report.Accepted = append(report.Accepted, finding)
	}
	return report, nil
}

func validSeverity(severity review.Severity) bool {
	switch severity {
	case review.SeverityInfo, review.SeverityLow, review.SeverityMedium, review.SeverityHigh, review.SeverityCritical:
		return true
	default:
		return false
	}
}
