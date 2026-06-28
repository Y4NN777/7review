package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/Y4NN777/7review/agent/review"
)

const (
	maxSelectedCorpusSections          = 24
	defaultMaxSupportingCorpusSections = 3
	maxCorpusSectionBytes              = 20 * 1024
	maxCorpusDocumentBytes             = 512 * 1024
)

type reviewSignals struct {
	Text       string
	IDs        map[string]struct{}
	Routes     map[string]struct{}
	Terms      map[string]struct{}
	PathParts  map[string]struct{}
	Components map[string]struct{}
	Entities   map[string]struct{}
	Skills     map[string]struct{}
	CodeRules  map[string]struct{}
}

type corpusDocument struct {
	Path      string
	Kind      review.Kind
	Authority string
	Content   string
}

type corpusSection struct {
	review.Section
	Authority   string
	HeadingPath []string
	Level       int
	Ordinal     int
}

type scoredCorpusSection struct {
	section        corpusSection
	score          int
	matchedSignals []string
	reason         string
}

type corpusFeature struct {
	section     corpusSection
	ids         map[string]struct{}
	routes      map[string]struct{}
	tokens      map[string]struct{}
	pathTokens  map[string]struct{}
	titleTokens map[string]struct{}
}

type corpusIndex struct {
	features []corpusFeature
	docFreq  map[string]int
	total    int
}

func selectCorpus(ctx context.Context, root string, rc *review.Context, maxSupporting int) ([]review.Section, []review.EvidenceItem, error) {
	docs, err := discoverCorpusDocuments(ctx, root)
	if err != nil {
		return nil, nil, err
	}
	signals := extractReviewSignals(rc.Request, rc.Diff, rc.Source.SCM, rc.SkillSections)
	var all []corpusSection
	var scored []scoredCorpusSection
	for _, doc := range docs {
		sections := splitCorpusDocument(doc)
		all = append(all, sections...)
	}
	index := buildCorpusIndex(all)
	graph := buildCorpusGraph(all)
	for _, feature := range index.features {
		item := scoreCorpusFeature(feature, signals, index)
		if item.score > 0 {
			scored = append(scored, item)
		}
	}
	scored = ensureCodeRuleOverviews(scored, index, signals)
	seeds := matchGraphSeeds(graph, signals)
	scored = mergeGraphSeedEvidence(scored, graph, seeds)
	sortScoredCorpus(scored)
	scored = expandGraphEvidence(graph, scored, seeds, graphExpansionLimits{PerSeed: 2})
	scored = expandGraphHierarchyEvidence(graph, scored, graphExpansionLimits{PerSeed: 2})
	sortScoredCorpus(scored)
	scored = limitSelectedCorpus(scored, maxSupporting)
	sections := make([]review.Section, 0, len(scored))
	for _, item := range scored {
		sections = append(sections, item.section.Section)
	}
	return sections, buildEvidenceManifest(scored), nil
}

func sortScoredCorpus(scored []scoredCorpusSection) {
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			if scored[i].section.Path == scored[j].section.Path {
				return scored[i].section.Title < scored[j].section.Title
			}
			return scored[i].section.Path < scored[j].section.Path
		}
		return scored[i].score > scored[j].score
	})
}

func ensureCodeRuleOverviews(scored []scoredCorpusSection, index corpusIndex, signals reviewSignals) []scoredCorpusSection {
	if len(signals.CodeRules) == 0 {
		return scored
	}
	selected := make(map[string]int, len(scored))
	for i, item := range scored {
		selected[sectionKey(item.section)] = i
	}
	for rule := range signals.CodeRules {
		var fallback *corpusSection
		for _, feature := range index.features {
			section := feature.section
			if corpusCodeRule(section.Path) != rule {
				continue
			}
			if fallback == nil {
				copy := section
				fallback = &copy
			}
			if !languageRuleOverview(section) {
				continue
			}
			key := sectionKey(section)
			if idx, ok := selected[key]; ok {
				scored[idx].score = max(scored[idx].score, 2000+authorityScore(section))
				scored[idx].reason = selectionReason(section, []string{"changed code language " + rule, "baseline code-zone rules"})
				break
			}
			selected[key] = len(scored)
			scored = append(scored, scoredCorpusSection{
				section:        section,
				score:          2000 + authorityScore(section),
				matchedSignals: []string{rule},
				reason:         selectionReason(section, []string{"changed code language " + rule, "baseline code-zone rules"}),
			})
			break
		}
		if fallback == nil {
			continue
		}
		key := sectionKey(*fallback)
		if idx, ok := selected[key]; ok {
			scored[idx].score = max(scored[idx].score, 2000+authorityScore(*fallback))
			scored[idx].reason = selectionReason(*fallback, []string{"changed code language " + rule, "baseline code-zone rules"})
			continue
		}
		selected[key] = len(scored)
		scored = append(scored, scoredCorpusSection{
			section:        *fallback,
			score:          2000 + authorityScore(*fallback),
			matchedSignals: []string{rule},
			reason:         selectionReason(*fallback, []string{"changed code language " + rule, "baseline code-zone rules"}),
		})
	}
	return scored
}

