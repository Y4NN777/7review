package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Y4NN777/7review/agent/config"
	"github.com/Y4NN777/7review/agent/orchestrator"
	"github.com/Y4NN777/7review/agent/review"
	"github.com/Y4NN777/7review/agent/skills"
	"github.com/Y4NN777/7review/agent/tools"
)

// Pipeline coordinates the review workflow for one merge request.
type Pipeline struct {
	Config           *config.Config
	SkillLoader      *skills.Loader
	Orchestrator     *orchestrator.Orchestrator
	Jobs             RunStore
	Policy           PolicyFilter
	FindingValidator FindingValidator
	Memory           MemoryStore
	ContextReducer   ContextReducer
	SCM              tools.SCM
	SCMPublisher     tools.Publisher
}

// Run executes the automated review pipeline.
func (p *Pipeline) Run(ctx context.Context, req review.Request) error {
	if p == nil || p.Orchestrator == nil {
		return fmt.Errorf("pipeline: orchestrator is not configured")
	}
	p.withDefaults()
	if err := p.requireConfiguredAdapters(); err != nil {
		return err
	}

	run, err := p.Jobs.Start(ctx, req)
	if err != nil {
		return err
	}

	rc := review.NewContext(req)
	rc.Source.Run.ID = run.ID
	rc.AvailableTools = []string{"scm", "diff", "context", "review", "report", "memory"}
	rc.Source.Run.AvailableTools = append([]string(nil), rc.AvailableTools...)

	scmContext, err := p.SCM.Enrich(ctx, req)
	if err != nil {
		_ = p.Jobs.Update(ctx, run.ID, StatusFailed, err)
		return err
	}
	applySCMContext(rc, scmContext)

	rc.Diff = normalizeDiff(scmContext.Files)
	rc.Source.Diff = rc.Diff
	rc.Request.ChangedPaths = rc.ChangedPaths()
	if p.SkillLoader != nil {
		rc.SkillSections = p.SkillLoader.Select(rc.Request)
		rc.Source.SkillSections = rc.SkillSections
	}
	rc.CorpusSections, err = selectCorpus(ctx, p.corpusRoot(), rc)
	if err != nil {
		_ = p.Jobs.Update(ctx, run.ID, StatusFailed, err)
		return err
	}

	if recall, err := p.Memory.Recall(ctx, req); err != nil {
		_ = p.Jobs.Update(ctx, run.ID, StatusFailed, err)
		return err
	} else {
		rc.Conventions = joinMemory(recall.Conventions)
		rc.Philosophy = joinMemory(recall.Decisions)
		rc.Source.Memory = review.MemoryRecall{
			Conventions: recall.Conventions,
			Decisions:   recall.Decisions,
			History:     recall.History,
		}
	}

	if _, err := p.Policy.Apply(ctx, rc); err != nil {
		_ = p.Jobs.Update(ctx, run.ID, StatusFailed, err)
		return err
	}
	if err := p.ContextReducer.Reduce(ctx, rc); err != nil {
		_ = p.Jobs.Update(ctx, run.ID, StatusFailed, err)
		return err
	}

	findings, err := p.runReview(ctx, rc)
	if err != nil {
		_ = p.Jobs.Update(ctx, run.ID, StatusFailed, err)
		return err
	}
	rc.Findings = findings

	validation, err := p.FindingValidator.Validate(ctx, rc, findings)
	if err != nil {
		_ = p.Jobs.Update(ctx, run.ID, StatusFailed, err)
		return err
	}
	rc.Findings = validation.Accepted
	rc.Source.Findings = validation.Accepted
	rc.DraftReport = renderReport(rc)
	rc.Source.Report.Draft = rc.DraftReport

	if err := p.SCMPublisher.PublishDraft(ctx, scmContext, rc.DraftReport); err != nil {
		_ = p.Jobs.SaveContext(ctx, run.ID, rc)
		_ = p.Jobs.Update(ctx, run.ID, StatusFailed, err)
		return err
	}
	if err := p.Jobs.SaveContext(ctx, run.ID, rc); err != nil {
		return err
	}
	if err := p.Jobs.Update(ctx, run.ID, StatusDrafted, nil); err != nil {
		return err
	}
	return nil
}

