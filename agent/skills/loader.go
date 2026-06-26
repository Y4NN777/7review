package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Loader loads repository-specific review skills from disk.
type Loader struct {
	SkillsDir string
	Skills    []Skill
}

// Skill is metadata and body loaded from a repository-local SKILL.md file.
type Skill struct {
	Name        string
	Description string
	Path        string
	Body        string
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

	skill := Skill{Path: path, Body: strings.TrimSpace(parts[2])}
	for _, line := range strings.Split(parts[1], "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			skill.Name = strings.TrimSpace(value)
		case "description":
			skill.Description = strings.TrimSpace(value)
		}
	}
	if skill.Name == "" || skill.Description == "" {
		return Skill{}, fmt.Errorf("skills: %s must define name and description", path)
	}
	return skill, nil
}