func limitSelectedCorpus(scored []scoredCorpusSection, maxSupporting int) []scoredCorpusSection {
	if maxSupporting <= 0 {
		maxSupporting = defaultMaxSupportingCorpusSections
	}
	out := make([]scoredCorpusSection, 0, maxSelectedCorpusSections)
	supporting := 0
	for _, item := range scored {
		if isSupportingCorpusSection(item.section) {
			if supporting >= maxSupporting {
				continue
			}
			supporting++
		}
		out = append(out, item)
		if len(out) == maxSelectedCorpusSections {
			break
		}
	}
	return out
}

func expandRelatedCorpus(scored []scoredCorpusSection, all []corpusSection) []scoredCorpusSection {
	if len(scored) == 0 {
		return scored
	}
	bySource := make(map[string][]corpusSection)
	for _, section := range all {
		bySource[section.Path] = append(bySource[section.Path], section)
	}
	out := append([]scoredCorpusSection(nil), scored...)
	selected := make(map[string]struct{}, len(scored))
	for _, item := range scored {
		selected[sectionKey(item.section)] = struct{}{}
	}
	relatedBySource := make(map[string]int)
	for _, seed := range scored {
		if !relatedContextEligible(seed.section) || seed.score < 20 {
			continue
		}
		sourceSections := bySource[seed.section.Path]
		for _, candidate := range relatedCandidates(seed.section, sourceSections) {
			if relatedBySource[candidate.Path] >= 2 {
				break
			}
			key := sectionKey(candidate)
			if _, ok := selected[key]; ok {
				continue
			}
			selected[key] = struct{}{}
			relatedBySource[candidate.Path]++
			out = append(out, scoredCorpusSection{
				section:        candidate,
				score:          relatedScore(seed.score, candidate),
				matchedSignals: append([]string(nil), seed.matchedSignals...),
				reason:         relatedSelectionReason(seed.section, candidate),
			})
		}
	}
	return out
}

func relatedContextEligible(section corpusSection) bool {
	switch section.Kind {
	case review.KindDesign, review.KindArchitecture, review.KindPlanning, review.KindContract:
		return true
	default:
		return false
	}
}

func relatedCandidates(seed corpusSection, sections []corpusSection) []corpusSection {
	var out []corpusSection
	if parent := parentSection(seed, sections); parent != nil {
		out = append(out, *parent)
	}
	if overview := overviewSection(seed, sections); overview != nil {
		out = append(out, *overview)
	}
	return out
}

func parentSection(seed corpusSection, sections []corpusSection) *corpusSection {
	if len(seed.HeadingPath) < 2 {
		return nil
	}
	parentTitle := seed.HeadingPath[len(seed.HeadingPath)-2]
	for i := range sections {
		section := sections[i]
		if section.Path == seed.Path && section.Title == parentTitle && section.Level < seed.Level {
			return &section
		}
	}
	return nil
}

func overviewSection(seed corpusSection, sections []corpusSection) *corpusSection {
	var first *corpusSection
	for i := range sections {
		section := sections[i]
		if section.Path != seed.Path || section.Title == seed.Title {
			continue
		}
		if first == nil || section.Ordinal < first.Ordinal {
			copy := section
			first = &copy
		}
		if isOverviewTitle(section.Title) {
			copy := section
			return &copy
		}
	}
	return first
}

func isOverviewTitle(title string) bool {
	title = strings.ToLower(title)
	switch {
	case strings.Contains(title, "overview"),
		strings.Contains(title, "vue d'ensemble"),
		strings.Contains(title, "summary"),
		strings.Contains(title, "résumé"),
		strings.Contains(title, "contexte"),
		strings.Contains(title, "context"),
		strings.Contains(title, "principle"),
		strings.Contains(title, "principe"),
		strings.Contains(title, "périmètre"),
		strings.Contains(title, "scope"):
		return true
	default:
		return false
	}
}

func relatedScore(seedScore int, section corpusSection) int {
	score := seedScore / 2
	if score < 45 {
		score = 45
	}
	if score > 80 {
		score = 80
	}
	return score + authorityScore(section)
}

func relatedSelectionReason(seed, candidate corpusSection) string {
	relation := "related context"
	if isOverviewTitle(candidate.Title) || candidate.Ordinal == 0 {
		relation = "document overview"
	}
	if len(seed.HeadingPath) > 1 && candidate.Title == seed.HeadingPath[len(seed.HeadingPath)-2] {
		relation = "parent heading"
	}
	return fmt.Sprintf("%s: selected with %s#%s", relation, seed.Path, seed.Title)
}

