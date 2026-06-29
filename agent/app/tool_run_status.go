package app

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/Y4NN777/7review/agent/pipeline"
	"github.com/Y4NN777/7review/agent/review"
)

type selectedContextStatusDTO struct {
	Run            string                 `json:"run"`
	CorpusSections []sectionStatusDTO     `json:"corpus_sections"`
	Evidence       []evidenceStatusDTO    `json:"evidence_manifest"`
	SkillSections  []sectionStatusDTO     `json:"skill_sections"`
	Memory         review.MemoryRecall    `json:"memory"`
	Model          modelReviewDTO         `json:"model_review"`
	InlineComments []review.InlineComment `json:"inline_comments,omitempty"`
	Warnings       []string               `json:"warnings,omitempty"`
}

type runTimelineDTO struct {
	Run        string              `json:"run"`
	Status     pipeline.RunStatus  `json:"status"`
	EventCount int                 `json:"event_count"`
	Events     []pipeline.RunEvent `json:"events"`
}

type sectionStatusDTO struct {
	Path            string      `json:"path"`
	Title           string      `json:"title"`
	Kind            review.Kind `json:"kind"`
	ContentBytes    int         `json:"content_bytes"`
	ContentLines    int         `json:"content_lines"`
	SelectionReason string      `json:"selection_reason,omitempty"`
}

type evidenceStatusDTO struct {
	Source          string      `json:"source"`
	HeadingOrKey    string      `json:"heading_or_key"`
	Kind            review.Kind `json:"kind"`
	Authority       string      `json:"authority"`
	MatchedSignals  []string    `json:"matched_signals,omitempty"`
	SelectionReason string      `json:"selection_reason"`
	Score           int         `json:"score"`
	ContentBytes    int         `json:"content_bytes"`
}

type diffSummaryDTO struct {
	Run          string           `json:"run"`
	FileCount    int              `json:"file_count"`
	TotalTokens  int              `json:"total_tokens"`
	Additions    int              `json:"additions"`
	Deletions    int              `json:"deletions"`
	Files        []fileDiffDTO    `json:"files"`
	ChangedFiles []changedFileDTO `json:"changed_files"`
}

type modelReviewDTO struct {
	ParseStatus        string `json:"parse_status,omitempty"`
	ParseWarning       string `json:"parse_warning,omitempty"`
	ParsedFindings     int    `json:"parsed_findings"`
	AcceptedFindings   int    `json:"accepted_findings"`
	RejectedFindings   int    `json:"rejected_findings"`
	ProviderTrace      string `json:"provider_trace,omitempty"`
	RawResponseBytes   int    `json:"raw_response_bytes"`
	RawResponseExcerpt string `json:"raw_response_excerpt,omitempty"`
}

type fileDiffDTO struct {
	Path       string `json:"path"`
	TokenCount int    `json:"token_count"`
	PatchLines int    `json:"patch_lines"`
}

type changedFileDTO struct {
	Path      string `json:"path"`
	OldPath   string `json:"old_path,omitempty"`
	Status    string `json:"status,omitempty"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	HasPatch  bool   `json:"has_patch"`
}

type mergeRequestDTO struct {
	Run         string          `json:"run"`
	Provider    string          `json:"provider,omitempty"`
	ProjectID   string          `json:"project_id,omitempty"`
	Repository  string          `json:"repository,omitempty"`
	ChangeID    string          `json:"change_id,omitempty"`
	MRIID       int             `json:"mr_iid,omitempty"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Author      string          `json:"author,omitempty"`
	WebURL      string          `json:"web_url,omitempty"`
	Labels      []string        `json:"labels,omitempty"`
	DiffRefs    review.DiffRefs `json:"diff_refs"`
}

type discussionsDTO struct {
	Run         string              `json:"run"`
	Discussions []review.Discussion `json:"discussions"`
}

