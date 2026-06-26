package policy

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Y4NN777/7review/agent/review"
)

// Decision records deterministic scope decisions before model review.
type Decision struct {
	SkippedPaths []string
	ReviewPaths  []string
}

// Filter decides which changed paths should reach expensive review steps.
type Filter interface {
	Apply(context.Context, *review.Context) (Decision, error)
}

// DefaultFilter skips common generated, vendored, and dependency-lock files.
type DefaultFilter struct{}

func (DefaultFilter) Apply(_ context.Context, rc *review.Context) (Decision, error) {
	var decision Decision
	for _, path := range rc.Request.ChangedPaths {
		if shouldSkip(path) {
			decision.SkippedPaths = append(decision.SkippedPaths, path)
			continue
		}
		decision.ReviewPaths = append(decision.ReviewPaths, path)
	}
	return decision, nil
}

func shouldSkip(path string) bool {
	clean := filepath.ToSlash(path)
	if strings.Contains(clean, "/vendor/") || strings.Contains(clean, "/node_modules/") {
		return true
	}
	switch filepath.Base(clean) {
	case "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "go.sum":
		return true
	}
	return strings.HasSuffix(clean, ".generated.go") || strings.HasSuffix(clean, ".min.js")
}
