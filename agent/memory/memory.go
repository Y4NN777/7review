package memory

import (
	"context"

	"github.com/Y4NN777/7review/agent/review"
)

// Recall contains memory retrieved for a review run.
type Recall struct {
	Conventions []string
	Decisions   []string
	History     []string
}

// UpdateProposal is produced after review and validated before persistence.
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

// Store handles both memory recall and validated memory writes.
type Store interface {
	Recall(context.Context, review.Request) (Recall, error)
	ProposeUpdate(context.Context, *review.Context) (UpdateProposal, error)
	Write(context.Context, UpdateProposal) error
}

// NoopStore is a safe default that prevents accidental memory pollution.
type NoopStore struct{}

func (NoopStore) Recall(context.Context, review.Request) (Recall, error) {
	return Recall{}, nil
}

func (NoopStore) ProposeUpdate(context.Context, *review.Context) (UpdateProposal, error) {
	return UpdateProposal{}, nil
}

func (NoopStore) Write(context.Context, UpdateProposal) error {
	return nil
}