// RunPostHIL continues the pipeline after human approval.
func (p *Pipeline) RunPostHIL(ctx context.Context, projectID string, mrIID int, approvedReport string) error {
	return p.ApproveRun(ctx, runID(projectID, mrIID), approvedReport)
}

func (p *Pipeline) ApproveRun(ctx context.Context, id string, approvedReport string) error {
	if p == nil {
		return fmt.Errorf("pipeline: not configured")
	}
	p.withDefaults()
	if err := p.requireConfiguredAdapters(); err != nil {
		return err
	}

	run, err := p.Jobs.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := ensureApprovableRun(run); err != nil {
		return err
	}
	rc := contextForRun(run)
	finalReport := strings.TrimSpace(approvedReport)
	if finalReport == "" {
		finalReport = finalizeReport(run.DraftReport)
	}
	if finalReport == "" {
		return fmt.Errorf("pipeline: approved report is empty")
	}

	rc.HILApproved = true
	rc.FinalReport = finalReport
	rc.Source.Report.Final = finalReport
	if err := p.Jobs.SaveContext(ctx, id, rc); err != nil {
		return err
	}
	if err := p.Jobs.Update(ctx, id, StatusFinalizing, nil); err != nil {
		return err
	}
	if err := p.publishFinal(ctx, rc, finalReport); err != nil {
		_ = p.Jobs.Update(ctx, id, StatusFailed, err)
		return err
	}
	if err := p.writeApprovedMemory(ctx, rc); err != nil {
		_ = p.Jobs.Update(ctx, id, StatusFailed, err)
		return err
	}
	if err := p.Jobs.SaveContext(ctx, id, rc); err != nil {
		return err
	}
	return p.Jobs.Update(ctx, id, StatusFinalized, nil)
}

func ensureApprovableRun(run *Run) error {
	if run == nil {
		return fmt.Errorf("pipeline: run is not loaded")
	}
	if strings.TrimSpace(run.DraftReport) == "" {
		return fmt.Errorf("pipeline: draft report required before HIL approval")
	}
	switch run.Status {
	case StatusDrafted:
		return nil
	case StatusFailed:
		if run.HILApproved {
			return fmt.Errorf("pipeline: use final publish retry for already approved failed run")
		}
		return nil
	default:
		return fmt.Errorf("pipeline: run status %q cannot be approved", run.Status)
	}
}

// PublishFinal republishes an already approved final report. It is useful for
// explicit publish commands and retries after transient SCM failures.
func (p *Pipeline) PublishFinal(ctx context.Context, id string, report string) error {
	if p == nil {
		return fmt.Errorf("pipeline: not configured")
	}
	p.withDefaults()
	if err := p.requireConfiguredAdapters(); err != nil {
		return err
	}

	run, err := p.Jobs.Get(ctx, id)
	if err != nil {
		return err
	}
	rc := contextForRun(run)
	if !rc.HILApproved {
		return fmt.Errorf("pipeline: HIL approval required before final publish")
	}
	finalReport := strings.TrimSpace(report)
	if finalReport == "" {
		finalReport = finalizeReport(rc.FinalReport)
	}
	if finalReport == "" {
		return fmt.Errorf("pipeline: final report is empty")
	}
	rc.FinalReport = finalReport
	rc.Source.Report.Final = finalReport
	if err := p.Jobs.Update(ctx, id, StatusFinalizing, nil); err != nil {
		return err
	}
	if err := p.publishFinal(ctx, rc, finalReport); err != nil {
		_ = p.Jobs.Update(ctx, id, StatusFailed, err)
		return err
	}
	if err := p.writeApprovedMemory(ctx, rc); err != nil {
		_ = p.Jobs.Update(ctx, id, StatusFailed, err)
		return err
	}
	if err := p.Jobs.SaveContext(ctx, id, rc); err != nil {
		return err
	}
	return p.Jobs.Update(ctx, id, StatusFinalized, nil)
}