func sectionKey(section corpusSection) string {
	return section.Path + "\x00" + section.Title + "\x00" + strings.Join(section.HeadingPath, "\x00")
}

func isSupportingCorpusSection(section corpusSection) bool {
	path := strings.ToLower(filepath.ToSlash(section.Path))
	base := filepath.Base(path)
	switch {
	case strings.Contains(path, "05-system-model"),
		strings.Contains(path, "responsability-mapping"),
		strings.Contains(path, "responsibility-mapping"),
		strings.Contains(path, "design-patterns"),
		base == "architecture.md",
		base == "index.md":
		return true
	default:
		return false
	}
}

func extractReviewSignals(req review.Request, diff *review.StructuredDiff, scm *review.SCMContext, skillSections []review.Section) reviewSignals {
	signals := reviewSignals{
		IDs:        make(map[string]struct{}),
		Routes:     make(map[string]struct{}),
		Terms:      make(map[string]struct{}),
		PathParts:  make(map[string]struct{}),
		Components: make(map[string]struct{}),
		Entities:   make(map[string]struct{}),
		Skills:     make(map[string]struct{}),
		CodeRules:  make(map[string]struct{}),
	}
	var textParts []string
	textParts = append(textParts, req.Title, req.Description, req.Repository, req.SourceBranch, req.TargetBranch)
	textParts = append(textParts, req.Labels...)
	textParts = append(textParts, req.ChangedPaths...)
	if scm != nil {
		textParts = append(textParts, scm.Title, scm.Description, scm.Repository, scm.Author)
		textParts = append(textParts, scm.Labels...)
		for _, file := range scm.Files {
			textParts = append(textParts, file.OldPath, file.NewPath, file.Patch)
			addPathSignals(signals, file.OldPath)
			addPathSignals(signals, file.NewPath)
			addEntitySignals(signals, file.Patch)
		}
		for _, commit := range scm.Commits {
			textParts = append(textParts, commit.Title, commit.Message, commit.Author)
			addEntitySignals(signals, commit.Title)
			addEntitySignals(signals, commit.Message)
		}
	}
	for _, skill := range skillSections {
		name := strings.ToLower(strings.TrimSpace(skill.Title))
		if name != "" {
			signals.Skills[name] = struct{}{}
			textParts = append(textParts, name)
		}
	}
	if diff != nil {
		for _, file := range diff.Files {
			textParts = append(textParts, file.Path, file.Patch)
			addPathSignals(signals, file.Path)
			addEntitySignals(signals, file.Patch)
		}
	}
	for _, path := range req.ChangedPaths {
		addPathSignals(signals, path)
	}
	signals.Text = strings.ToLower(strings.Join(textParts, "\n"))
	for _, id := range idPattern.FindAllString(signals.Text, -1) {
		id = strings.ToUpper(id)
		if validCorpusID(id) {
			signals.IDs[id] = struct{}{}
		}
	}
	for _, route := range routePattern.FindAllString(strings.Join(textParts, "\n"), -1) {
		route = normalizeRoute(route)
		if apiRouteSignal(route) {
			signals.Routes[route] = struct{}{}
		}
	}
	for _, term := range tokenizeTerms(strings.Join(textParts, "\n")) {
		if queryTerm(term) {
			signals.Terms[term] = struct{}{}
		}
	}
	return signals
}

func addPathSignals(signals reviewSignals, path string) {
	slashed := strings.ToLower(filepath.ToSlash(path))
	for _, term := range codeZoneTerms(slashed) {
		signals.PathParts[term] = struct{}{}
		signals.Components[term] = struct{}{}
	}
	if rule := codeRuleForPath(slashed); rule != "" {
		signals.CodeRules[rule] = struct{}{}
	}
	parts := strings.FieldsFunc(slashed, func(r rune) bool {
		return r == '/' || r == '-' || r == '_' || r == '.' || r == ' '
	})
	for _, part := range parts {
		part = normalizeTerm(part)
		if part == "" || stopSignal(part) {
			continue
		}
		signals.PathParts[part] = struct{}{}
		if len(part) >= 3 {
			signals.Components[part] = struct{}{}
		}
	}
}

func codeZoneTerms(path string) []string {
	var terms []string
	if rule := codeRuleForPath(path); rule != "" {
		terms = append(terms, rule)
		switch rule {
		case "python":
			terms = append(terms, "py")
		case "go":
			terms = append(terms, "golang")
		case "typescript":
			terms = append(terms, "ts", "tsx")
		case "javascript":
			terms = append(terms, "js", "jsx")
		}
	}
	switch {
	case strings.HasPrefix(path, "services/backend/"), path == "services/backend":
		terms = append(terms, "fastapi")
	case strings.HasPrefix(path, "services/gateway/"), path == "services/gateway":
		terms = append(terms, "gateway", "websocket")
	case strings.HasPrefix(path, "clients/web/"), path == "clients/web":
		terms = append(terms, "nextjs", "composer")
	case strings.HasPrefix(path, "packages/design-tokens/"), path == "packages/design-tokens":
		terms = append(terms, "design", "tokens")
	default:
	}
	return uniqueStrings(terms)
}

