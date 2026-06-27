package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Y4NN777/7review/agent/review"
)

// Loader loads repository-specific review skills from disk.
type Loader struct {
	SkillsDir string
	Skills    []Skill
}

// Skill is metadata and body loaded from a repository-local SKILL.md file.
type Skill struct {
	Name          string
	Description   string
	License       string
	Compatibility string
	AllowedTools  string
	Metadata      map[string]string
	Path          string
	Frontmatter   string
	Body          string
}

// Load initializes available skills.
func (s *Loader) Load() error {
	if s.SkillsDir == "" {
		return nil
	}
	entries, err := os.ReadDir(s.SkillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("skills: read %s: %w", s.SkillsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(s.SkillsDir, entry.Name(), "SKILL.md")
		skill, err := loadSkill(path)
		if err != nil {
			return err
		}
		s.Skills = append(s.Skills, skill)
	}
	return nil
}

// Select returns the review guidance skills relevant to the normalized request.
func (s *Loader) Select(req review.Request) []review.Section {
	if s == nil {
		return nil
	}
	var selected []review.Section
	for _, skill := range s.Skills {
		if skill.appliesTo(req) {
			selected = append(selected, review.Section{
				Path:    skill.Path,
				Title:   skill.Name,
				Content: skill.ActivationContent(),
				Kind:    review.KindRules,
			})
		}
	}
	return selected
}

func loadSkill(path string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, fmt.Errorf("skills: read %s: %w", path, err)
	}
	text := string(data)
	parts := strings.SplitN(text, "---", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[0]) != "" {
		return Skill{}, fmt.Errorf("skills: %s missing YAML frontmatter", path)
	}

	frontmatter := strings.TrimSpace(parts[1])
	fields := parseFrontmatter(frontmatter)
	skill := Skill{
		Name:          fields["name"],
		Description:   fields["description"],
		License:       fields["license"],
		Compatibility: fields["compatibility"],
		AllowedTools:  fields["allowed-tools"],
		Metadata:      parseMetadata(frontmatter),
		Path:          path,
		Frontmatter:   frontmatter,
		Body:          strings.TrimSpace(parts[2]),
	}
	if skill.Name == "" || skill.Description == "" {
		return Skill{}, fmt.Errorf("skills: %s must define name and description", path)
	}
	if err := validateSkill(path, skill); err != nil {
		return Skill{}, err
	}
	return skill, nil
}

func (s Skill) ActivationContent() string {
	if strings.TrimSpace(s.Frontmatter) == "" {
		return s.Body
	}
	return "---\n" + strings.TrimSpace(s.Frontmatter) + "\n---\n\n" + s.Body
}

func parseFrontmatter(frontmatter string) map[string]string {
	fields := make(map[string]string)
	lines := strings.Split(frontmatter, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if isIndented(line) {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if value == "|" || value == "|-" || value == "|+" || value == ">" || value == ">-" || value == ">+" {
			block, next := collectBlockScalar(lines, i+1, strings.HasPrefix(value, ">"))
			fields[key] = block
			i = next - 1
			continue
		}
		fields[key] = unquoteYAML(value)
	}
	return fields
}

func parseMetadata(frontmatter string) map[string]string {
	metadata := make(map[string]string)
	lines := strings.Split(frontmatter, "\n")
	inMetadata := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !isIndented(line) {
			key, value, ok := strings.Cut(trimmed, ":")
			inMetadata = ok && strings.TrimSpace(key) == "metadata" && strings.TrimSpace(value) == ""
			continue
		}
		if !inMetadata {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		metadata[strings.TrimSpace(key)] = unquoteYAML(strings.TrimSpace(value))
	}
	return metadata
}

func collectBlockScalar(lines []string, start int, folded bool) (string, int) {
	var block []string
	i := start
	for ; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			block = append(block, "")
			continue
		}
		if !isIndented(line) {
			break
		}
		block = append(block, strings.TrimSpace(line))
	}
	if folded {
		return strings.TrimSpace(strings.Join(block, " ")), i
	}
	return strings.TrimSpace(strings.Join(block, "\n")), i
}