func (p *Pipeline) SuppressFinding(ctx context.Context, id string, findingID string, reason string) error {
	if p == nil {
		return fmt.Errorf("pipeline: not configured")
	}
	p.withDefaults()
	findingID = strings.TrimSpace(findingID)
	reason = strings.TrimSpace(reason)
	if findingID == "" {
		return fmt.Errorf("pipeline: finding id is required")
	}
	if reason == "" {
		return fmt.Errorf("pipeline: suppression reason is required")
	}

	run, err := p.Jobs.Get(ctx, id)
	if err != nil {
		return err
	}
	switch run.Status {
	case StatusDrafted, StatusFailed:
	default:
		return fmt.Errorf("pipeline: run status %q cannot suppress findings", run.Status)
	}
	rc := contextForRun(run)
	var kept []review.Finding
	found := false
	for _, finding := range rc.Findings {
		if strings.EqualFold(strings.TrimSpace(finding.ID), findingID) {
			found = true
			continue
		}
		kept = append(kept, finding)
	}
	if !found {
		return fmt.Errorf("pipeline: finding %q not found", findingID)
	}
	rc.Findings = kept
	rc.Source.Findings = kept
	rc.HILRejectedIDs = appendUnique(rc.HILRejectedIDs, findingID)
	rc.HILAddedNotes = append(rc.HILAddedNotes, fmt.Sprintf("suppressed %s: %s", findingID, reason))
	rc.DraftReport = renderReport(rc)
	rc.Source.Report.Draft = rc.DraftReport
	if err := p.Jobs.SaveContext(ctx, id, rc); err != nil {
		return err
	}
	return p.Jobs.Update(ctx, id, StatusDrafted, nil)
}

func (p *Pipeline) publishFinal(ctx context.Context, rc *review.Context, finalReport string) error {
	if rc == nil || !rc.HILApproved {
		return fmt.Errorf("pipeline: HIL approval required before final publish")
	}
	source := rc.Source.SCM
	if source == nil {
		enriched, err := p.SCM.Enrich(ctx, rc.Request)
		if err != nil {
			return err
		}
		source = enriched
		rc.Source.SCM = source
	}
	return p.SCMPublisher.PublishFinal(ctx, source, finalReport)
}

func (p *Pipeline) writeApprovedMemory(ctx context.Context, rc *review.Context) error {
	proposal, err := p.Memory.ProposeUpdate(ctx, rc)
	if err != nil {
		return err
	}
	return p.Memory.Write(ctx, proposal)
}

func contextForRun(run *Run) *review.Context {
	if run == nil {
		return review.NewContext(review.Request{})
	}
	if run.Context != nil {
		return run.Context
	}
	rc := review.NewContext(run.Request)
	rc.DraftReport = run.DraftReport
	rc.FinalReport = run.FinalReport
	rc.HILApproved = run.HILApproved
	rc.Findings = append([]review.Finding(nil), run.Findings...)
	rc.WebURL = run.WebURL
	rc.Source.Findings = rc.Findings
	rc.Source.Report.Draft = rc.DraftReport
	rc.Source.Report.Final = rc.FinalReport
	return rc
}

func runID(projectID string, mrIID int) string {
	return projectID + "!" + strconv.Itoa(mrIID)
}

func finalizeReport(report string) string {
	report = strings.TrimSpace(report)
	if report == "" {
		return ""
	}
	report = strings.Replace(report, "## 7review Draft", "## 7review Final", 1)
	return report
}

func appendUnique(items []string, item string) []string {
	for _, existing := range items {
		if strings.EqualFold(strings.TrimSpace(existing), item) {
			return items
		}
	}
	return append(items, item)
}

func (p *Pipeline) withDefaults() {
	if p.Jobs == nil {
		p.Jobs = NewMemoryRunStore()
	}
	if p.Policy == nil {
		p.Policy = DefaultPolicyFilter{}
	}
	if p.FindingValidator == nil {
		p.FindingValidator = DefaultFindingValidator{}
	}
	if p.Memory == nil {
		p.Memory = NoopMemoryStore{}
	}
	if p.ContextReducer == nil {
		p.ContextReducer = NoopContextReducer{}
	}
	if p.SCM == nil {
		p.SCM = tools.NoopSCM{}
	}
	if p.SCMPublisher == nil {
		p.SCMPublisher = tools.NoopPublisher{}
	}
}