func codeRuleForPath(path string) string {
	base := filepath.Base(path)
	switch filepath.Ext(base) {
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".java":
		return "java"
	case ".kt", ".kts":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".rs":
		return "rust"
	case ".cs":
		return "csharp"
	}
	switch base {
	case "go.mod", "go.sum":
		return "go"
	case "pyproject.toml", "setup.py", "requirements.txt", "poetry.lock", "uv.lock":
		return "python"
	case "tsconfig.json", "tsconfig.base.json":
		return "typescript"
	case "package.json":
		return "javascript"
	case "gemfile", "gemfile.lock":
		return "ruby"
	case "cargo.toml", "cargo.lock":
		return "rust"
	case "composer.json", "composer.lock":
		return "php"
	}
	switch {
	case strings.HasPrefix(path, "services/backend/"), path == "services/backend":
		return "python"
	case strings.HasPrefix(path, "services/gateway/"), path == "services/gateway":
		return "go"
	case strings.HasPrefix(path, "clients/web/"), path == "clients/web":
		return "typescript"
	default:
		return ""
	}
}

func addEntitySignals(signals reviewSignals, text string) {
	for _, match := range entityPattern.FindAllStringSubmatch(text, -1) {
		for _, value := range match[1:] {
			value = normalizeTerm(value)
			if value != "" && !stopSignal(value) {
				signals.Entities[value] = struct{}{}
			}
		}
	}
	for _, match := range identifierPattern.FindAllString(text, -1) {
		match = normalizeTerm(match)
		if len(match) >= 4 && !stopSignal(match) {
			signals.Entities[match] = struct{}{}
		}
	}
}

func discoverCorpusDocuments(ctx context.Context, root string) ([]corpusDocument, error) {
	var docs []corpusDocument
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
			case ".git", ".claude", ".venv", ".uv-cache", ".ruff_cache", ".go-cache", ".go-mod-cache", "__pycache__", "node_modules", "vendor", "dist", "build", ".next", ".cache":
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		kind, authority, ok := classifyCorpusDocument(rel)
		if !ok {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxCorpusDocumentBytes {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		docs = append(docs, corpusDocument{
			Path:      rel,
			Kind:      kind,
			Authority: authority,
			Content:   string(data),
		})
		return nil
	})
	return docs, err
}

func classifyCorpusDocument(path string) (review.Kind, string, bool) {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lower)
	ext := filepath.Ext(lower)
	switch {
	case base == "agents.md", base == "claude.md":
		return review.KindRules, "baseline_rules", true
	case strings.Contains(lower, "rules/"), strings.Contains(lower, "/rules"), strings.Contains(lower, "rule"), strings.Contains(lower, "convention"):
		return review.KindRules, "rules", isTextCorpusExt(ext)
	case strings.Contains(lower, "prd"), strings.Contains(lower, "srs"), strings.Contains(lower, "requirement"), strings.Contains(lower, "planning"):
		return review.KindPlanning, "requirements", isTextCorpusExt(ext)
	case strings.Contains(lower, "contract"), strings.Contains(lower, "schema"), strings.Contains(lower, "protobuf"), strings.Contains(lower, "proto"):
		return review.KindContract, "contract", isTextCorpusExt(ext)
	case strings.Contains(lower, "data-model"), strings.Contains(lower, "datamodel"), strings.Contains(lower, "data_model"), strings.Contains(lower, "model"):
		return review.KindContract, "contract", isTextCorpusExt(ext)
	case strings.Contains(lower, "openapi"), strings.Contains(lower, "asyncapi"):
		return review.KindAPI, "api_contract", isTextCorpusExt(ext)
	case strings.Contains(lower, "adr"), strings.Contains(lower, "architecture"), strings.Contains(lower, "design-doc"), strings.Contains(lower, "design_doc"):
		return review.KindArchitecture, "architecture", isTextCorpusExt(ext)
	case strings.Contains(lower, "api"):
		return review.KindAPI, "api_contract", isTextCorpusExt(ext)
	case strings.Contains(lower, "security"), strings.Contains(lower, "threat"):
		return review.KindSecurity, "security", isTextCorpusExt(ext)
	case base == "design.md", strings.Contains(lower, "design-token"), strings.Contains(lower, "design/"), strings.Contains(lower, "tokens"):
		return review.KindDesign, "design", isTextCorpusExt(ext)
	case strings.Contains(lower, "release"), strings.Contains(lower, "runbook"), strings.Contains(lower, "deployment"), strings.Contains(lower, "delivery"):
		return review.KindDelivery, "operations", isTextCorpusExt(ext)
	default:
		return "", "", false
	}
}