func (r appToolRunner) SelectedContext(ctx context.Context, id string) (any, error) {
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	source := sourceForRun(run)
	return selectedContextStatusDTO{
		Run:            run.ID,
		CorpusSections: sectionDTOs(source.CorpusSections),
		Evidence:       evidenceDTOs(source.Evidence, source.CorpusSections),
		SkillSections:  skillSectionDTOs(source.SkillSections, source.Request),
		Memory:         source.Memory,
		Model:          modelReviewStatusDTO(source.Model),
		InlineComments: append([]review.InlineComment(nil), source.InlineComments...),
		Warnings:       append([]string(nil), source.Run.Warnings...),
	}, nil
}

func modelReviewStatusDTO(model review.ModelReview) modelReviewDTO {
	return modelReviewDTO{
		ParseStatus:        model.ParseStatus,
		ParseWarning:       model.ParseWarning,
		ParsedFindings:     model.ParsedFindings,
		AcceptedFindings:   model.AcceptedFindings,
		RejectedFindings:   model.RejectedFindings,
		ProviderTrace:      model.ProviderTrace,
		RawResponseBytes:   model.RawResponseBytes,
		RawResponseExcerpt: model.RawResponseExcerpt,
	}
}

func (r appToolRunner) RunTimeline(ctx context.Context, id string) (any, error) {
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return runTimelineDTO{
		Run:        run.ID,
		Status:     run.Status,
		EventCount: len(run.Events),
		Events:     append([]pipeline.RunEvent(nil), run.Events...),
	}, nil
}

func (r appToolRunner) DiffSummary(ctx context.Context, id string) (any, error) {
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	source := sourceForRun(run)
	diff := source.Diff
	if diff == nil && run.Context != nil {
		diff = run.Context.Diff
	}
	out := diffSummaryDTO{Run: run.ID}
	for _, changed := range source.ChangedFiles {
		out.Additions += changed.Additions
		out.Deletions += changed.Deletions
		out.ChangedFiles = append(out.ChangedFiles, changedFileDTO{
			Path:      changed.NewPath,
			OldPath:   changed.OldPath,
			Status:    changed.Status,
			Additions: changed.Additions,
			Deletions: changed.Deletions,
			HasPatch:  strings.TrimSpace(changed.Patch) != "",
		})
	}
	if diff != nil {
		for _, file := range diff.Files {
			out.TotalTokens += file.TokenCount
			out.Files = append(out.Files, fileDiffDTO{
				Path:       file.Path,
				TokenCount: file.TokenCount,
				PatchLines: countLines(file.Patch),
			})
		}
	}
	out.FileCount = len(out.Files)
	if out.FileCount == 0 {
		out.FileCount = len(out.ChangedFiles)
	}
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	sort.Slice(out.ChangedFiles, func(i, j int) bool { return out.ChangedFiles[i].Path < out.ChangedFiles[j].Path })
	return out, nil
}

func (r appToolRunner) MergeRequest(ctx context.Context, id string) (any, error) {
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	source := sourceForRun(run)
	scm := sourceSCM(source)
	return mergeRequestDTO{
		Run:         run.ID,
		Provider:    sourceProvider(source, run),
		ProjectID:   sourceProjectID(source, run),
		Repository:  scm.Repository,
		ChangeID:    sourceChangeID(source, run),
		MRIID:       scm.MRIID,
		Title:       firstNonEmptyStatus(scm.Title, run.Request.Title),
		Description: firstNonEmptyStatus(scm.Description, run.Request.Description),
		Author:      firstNonEmptyStatus(scm.Author, run.Request.Author),
		WebURL:      firstNonEmptyStatus(scm.WebURL, run.WebURL),
		Labels:      append([]string(nil), scm.Labels...),
		DiffRefs:    scm.DiffRefs,
	}, nil
}

