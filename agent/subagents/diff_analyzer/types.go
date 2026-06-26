package diff_analyzer

// StructuredDiff is the normalized representation of a merge request diff.
type StructuredDiff struct {
	Files []FileDiff
}

// FileDiff describes one changed file and its estimated review complexity.
type FileDiff struct {
	Path       string
	Patch      string
	TokenCount int
}