func (p *Pipeline) requireConfiguredAdapters() error {
	if p == nil || p.Config == nil {
		return nil
	}
	var missing []string
	if strings.TrimSpace(p.Config.HeadroomURL) != "" && isNoopContextReducer(p.ContextReducer) {
		missing = append(missing, "headroom context reducer")
	}
	if strings.TrimSpace(p.Config.MemPalaceURL) != "" && isNoopMemoryStore(p.Memory) {
		missing = append(missing, "mempalace memory store")
	}
	if hasConfiguredSCM(p.Config) && isNoopSCM(p.SCM) {
		missing = append(missing, "SCM enrichment adapter")
	}
	if hasConfiguredSCM(p.Config) && isNoopPublisher(p.SCMPublisher) {
		missing = append(missing, "SCM publisher adapter")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("pipeline: configured production dependencies are missing adapters: %s", strings.Join(missing, ", "))
}

func hasConfiguredSCM(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	hasGitLab := strings.TrimSpace(cfg.GitLabURL) != "" && strings.TrimSpace(cfg.GitLabToken) != "" && strings.TrimSpace(cfg.WebhookSecret) != ""
	hasGitHub := strings.TrimSpace(cfg.GitHubAPIURL) != "" && strings.TrimSpace(cfg.GitHubToken) != "" && strings.TrimSpace(cfg.GitHubWebhookSecret) != ""
	return hasGitLab || hasGitHub
}

func isNoopMemoryStore(memory MemoryStore) bool {
	switch memory.(type) {
	case NoopMemoryStore, *NoopMemoryStore:
		return true
	default:
		return false
	}
}

func isNoopContextReducer(reducer ContextReducer) bool {
	switch reducer.(type) {
	case NoopContextReducer, *NoopContextReducer:
		return true
	default:
		return false
	}
}

func isNoopSCM(scm tools.SCM) bool {
	switch scm.(type) {
	case tools.NoopSCM, *tools.NoopSCM:
		return true
	default:
		return false
	}
}

func isNoopPublisher(publisher tools.Publisher) bool {
	switch publisher.(type) {
	case tools.NoopPublisher, *tools.NoopPublisher:
		return true
	default:
		return false
	}
}

func (p *Pipeline) corpusRoot() string {
	if p != nil && p.Config != nil && strings.TrimSpace(p.Config.CorpusRoot) != "" {
		return p.Config.CorpusRoot
	}
	return "."
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

func applySCMContext(rc *review.Context, scmContext *review.SCMContext) {
	if scmContext == nil {
		return
	}
	rc.Source.SCM = scmContext
	rc.Source.ChangedFiles = scmContext.Files
	rc.MRTitle = scmContext.Title
	rc.MRAuthor = scmContext.Author
	rc.WebURL = scmContext.WebURL
	rc.DiffRefs = scmContext.DiffRefs
	if rc.Request.Title == "" {
		rc.Request.Title = scmContext.Title
	}
	if rc.Request.Description == "" {
		rc.Request.Description = scmContext.Description
	}
	if len(rc.Request.Labels) == 0 {
		rc.Request.Labels = scmContext.Labels
	}
}

func normalizeDiff(files []review.ChangedFile) *review.StructuredDiff {
	out := &review.StructuredDiff{Files: make([]review.FileDiff, 0, len(files))}
	for _, file := range files {
		path := filepath.ToSlash(file.NewPath)
		if path == "" {
			path = filepath.ToSlash(file.OldPath)
		}
		out.Files = append(out.Files, review.FileDiff{
			Path:       path,
			Patch:      file.Patch,
			TokenCount: estimateTokens(file.Patch),
		})
	}
	return out
}

func (p *Pipeline) runReview(ctx context.Context, rc *review.Context) ([]review.Finding, error) {
	if p.Orchestrator == nil || rc.Diff == nil || len(rc.Diff.Files) == 0 {
		return nil, nil
	}
	maxTokens := 6000
	if p.Config != nil && p.Config.MaxDiffTokens > 0 {
		maxTokens = p.Config.MaxDiffTokens
	}
	batches := chunkDiff(rc.Diff.Files, maxTokens)
	err := p.Orchestrator.CompleteParallel(ctx, rc, reviewSystemPrompt(rc), batches, func(batch []review.FileDiff) string {
		return reviewUserMessage(rc, batch)
	})
	if err != nil {
		return nil, err
	}
	return parseFindings(strings.Join(rc.AllFindings(), "\n")), nil
}

func chunkDiff(files []review.FileDiff, maxTokens int) [][]review.FileDiff {
	if maxTokens <= 0 {
		maxTokens = 6000
	}
	var chunks [][]review.FileDiff
	var current []review.FileDiff
	total := 0
	for _, file := range files {
		if len(current) > 0 && total+file.TokenCount > maxTokens {
			chunks = append(chunks, current)
			current = nil
			total = 0
		}
		current = append(current, file)
		total += file.TokenCount
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks
}

func reviewSystemPrompt(rc *review.Context) string {
	var b strings.Builder
	b.WriteString("You are a code review agent. Return only JSON: either an array of findings or {\"findings\": [...]}. ")
	b.WriteString("Each finding must include id, severity, title, description, suggestion, location, and confidence. ")
	b.WriteString("Only report actionable issues in changed files.\n")
	for _, section := range append(rc.SkillSections, rc.CorpusSections...) {
		fmt.Fprintf(&b, "\n# %s (%s)\n%s\n", section.Title, section.Kind, section.Content)
	}
	return b.String()
}

func reviewUserMessage(rc *review.Context, batch []review.FileDiff) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Review %s change %s: %s\n\n", rc.Request.Provider, rc.Request.ChangeID, rc.Request.Title)
	for _, file := range batch {
		fmt.Fprintf(&b, "## %s\n```diff\n%s\n```\n", file.Path, file.Patch)
	}
	return b.String()
}

func parseFindings(text string) []review.Finding {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	for _, candidate := range jsonCandidates(text) {
		if findings, ok := decodeFindings(candidate); ok {
			return findings
		}
	}
	return nil
}

func decodeFindings(text string) ([]review.Finding, bool) {
	var findings []review.Finding
	if err := json.Unmarshal([]byte(text), &findings); err == nil {
		return findings, true
	}
	var envelope struct {
		Findings *[]review.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err == nil && envelope.Findings != nil {
		return *envelope.Findings, true
	}
	return nil, false
}

func jsonCandidates(text string) []string {
	var candidates []string
	trimmed := strings.TrimSpace(text)
	candidates = append(candidates, trimmed)
	candidates = append(candidates, fencedJSONBlocks(trimmed)...)
	candidates = append(candidates, balancedJSONSpans(trimmed, '{', '}')...)
	candidates = append(candidates, balancedJSONSpans(trimmed, '[', ']')...)
	return candidates
}

func fencedJSONBlocks(text string) []string {
	var blocks []string
	remaining := text
	for {
		start := strings.Index(remaining, "```")
		if start < 0 {
			return blocks
		}
		afterFence := remaining[start+3:]
		newline := strings.Index(afterFence, "\n")
		if newline < 0 {
			return blocks
		}
		label := strings.TrimSpace(afterFence[:newline])
		contentStart := newline + 1
		end := strings.Index(afterFence[contentStart:], "```")
		if end < 0 {
			return blocks
		}
		content := strings.TrimSpace(afterFence[contentStart : contentStart+end])
		if label == "" || strings.EqualFold(label, "json") {
			blocks = append(blocks, content)
		}
		remaining = afterFence[contentStart+end+3:]
	}
}

func balancedJSONSpans(text string, open, close rune) []string {
	var spans []string
	for start, r := range text {
		if r != open {
			continue
		}
		if span := sliceBalancedJSONFrom(text, start, open, close); span != "" {
			spans = append(spans, span)
		}
	}
	return spans
}

func sliceBalancedJSONFrom(text string, start int, open, close rune) string {
	depth := 0
	inString := false
	escaped := false
	for idx, r := range text[start:] {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inString:
			escaped = true
		case r == '"':
			inString = !inString
		case !inString && r == open:
			depth++
		case !inString && r == close:
			depth--
			if depth == 0 {
				return strings.TrimSpace(text[start : start+idx+1])
			}
		}
	}
	return ""
}

func renderReport(rc *review.Context) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- 7review:bot-report project=%s change=%s -->\n", rc.Request.ProjectID, rc.Request.ChangeID)
	b.WriteString("## 7review Draft\n\n")
	if len(rc.Findings) == 0 {
		b.WriteString("No validated findings.\n")
		appendWarnings(&b, rc)
		return b.String()
	}
	for _, finding := range rc.Findings {
		fmt.Fprintf(&b, "### %s: %s\n\n", finding.Severity, finding.Title)
		if finding.Location.Path != "" {
			fmt.Fprintf(&b, "**Location:** `%s`", finding.Location.Path)
			if finding.Location.Line > 0 {
				fmt.Fprintf(&b, ":%d", finding.Location.Line)
			}
			b.WriteString("\n\n")
		}
		if finding.Description != "" {
			fmt.Fprintf(&b, "%s\n\n", finding.Description)
		}
		if finding.Suggestion != "" {
			fmt.Fprintf(&b, "**Suggestion:** %s\n\n", finding.Suggestion)
		}
	}
	appendWarnings(&b, rc)
	return b.String()
}

func appendWarnings(b *strings.Builder, rc *review.Context) {
	if len(rc.Run.Warnings) == 0 {
		return
	}
	b.WriteString("\n---\nWarnings:\n")
	for _, warning := range rc.Run.Warnings {
		fmt.Fprintf(b, "- %s\n", warning)
	}
}

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	byChars := len(text) / 4
	byWords := len(strings.Fields(text))
	if byChars > byWords {
		return byChars
	}
	return byWords
}