func (r appToolRunner) ChangedFiles(ctx context.Context, id string) (any, error) {
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	source := sourceForRun(run)
	files := make([]changedFileDTO, 0, len(source.ChangedFiles))
	for _, changed := range source.ChangedFiles {
		files = append(files, changedFileDTO{
			Path:      changed.NewPath,
			OldPath:   changed.OldPath,
			Status:    changed.Status,
			Additions: changed.Additions,
			Deletions: changed.Deletions,
			HasPatch:  strings.TrimSpace(changed.Patch) != "",
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return map[string]any{"run": run.ID, "files": files}, nil
}

func (r appToolRunner) Discussions(ctx context.Context, id string) (any, error) {
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	source := sourceForRun(run)
	return discussionsDTO{Run: run.ID, Discussions: append([]review.Discussion(nil), sourceSCM(source).Discussions...)}, nil
}

func sourceForRun(run *pipeline.Run) review.Source {
	if run == nil {
		return review.Source{}
	}
	if run.Source != nil {
		return *run.Source
	}
	if run.Context != nil {
		return run.Context.Source
	}
	return review.Source{Request: run.Request}
}

func firstNonEmptyStatus(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sectionDTOs(sections []review.Section) []sectionStatusDTO {
	out := make([]sectionStatusDTO, 0, len(sections))
	for _, section := range sections {
		out = append(out, sectionStatusDTO{
			Path:         section.Path,
			Title:        section.Title,
			Kind:         section.Kind,
			ContentBytes: len(section.Content),
			ContentLines: countLines(section.Content),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].Path < out[j].Path
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func skillSectionDTOs(sections []review.Section, req review.Request) []sectionStatusDTO {
	out := sectionDTOs(sections)
	for i := range out {
		out[i].SelectionReason = skillSelectionReason(out[i].Title, req)
	}
	return out
}

func skillSelectionReason(name string, req review.Request) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "methodology-review", "project-knowledge", "framework-rules-review", "traceability-review":
		return "baseline skill always active"
	case "github-merge-api":
		if strings.EqualFold(req.Provider, "github") {
			return "matched provider github"
		}
	case "gitlab-merge-api":
		if strings.EqualFold(req.Provider, "gitlab") {
			return "matched provider gitlab"
		}
	}
	var signals []string
	if strings.TrimSpace(req.Title) != "" {
		signals = append(signals, "title")
	}
	if strings.TrimSpace(req.Description) != "" {
		signals = append(signals, "description")
	}
	if len(req.Labels) > 0 {
		signals = append(signals, "labels "+strings.Join(firstStrings(req.Labels, 3), ", "))
	}
	if len(req.ChangedPaths) > 0 {
		signals = append(signals, "changed paths "+strings.Join(firstStrings(req.ChangedPaths, 3), ", "))
	}
	if len(signals) == 0 {
		return "matched review request signals"
	}
	return "matched " + strings.Join(signals, " + ")
}

func firstStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return append([]string(nil), values...)
	}
	out := append([]string(nil), values[:limit]...)
	out = append(out, "+"+strconv.Itoa(len(values)-limit)+" more")
	return out
}

func evidenceDTOs(items []review.EvidenceItem, sections []review.Section) []evidenceStatusDTO {
	if len(items) == 0 && len(sections) > 0 {
		items = make([]review.EvidenceItem, 0, len(sections))
		for _, section := range sections {
			items = append(items, review.EvidenceItem{
				Source:          section.Path,
				HeadingOrKey:    section.Title,
				Kind:            section.Kind,
				Authority:       "unknown",
				SelectionReason: "legacy selected context",
				ContentBytes:    len(section.Content),
			})
		}
	}
	out := make([]evidenceStatusDTO, 0, len(items))
	for _, item := range items {
		out = append(out, evidenceStatusDTO{
			Source:          item.Source,
			HeadingOrKey:    item.HeadingOrKey,
			Kind:            item.Kind,
			Authority:       item.Authority,
			MatchedSignals:  append([]string(nil), item.MatchedSignals...),
			SelectionReason: item.SelectionReason,
			Score:           item.Score,
			ContentBytes:    item.ContentBytes,
		})
	}
	return out
}

func countLines(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}
