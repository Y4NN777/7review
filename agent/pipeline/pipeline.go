package pipeline

import (
	"context"
	"fmt"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/orchestrator"
	"github.com/Y4NN777/7review/agent/review"
	"github.com/Y4NN777/7review/agent/skills"
)

// Pipeline coordinates the review workflow for one merge request.
type Pipeline struct {
	Config       *config.Config
	SkillLoader  *skills.Loader
	Orchestrator *orchestrator.Orchestrator
}

// Run executes the automated review pipeline.
func (p *Pipeline) Run(ctx context.Context, projectID string, mrIID int) error {
	if p == nil || p.Orchestrator == nil {
		return fmt.Errorf("pipeline: orchestrator is not configured")
	}
	_ = ctx
	_ = review.NewContext(projectID, mrIID)
	return nil
}

// RunPostHIL continues the pipeline after human approval.
func (p *Pipeline) RunPostHIL(ctx context.Context, projectID string, mrIID int, approvedReport string) error {
	if p == nil {
		return fmt.Errorf("pipeline: not configured")
	}
	_ = ctx
	_ = projectID
	_ = mrIID
	_ = approvedReport
	return nil
}
