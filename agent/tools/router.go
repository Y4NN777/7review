package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/Y4NN777/7review/agent/review"
)

// SCM enriches webhook events with merge/pull request API context.
type SCM interface {
	Enrich(context.Context, review.Request) (*review.SCMContext, error)
}

// Publisher writes review reports back to the source-control platform.
type Publisher interface {
	PublishDraft(context.Context, *review.SCMContext, string) error
	PublishFinal(context.Context, *review.SCMContext, string) error
}

type ProviderRouter struct {
	SCM        map[string]SCM
	Publishers map[string]Publisher
}

func (r ProviderRouter) Enrich(ctx context.Context, req review.Request) (*review.SCMContext, error) {
	tool, ok := r.SCM[strings.ToLower(req.Provider)]
	if !ok || tool == nil {
		return nil, fmt.Errorf("tools: no SCM configured for provider %q", req.Provider)
	}
	return tool.Enrich(ctx, req)
}

func (r ProviderRouter) PublishDraft(ctx context.Context, source *review.SCMContext, report string) error {
	tool, ok := r.Publishers[strings.ToLower(source.Provider)]
	if !ok || tool == nil {
		return nil
	}
	return tool.PublishDraft(ctx, source, report)
}

func (r ProviderRouter) PublishFinal(ctx context.Context, source *review.SCMContext, report string) error {
	tool, ok := r.Publishers[strings.ToLower(source.Provider)]
	if !ok || tool == nil {
		return nil
	}
	return tool.PublishFinal(ctx, source, report)
}

// NoopSCM is a safe development default that uses request fields only.
type NoopSCM struct{}

func (NoopSCM) Enrich(_ context.Context, req review.Request) (*review.SCMContext, error) {
	files := make([]review.ChangedFile, 0, len(req.ChangedPaths))
	for _, path := range req.ChangedPaths {
		files = append(files, review.ChangedFile{NewPath: path, Status: "modified"})
	}
	return &review.SCMContext{
		Provider:    req.Provider,
		ProjectID:   req.ProjectID,
		Repository:  req.Repository,
		ChangeID:    req.ChangeID,
		MRIID:       req.MRIID,
		Title:       req.Title,
		Description: req.Description,
		Author:      req.Author,
		WebURL:      req.WebURL,
		Labels:      req.Labels,
		DiffRefs: review.DiffRefs{
			BaseSHA: req.TargetSHA,
			HeadSHA: req.SourceSHA,
		},
		Files: files,
	}, nil
}

// NoopPublisher accepts reports without publishing them externally.
type NoopPublisher struct{}

func (NoopPublisher) PublishDraft(context.Context, *review.SCMContext, string) error {
	return nil
}

func (NoopPublisher) PublishFinal(context.Context, *review.SCMContext, string) error {
	return nil
}
