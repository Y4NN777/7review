package tools

// Config contains shared settings for external service clients.
type Config struct {
	GitLabURL   string
	GitLabToken string
}

// DiffRefs identifies the Git refs used to compute a merge request diff.
type DiffRefs struct {
	BaseSHA  string
	HeadSHA  string
	StartSHA string
}

// ContractSection contains project-specific guidance relevant to a review.
type ContractSection struct {
	Path    string
	Content string
}
