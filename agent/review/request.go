package review

// Request is the normalized input for one merge request review run.
// Adapters such as GitLab webhooks should convert provider payloads into this
// shape before entering the pipeline.
type Request struct {
	ProjectID    string
	MRIID        int
	SourceSHA    string
	TargetSHA    string
	SourceBranch string
	TargetBranch string
	Author       string
	Labels       []string
	ChangedPaths []string
}
