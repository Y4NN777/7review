package jobs

import (
	"context"
	"strconv"
	"sync"

	"github.com/Y4NN777/7review/agent/review"
)

// Status describes where a review run is in the workflow.
type Status string

const (
	StatusQueued      Status = "queued"
	StatusRunning     Status = "running"
	StatusDrafted     Status = "drafted"
	StatusAwaitingHIL Status = "awaiting_hil"
	StatusApproved    Status = "approved"
	StatusPublished   Status = "published"
	StatusFailed      Status = "failed"
)

// Run stores idempotency and lifecycle metadata for a review request.
type Run struct {
	ID      string
	Request review.Request
	Status  Status
	Error   string
}

// Store tracks review runs.
type Store interface {
	Start(context.Context, review.Request) (*Run, error)
	Update(context.Context, string, Status, error) error
}

// MemoryStore is a process-local implementation suitable for development.
type MemoryStore struct {
	mu   sync.Mutex
	runs map[string]*Run
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{runs: make(map[string]*Run)}
}

func (s *MemoryStore) Start(_ context.Context, req review.Request) (*Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := req.ProjectID + "!" + strconv.Itoa(req.MRIID)
	run := &Run{ID: id, Request: req, Status: StatusRunning}
	s.runs[id] = run
	return run, nil
}

func (s *MemoryStore) Update(_ context.Context, id string, status Status, err error) error {
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
	}
	return nil
}