func isTextCorpusExt(ext string) bool {
	switch ext {
	case ".md", ".markdown", ".txt", ".yaml", ".yml", ".json", ".proto":
		return true
	default:
		return ext == ""
	}
}

func splitCorpusDocument(doc corpusDocument) []corpusSection {
	ext := strings.ToLower(filepath.Ext(doc.Path))
	lower := strings.ToLower(doc.Path)
	switch {
	case ext == ".md" || ext == ".markdown" || filepath.Base(lower) == "agents.md" || filepath.Base(lower) == "claude.md":
		return splitMarkdownDocument(doc)
	case ext == ".json":
		if sections := splitJSONAPIDocument(doc); len(sections) > 0 {
			return sections
		}
	case ext == ".yaml" || ext == ".yml":
		if sections := splitYAMLAPIDocument(doc); len(sections) > 0 {
			return sections
		}
	}
	return []corpusSection{boundedCorpusSection(doc, filepath.Base(doc.Path), doc.Content)}
}

func splitMarkdownDocument(doc corpusDocument) []corpusSection {
	lines := strings.Split(doc.Content, "\n")
	type mark struct {
		idx  int
		lvl  int
		path []string
	}
	var marks []mark
	var stack []string
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			level := markdownHeadingLevel(trimmed)
			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if title != "" {
				if level <= 0 {
					level = 1
				}
				if len(stack) >= level {
					stack = stack[:level-1]
				}
				for len(stack) < level-1 {
					stack = append(stack, "")
				}
				stack = append(stack, title)
				path := append([]string(nil), stack...)
				marks = append(marks, mark{idx: i, lvl: level, path: path})
			}
		}
	}
	if len(marks) == 0 {
		return []corpusSection{boundedCorpusSection(doc, filepath.Base(doc.Path), doc.Content)}
	}
	var sections []corpusSection
	for i, mark := range marks {
		end := len(lines)
		if i+1 < len(marks) {
			end = marks[i+1].idx
		}
		content := strings.TrimSpace(strings.Join(lines[mark.idx:end], "\n"))
		if content != "" {
			title := mark.path[len(mark.path)-1]
			section := boundedCorpusSection(doc, title, content)
			section.Level = mark.lvl
			section.HeadingPath = append([]string(nil), mark.path...)
			section.Ordinal = len(sections)
			sections = append(sections, section)
		}
	}
	return sections
}

func markdownHeadingLevel(line string) int {
	level := 0
	for _, r := range line {
		if r != '#' {
			break
		}
		level++
	}
	if level > 6 {
		return 0
	}
	return level
}

func splitYAMLAPIDocument(doc corpusDocument) []corpusSection {
	lines := strings.Split(doc.Content, "\n")
	var sections []corpusSection
	for _, parent := range []string{"paths", "channels"} {
		sections = append(sections, yamlChildSections(doc, lines, parent)...)
	}
	if schemaLines := yamlTopLevelBlock(lines, "components"); len(schemaLines) > 0 {
		sections = append(sections, yamlNestedChildSections(doc, schemaLines, "schemas")...)
		sections = append(sections, yamlNestedChildSections(doc, schemaLines, "messages")...)
	}
	return sections
}

func yamlChildSections(doc corpusDocument, lines []string, parent string) []corpusSection {
	block := yamlTopLevelBlock(lines, parent)
	var sections []corpusSection
	for i, line := range block {
		indent := countIndent(line)
		key, ok := yamlKey(line)
		if !ok || indent != 2 {
			continue
		}
		content := collectIndentedBlock(block, i, indent)
		sections = append(sections, boundedCorpusSection(doc, parent+"."+key, strings.Join(content, "\n")))
	}
	return sections
}

func yamlNestedChildSections(doc corpusDocument, lines []string, parent string) []corpusSection {
	var sections []corpusSection
	for i, line := range lines {
		indent := countIndent(line)
		key, ok := yamlKey(line)
		if !ok || key != parent {
			continue
		}
		block := collectIndentedBlock(lines, i, indent)
		for j, child := range block {
			childIndent := countIndent(child)
			childKey, ok := yamlKey(child)
			if !ok || childIndent != indent+2 {
				continue
			}
			content := collectIndentedBlock(block, j, childIndent)
			sections = append(sections, boundedCorpusSection(doc, parent+"."+childKey, strings.Join(content, "\n")))
		}
	}
	return sections
}

func yamlTopLevelBlock(lines []string, key string) []string {
	for i, line := range lines {
		found, ok := yamlKey(line)
		if !ok || countIndent(line) != 0 || found != key {
			continue
		}
		return collectIndentedBlock(lines, i, 0)
	}
	return nil
}