func selectCorpus(ctx context.Context, root string, rc *review.Context) ([]review.Section, error) {
	candidates, err := discoverCorpus(ctx, root)
	if err != nil {
		return nil, err
	}
	query := strings.ToLower(strings.Join(append([]string{rc.Request.Title, rc.Request.Description}, append(rc.Request.Labels, rc.ChangedPaths()...)...), " "))
	ids := idPattern.FindAllString(query, -1)
	type scored struct {
		section review.Section
		score   int
	}
	var scoredSections []scored
	for _, section := range candidates {
		score := 0
		path := strings.ToLower(section.Path)
		content := strings.ToLower(section.Content)
		for _, changed := range rc.ChangedPaths() {
			for _, part := range strings.Split(strings.ToLower(filepath.ToSlash(changed)), "/") {
				if part != "" && strings.Contains(path, part) {
					score += 3
				}
			}
		}
		for _, id := range ids {
			if strings.Contains(content, strings.ToLower(id)) || strings.Contains(path, strings.ToLower(id)) {
				score += 5
			}
		}
		if score == 0 && (section.Kind == review.KindRules || section.Kind == review.KindArchitecture) {
			score = 1
		}
		if score > 0 {
			scoredSections = append(scoredSections, scored{section: section, score: score})
		}
	}
	sort.SliceStable(scoredSections, func(i, j int) bool {
		if scoredSections[i].score == scoredSections[j].score {
			return scoredSections[i].section.Path < scoredSections[j].section.Path
		}
		return scoredSections[i].score > scoredSections[j].score
	})
	if len(scoredSections) > 64 {
		scoredSections = scoredSections[:64]
	}
	out := make([]review.Section, 0, len(scoredSections))
	for _, item := range scoredSections {
		out = append(out, item.section)
	}
	return out, nil
}