func isIndented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

func unquoteYAML(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	return strings.Trim(value, `'`)
}

var skillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$|^[a-z0-9]$`)

func validateSkill(path string, skill Skill) error {
	if !skillNamePattern.MatchString(skill.Name) {
		return fmt.Errorf("skills: %s invalid name %q", path, skill.Name)
	}
	if len(skill.Description) > 1024 {
		return fmt.Errorf("skills: %s description exceeds 1024 characters", path)
	}
	dirName := filepath.Base(filepath.Dir(path))
	if dirName != skill.Name {
		return fmt.Errorf("skills: %s name %q must match directory %q", path, skill.Name, dirName)
	}
	if strings.TrimSpace(skill.Body) == "" {
		return fmt.Errorf("skills: %s must include markdown instructions", path)
	}
	return nil
}

func (s Skill) appliesTo(req review.Request) bool {
	name := strings.ToLower(s.Name)

	switch name {
	case "github-merge-api":
		return strings.EqualFold(req.Provider, "github")
	case "gitlab-merge-api":
		return strings.EqualFold(req.Provider, "gitlab")
	}
	if alwaysOnSkill(name) {
		return true
	}
	return s.score(req) >= 2
}

func alwaysOnSkill(name string) bool {
	switch name {
	case "methodology-review", "project-knowledge", "framework-rules-review", "traceability-review":
		return true
	default:
		return false
	}
}

func (s Skill) score(req review.Request) int {
	requestTerms := requestTerms(req)
	if len(requestTerms) == 0 {
		if s.Name == "test-quality-review" && hasReviewableCode(req) {
			return 2
		}
		return 0
	}
	index := skillIndexText(s)
	score := 0
	for _, term := range requestTerms {
		if strings.Contains(index, term) {
			score++
		}
	}
	if s.Name == "test-quality-review" && hasReviewableCode(req) {
		score++
	}
	return score
}

func selectionText(req review.Request) string {
	parts := []string{
		req.Provider,
		req.Title,
		req.Description,
		req.Repository,
		req.SourceBranch,
		req.TargetBranch,
	}
	parts = append(parts, req.Labels...)
	parts = append(parts, req.ChangedPaths...)
	return strings.ToLower(strings.Join(parts, " "))
}

func requestTerms(req review.Request) []string {
	text := selectionText(req)
	candidates := splitTerms(text)
	seen := make(map[string]bool)
	var terms []string
	for _, term := range candidates {
		if ignoreTerm(term) || seen[term] {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
	}
	sort.Strings(terms)
	return terms
}

func skillIndexText(skill Skill) string {
	parts := []string{skill.Name, skill.Description}
	for _, line := range strings.Split(skill.Body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
			parts = append(parts, trimmed)
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

var termSplitPattern = regexp.MustCompile(`[^a-z0-9._/-]+`)

func splitTerms(text string) []string {
	raw := termSplitPattern.Split(strings.ToLower(text), -1)
	var out []string
	for _, item := range raw {
		item = strings.Trim(item, "._/-")
		if item == "" {
			continue
		}
		out = append(out, item)
		out = append(out, splitPathTerms(item)...)
	}
	return out
}

func splitPathTerms(item string) []string {
	var out []string
	for _, sep := range []string{"/", "-", "_", "."} {
		if strings.Contains(item, sep) {
			for _, part := range strings.Split(item, sep) {
				part = strings.TrimSpace(part)
				if part != "" {
					out = append(out, part)
				}
			}
		}
	}
	return out
}

func ignoreTerm(term string) bool {
	if len(term) < 3 {
		return true
	}
	switch term {
	case "the", "and", "for", "with", "from", "this", "that", "change", "update", "review", "agent", "docs", "file", "files":
		return true
	default:
		return false
	}
}

func hasReviewableCode(req review.Request) bool {
	for _, path := range req.ChangedPaths {
		lower := strings.ToLower(filepath.ToSlash(path))
		if strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".txt") {
			continue
		}
		if strings.Contains(lower, "testdata/") {
			continue
		}
		return true
	}
	return false
}
