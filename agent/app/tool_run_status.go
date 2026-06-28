package app

import (
	"context"
	"sort"
	"strings"

	"github.com/Y4NN777/7review/agent/pipeline"
	"github.com/Y4NN777/7review/agent/review"
)

type selectedContextStatusDTO struct {
	Run            string              `json:"run"`
	CorpusSections []sectionStatusDTO  `json:"corpus_sections"`
	SkillSections  []sectionStatusDTO  `json:"skill_sections"`
	Memory         review.MemoryRecall `json:"memory"`
	Warnings       []string            `json:"warnings,omitempty"`
}

type runTimelineDTO struct {
	Run        string              `json:"run"`
	Status     pipeline.RunStatus  `json:"status"`
	EventCount int                 `json:"event_count"`
	Events     []pipeline.RunEvent `json:"events"`
}

type sectionStatusDTO struct {
	Path         string      `json:"path"`
	Title        string      `json:"title"`
	Kind         review.Kind `json:"kind"`
	ContentBytes int         `json:"content_bytes"`
	ContentLines int         `json:"content_lines"`
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

func (r appToolRunner) SelectedContext(ctx context.Context, id string) (any, error) {
	run, err := r.server.pipeline.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	source := sourceForRun(run)
	return selectedContextStatusDTO{
		Run:            run.ID,
		CorpusSections: sectionDTOs(source.CorpusSections),
		SkillSections:  sectionDTOs(source.SkillSections),
		Memory:         source.Memory,
		Warnings:       append([]string(nil), source.Run.Warnings...),
	}, nil
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

func countLines(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}