func yamlKey(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	key, _, ok := strings.Cut(trimmed, ":")
	if !ok {
		return "", false
	}
	key = strings.Trim(strings.TrimSpace(key), `"'`)
	if key == "" {
		return "", false
	}
	return key, true
}

func collectIndentedBlock(lines []string, start, indent int) []string {
	var out []string
	for i := start; i < len(lines); i++ {
		line := lines[i]
		if i > start && strings.TrimSpace(line) != "" && countIndent(line) <= indent {
			break
		}
		out = append(out, line)
	}
	return out
}

func countIndent(line string) int {
	count := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}

func splitJSONAPIDocument(doc corpusDocument) []corpusSection {
	var decoded map[string]any
	if err := json.Unmarshal([]byte(doc.Content), &decoded); err != nil {
		return nil
	}
	var sections []corpusSection
	for _, parent := range []string{"paths", "channels"} {
		if children, ok := decoded[parent].(map[string]any); ok {
			keys := sortedAnyKeys(children)
			for _, key := range keys {
				sections = append(sections, boundedCorpusSection(doc, parent+"."+key, marshalCompact(children[key])))
			}
		}
	}
	if components, ok := decoded["components"].(map[string]any); ok {
		for _, parent := range []string{"schemas", "messages"} {
			if children, ok := components[parent].(map[string]any); ok {
				keys := sortedAnyKeys(children)
				for _, key := range keys {
					sections = append(sections, boundedCorpusSection(doc, parent+"."+key, marshalCompact(children[key])))
				}
			}
		}
	}
	return sections
}

func sortedAnyKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func marshalCompact(value any) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func boundedCorpusSection(doc corpusDocument, title, content string) corpusSection {
	content = strings.TrimSpace(content)
	if len(content) > maxCorpusSectionBytes {
		content = content[:maxCorpusSectionBytes] + "\n[truncated]"
	}
	return corpusSection{
		Section: review.Section{
			Path:    doc.Path,
			Title:   strings.TrimSpace(title),
			Content: content,
			Kind:    doc.Kind,
		},
		Authority: doc.Authority,
	}
}

func buildCorpusIndex(sections []corpusSection) corpusIndex {
	index := corpusIndex{
		features: make([]corpusFeature, 0, len(sections)),
		docFreq:  make(map[string]int),
		total:    len(sections),
	}
	for _, section := range sections {
		text := section.Path + "\n" + section.Title + "\n" + section.Content
		feature := corpusFeature{
			section:     section,
			ids:         make(map[string]struct{}),
			routes:      make(map[string]struct{}),
			tokens:      setFromStrings(tokenizeTerms(text)),
			pathTokens:  setFromStrings(tokenizeTerms(section.Path)),
			titleTokens: setFromStrings(tokenizeTerms(section.Title)),
		}
		for _, id := range idPattern.FindAllString(text, -1) {
			id = strings.ToUpper(id)
			if validCorpusID(id) {
				feature.ids[id] = struct{}{}
			}
		}
		for _, route := range routePattern.FindAllString(text, -1) {
			feature.routes[normalizeRoute(route)] = struct{}{}
		}
		for token := range feature.tokens {
			index.docFreq[token]++
		}
		index.features = append(index.features, feature)
	}
	return index
}

