
package skills

// Loader loads repository-specific review skills from disk.
type Loader struct {
	SkillsDir string
}

// Load initializes available skills.
func (s *Loader) Load() error {
	return nil
}
