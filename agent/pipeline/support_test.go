package pipeline

import (
	"context"
	"testing"

	"github.com/Y4NN777/7review/agent/review"
)

func TestPathPolicyFilterUsesProfileIgnorePatterns(t *testing.T) {
	filter := PathPolicyFilter{Ignore: []string{"generated/**", "**/*.snap"}}
	decision, err := filter.Apply(context.Background(), &review.Context{
		Request: review.Request{
			ChangedPaths: []string{
				"generated/client.go",
				"ui/button.snap",
				"agent/app/server.go",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(decision.SkippedPaths) != 2 {
		t.Fatalf("expected profile ignored paths, got %#v", decision)
	}
	if len(decision.ReviewPaths) != 1 || decision.ReviewPaths[0] != "agent/app/server.go" {
		t.Fatalf("expected review path preserved, got %#v", decision)
	}
}
