package scm

// DiffRefs identifies the Git refs used to compute a merge or pull request diff.
type DiffRefs struct {
	BaseSHA  string
	HeadSHA  string
	StartSHA string
}
