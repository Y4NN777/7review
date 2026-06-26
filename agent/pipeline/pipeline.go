package pipeline

import (
	"context"
	"fmt"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/jobs"
	"github.com/Y4NN777/7review/agent/memory"
	"github.com/Y4NN777/7review/agent/orchestrator"
	"github.com/Y4NN777/7review/agent/policy"
	"github.com/Y4NN777/7review/agent/publisher"
	"github.com/Y4NN777/7review/agent/review"
	"github.com/Y4NN777/7review/agent/skills"
	"github.com/Y4NN777/7review/agent/validator"
)

// Pipeline coordinates the review workflow for one merge request.
type Pipeline struct {
	Config           *config.Config
	SkillLoader      *skills.Loader
	Orchestrator     *orchestrator.Orchestrator
	Jobs             jobs.Store
	Policy           policy.Filter
	FindingValidator validator.FindingValidator
	Publisher        publisher.Publisher
	Memory           memory.Store
}

// Run executes the automated review pipeline.
func (p *Pipeline) Run(ctx context.Context, req review.Request) error {
	if p == nil || p.Orchestrator == nil {
		return fmt.Errorf("pipeline: orchestrator is not configured")
	}
	p.withDefaults()

	run, err := p.Jobs.Start(ctx, req)
	if err != nil {
		return err
	}

	rc := review.NewContext(req)
	if recall, err := p.Memory.Recall(ctx, req); err != nil {
		_ = p.Jobs.Update(ctx, run.ID, jobs.StatusFailed, err)
		return err
	} else {
		rc.Conventions = joinMemory(recall.Conventions)
		rc.Philosophy = joinMemory(recall.Decisions)
	}

	if _, err := p.Policy.Apply(ctx, rc); err != nil {
		_ = p.Jobs.Update(ctx, run.ID, jobs.StatusFailed, err)
		return err
	}

	validation, err := p.FindingValidator.Validate(ctx, rc, rc.Findings)
	if err != nil {
		_ = p.Jobs.Update(ctx, run.ID, jobs.StatusFailed, err)
		return err
	}
	rc.Findings = validation.Accepted

	if err := p.Publisher.PublishDraft(ctx, rc); err != nil {
		_ = p.Jobs.Update(ctx, run.ID, jobs.StatusFailed, err)
		return err
	}
	if err := p.Jobs.Update(ctx, run.ID, jobs.StatusDrafted, nil); err != nil {
		return err
	}
	return nil
}

// RunPostHIL continues the pipeline after human approval.
func (p *Pipeline) RunPostHIL(ctx context.Context, projectID string, mrIID int, approvedReport string) error {
	if p == nil {
		return fmt.Errorf("pipeline: not configured")
	}
	p.withDefaults()

	req := review.Request{ProjectID: projectID, MRIID: mrIID}
	rc := review.NewContext(req)
	rc.HILApproved = true
	rc.FinalReport = approvedReport

	proposal, err := p.Memory.ProposeUpdate(ctx, rc)
	if err != nil {
		return err
	}
	if err := p.Memory.Write(ctx, proposal); err != nil {
		return err
	}
	if err := p.Publisher.PublishFinal(ctx, rc); err != nil {
		return err
	}
	return nil
}

func (p *Pipeline) withDefaults() {
	if p.Jobs == nil {
		p.Jobs = jobs.NewMemoryStore()
	}
	if p.Policy == nil {
		p.Policy = policy.DefaultFilter{}
	}
	if p.FindingValidator == nil {
		p.FindingValidator = validator.DefaultFindingValidator{}
	}
	if p.Publisher == nil {
		p.Publisher = publisher.NoopPublisher{}
	}
	if p.Memory == nil {
		p.Memory = memory.NoopStore{}
	}
}

func joinMemory(items []string) string {
	var out string
	for i, item := range items {
		if i > 0 {
			out += "\n"
		}
		out += item
	}
	return out
}