func scoreCorpusFeature(feature corpusFeature, signals reviewSignals, index corpusIndex) scoredCorpusSection {
	section := feature.section
	if rule := corpusCodeRule(section.Path); rule != "" && len(signals.CodeRules) > 0 {
		if _, ok := signals.CodeRules[rule]; !ok {
			return scoredCorpusSection{}
		}
	}

	score := 0
	var matched []string
	var reasons []string

	for id := range signals.IDs {
		if _, ok := feature.ids[id]; ok {
			score += 180
			if strings.EqualFold(strings.TrimSpace(section.Title), id) || strings.HasPrefix(strings.ToUpper(section.Title), id+" ") {
				score += 80
			}
			if section.Authority == "contract" || strings.Contains(strings.ToLower(section.Path), "contract") {
				score += 40
			}
			matched = append(matched, id)
			reasons = append(reasons, "identifier "+id)
		}
	}

	for route := range signals.Routes {
		if featureRouteMatches(feature, route) {
			score += 120
			matched = append(matched, route)
			reasons = append(reasons, "API route "+route)
		}
	}

	if rule := corpusCodeRule(section.Path); rule != "" {
		if _, ok := signals.CodeRules[rule]; ok {
			if languageRuleOverview(section) {
				score += 360
			} else {
				score += 55
			}
			matched = append(matched, rule)
			reasons = append(reasons, "changed code language "+rule)
		}
	}

	for part := range signals.PathParts {
		if _, ok := feature.pathTokens[part]; ok {
			score += 20
			matched = append(matched, part)
			reasons = append(reasons, "changed path "+part)
			continue
		}
		if _, ok := feature.titleTokens[part]; ok {
			score += 18
			matched = append(matched, part)
			reasons = append(reasons, "changed path "+part)
			continue
		}
		if _, ok := feature.tokens[part]; ok {
			score += 6
			matched = append(matched, part)
			reasons = append(reasons, "changed path "+part)
		}
	}

	for entity := range signals.Entities {
		if _, ok := feature.tokens[entity]; ok {
			score += 12
			matched = append(matched, entity)
			reasons = append(reasons, "entity "+entity)
		}
	}

	for term := range signals.Terms {
		df := index.docFreq[term]
		if df == 0 || !termSelective(term, df, index.total) {
			continue
		}
		if _, ok := feature.titleTokens[term]; ok {
			score += 35 + rarityBonus(df, index.total)
			matched = append(matched, term)
			reasons = append(reasons, "query term "+term)
			continue
		}
		if _, ok := feature.pathTokens[term]; ok {
			score += 12 + rarityBonus(df, index.total)
			matched = append(matched, term)
			reasons = append(reasons, "query term "+term)
			continue
		}
		if _, ok := feature.tokens[term]; ok {
			score += 4 + rarityBonus(df, index.total)
			matched = append(matched, term)
			reasons = append(reasons, "query term "+term)
		}
	}

	if score > 0 && skillRelevant(section, signals) {
		score += 25
		reasons = append(reasons, "domain skill relevance")
	}
	if score == 0 && section.Authority == "baseline_rules" && len(section.Content) <= 16*1024 {
		score += 5
		reasons = append(reasons, "baseline rules")
	}
	if score > 0 {
		score += authorityScore(section)
	}
	matched = uniqueStrings(matched)
	reasons = uniqueReasonStrings(reasons)
	if score == 0 {
		return scoredCorpusSection{}
	}
	return scoredCorpusSection{
		section:        section,
		score:          score,
		matchedSignals: matched,
		reason:         selectionReason(section, reasons),
	}
}

func featureRouteMatches(feature corpusFeature, route string) bool {
	for candidate := range feature.routes {
		if routeMatches(candidate, route) || routeMatches(route, candidate) {
			return true
		}
	}
	title := strings.ToLower(feature.section.Title)
	return routeMatches(title, route)
}

func languageRuleOverview(section corpusSection) bool {
	title := strings.ToLower(section.Title)
	return section.Level <= 1 ||
		strings.Contains(title, "lois ") ||
		strings.Contains(title, "rules") ||
		strings.Contains(title, "guidelines") ||
		strings.Contains(title, "conventions")
}

func rarityBonus(docFreq, total int) int {
	switch {
	case docFreq <= 1:
		return 18
	case docFreq <= 3:
		return 12
	case total > 0 && docFreq*10 <= total:
		return 8
	default:
		return 0
	}
}

func termSelective(term string, docFreq, total int) bool {
	if len(term) < 4 || docFreq == 0 {
		return false
	}
	if total <= 0 {
		return true
	}
	return docFreq*3 <= total*2
}

func corpusCodeRule(path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lower)
	switch {
	case strings.Contains(lower, "rules/code/python"), strings.Contains(base, "python"):
		return "python"
	case strings.Contains(lower, "rules/code/go"), base == "go.md", strings.Contains(base, "golang"):
		return "go"
	case strings.Contains(lower, "rules/code/typescript"), strings.Contains(base, "typescript"), base == "ts.md":
		return "typescript"
	case strings.Contains(lower, "rules/code/javascript"), strings.Contains(base, "javascript"):
		return "javascript"
	case strings.Contains(lower, "rules/code/kotlin"), strings.Contains(base, "kotlin"):
		return "kotlin"
	case strings.Contains(lower, "rules/code/swift"), strings.Contains(base, "swift"):
		return "swift"
	case strings.Contains(lower, "rules/code/java"), base == "java.md":
		return "java"
	case strings.Contains(lower, "rules/code/ruby"), strings.Contains(base, "ruby"), strings.Contains(base, "rails"):
		return "ruby"
	case strings.Contains(lower, "rules/code/php"), base == "php.md":
		return "php"
	case strings.Contains(lower, "rules/code/rust"), base == "rust.md":
		return "rust"
	case strings.Contains(lower, "rules/code/csharp"), strings.Contains(base, "csharp"), strings.Contains(base, "dotnet"):
		return "csharp"
	default:
		return ""
	}
}

func authorityScore(section corpusSection) int {
	switch section.Authority {
	case "baseline_rules":
		return 12
	case "rules", "contract", "api_contract", "requirements":
		return 8
	case "architecture", "security", "design":
		return 5
	default:
		return 2
	}
}