func discoverCorpus(ctx context.Context, root string) ([]review.Section, error) {
	var sections []review.Section
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		kind, ok := classifyCorpus(rel)
		if !ok {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > 128*1024 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sections = append(sections, review.Section{
			Path:    rel,
			Title:   filepath.Base(rel),
			Content: string(data),
			Kind:    kind,
		})
		return nil
	})
	return sections, err
}

func classifyCorpus(path string) (review.Kind, bool) {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lower)
	switch {
	case strings.Contains(lower, "rule"), strings.Contains(lower, "convention"), base == "agents.md":
		return review.KindRules, true
	case strings.Contains(lower, "prd"), strings.Contains(lower, "srs"), strings.Contains(lower, "requirement"), strings.Contains(lower, "planning"):
		return review.KindPlanning, true
	case strings.Contains(lower, "contract"), strings.Contains(lower, "schema"), strings.Contains(lower, "protobuf"), strings.Contains(lower, "openapi"):
		return review.KindContract, true
	case strings.Contains(lower, "adr"), strings.Contains(lower, "architecture"), strings.Contains(lower, "design-doc"):
		return review.KindArchitecture, true
	case strings.Contains(lower, "api"):
		return review.KindAPI, true
	case strings.Contains(lower, "security"), strings.Contains(lower, "threat"):
		return review.KindSecurity, true
	case strings.Contains(lower, "design-token"), strings.Contains(lower, "tokens"):
		return review.KindDesign, true
	case strings.Contains(lower, "release"), strings.Contains(lower, "runbook"), strings.Contains(lower, "delivery"):
		return review.KindDelivery, true
	default:
		return "", false
	}
}

var idPattern = regexp.MustCompile(`(?i)\b(?:FR|INV|PRO|ADR)-[0-9A-Za-z._-]+\b`)
