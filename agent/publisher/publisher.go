package publisher

import (
	"context"

	"github.com/Y4NN777/7review/agent/review"
)

// Publisher writes draft and final reports to the external review system.
type Publisher interface {
	PublishDraft(context.Context, *review.Context) error
	PublishFinal(context.Context, *review.Context) error
}

// NoopPublisher keeps the pipeline buildable until a GitLab publisher is wired.
type NoopPublisher struct{}

func (NoopPublisher) PublishDraft(context.Context, *review.Context) error {
	return nil
}

func (NoopPublisher) PublishFinal(context.Context, *review.Context) error {
	return nil
}