func skillRelevant(section corpusSection, signals reviewSignals) bool {
	if len(signals.Skills) == 0 {
		return false
	}
	kind := section.Kind
	for skill := range signals.Skills {
		switch {
		case strings.Contains(skill, "api") && (kind == review.KindAPI || kind == review.KindContract):
			return true
		case strings.Contains(skill, "data") && (kind == review.KindContract || strings.Contains(strings.ToLower(section.Path), "data")):
			return true
		case strings.Contains(skill, "security") && kind == review.KindSecurity:
			return true
		case strings.Contains(skill, "design") && kind == review.KindDesign:
			return true
		case strings.Contains(skill, "reliability") && (kind == review.KindArchitecture || kind == review.KindDelivery):
			return true
		case strings.Contains(skill, "framework") && kind == review.KindRules:
			return true
		case strings.Contains(skill, "traceability") && (kind == review.KindPlanning || kind == review.KindContract):
			return true
		}
	}
	return false
}

func selectionReason(section corpusSection, reasons []string) string {
	prefix := section.Authority
	if prefix == "" {
		prefix = string(section.Kind)
	}
	if len(reasons) == 0 {
		return prefix
	}
	if len(reasons) > 4 {
		reasons = reasons[:4]
	}
	return prefix + ": " + strings.Join(reasons, " + ")
}

func buildEvidenceManifest(scored []scoredCorpusSection) []review.EvidenceItem {
	out := make([]review.EvidenceItem, 0, len(scored))
	for _, item := range scored {
		out = append(out, review.EvidenceItem{
			Source:          item.section.Path,
			HeadingOrKey:    item.section.Title,
			Kind:            item.section.Kind,
			Authority:       item.section.Authority,
			MatchedSignals:  append([]string(nil), item.matchedSignals...),
			SelectionReason: item.reason,
			Score:           item.score,
			ContentBytes:    len(item.section.Content),
		})
	}
	return out
}

func routeMatches(text, route string) bool {
	if strings.Contains(text, strings.ToLower(route)) {
		return true
	}
	normalized := strings.ReplaceAll(strings.ToLower(route), "{", ":")
	normalized = strings.ReplaceAll(normalized, "}", "")
	return strings.Contains(text, normalized)
}

func normalizeRoute(route string) string {
	route = strings.Trim(route, "`'\"),.;")
	return strings.TrimSpace(route)
}

func tokenizeTerms(text string) []string {
	raw := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_')
	})
	out := make([]string, 0, len(raw))
	for _, value := range raw {
		value = normalizeTerm(value)
		if value == "" || stopSignal(value) {
			continue
		}
		out = append(out, value)
	}
	return uniqueStrings(out)
}

func setFromStrings(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func queryTerm(value string) bool {
	if len(value) < 3 || stopSignal(value) {
		return false
	}
	switch value {
	case "fix", "fixed", "cleanup", "clean", "follow", "followup", "implementation", "update", "updates", "change", "changes", "small", "review", "merge", "pull", "request", "commit", "commits", "branch", "main", "master", "true", "false", "null", "none", "return", "where", "from", "into", "await", "async":
		return false
	default:
		return true
	}
}

func apiRouteSignal(route string) bool {
	if route == "" || !strings.HasPrefix(route, "/") {
		return false
	}
	if ext := filepath.Ext(route); ext != "" {
		return false
	}
	first := strings.Trim(strings.Split(strings.TrimPrefix(route, "/"), "/")[0], "{}:")
	if first == "" {
		return false
	}
	switch first {
	case "api", "v1", "v2", "internal", "auth", "users", "user", "sessions", "session", "messages", "message", "conversations", "conversation", "groups", "media", "organizations", "orgs", "admin", "reports", "calls", "profile", "onboarding":
		return true
	default:
		return strings.Contains(route, "{") || strings.Contains(route, ":")
	}
}

func normalizeTerm(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.Trim(value, "`'\"()[]{}:;,")
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func stopSignal(value string) bool {
	switch value {
	case "", "go", "js", "ts", "tsx", "jsx", "py", "md", "yml", "yaml", "json", "new", "old", "src", "app", "cmd", "lib", "test", "tests", "spec", "docs", "services", "clients", "backend", "frontend":
		return true
	default:
		return len(value) < 2
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func uniqueReasonStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

var (
	idPattern         = regexp.MustCompile(`(?i)\b[A-Z][A-Z0-9]{1,8}-[A-Z0-9._-]+\b`)
	routePattern      = regexp.MustCompile(`/[A-Za-z0-9_./{}:-]+`)
	identifierPattern = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9_]{3,}\b`)
	entityPattern     = regexp.MustCompile(`(?i)\b(?:table|entity|model|schema|collection|topic|channel|queue)\s+["'\` + "`" + `]?([A-Za-z][A-Za-z0-9_./-]+)|\b(?:from|into|update|join)\s+["'\` + "`" + `]?([A-Za-z][A-Za-z0-9_./-]+)`)
)
