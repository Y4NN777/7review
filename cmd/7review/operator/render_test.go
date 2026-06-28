package operator

import (
	"strings"
	"testing"
)

func TestRenderSelectedContextSummaryShowsTraceReasons(t *testing.T) {
	out := RenderSelectedContextSummary(SelectedContext{
		Run: "owner/repo!7",
		Evidence: []ContextEvidence{{
			Source:          "docs/openapi.yaml",
			HeadingOrKey:    "schemas.Session",
			Kind:            "interface",
			Authority:       "contract",
			MatchedSignals:  []string{"/sessions", "Session"},
			SelectionReason: "interface_trace: /sessions -> schemas.Session",
			Score:           870,
		}},
		SkillSections: []ContextSection{{
			Path:            "agent/skills/api-contract-review/SKILL.md",
			Title:           "API contract review",
			SelectionReason: "language Go",
		}},
	})

	for _, want := range []string{
		"context owner/repo!7",
		"corpus 0 evidence 1 skills 1",
		"docs/openapi.yaml#schemas.Session",
		"interface_trace: /sessions -> schemas.Session",
		"signals /sessions, Session",
		"agent/skills/api-contract-review/SKILL.md#API contract review",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("selected context render missing %q:\n%s", want, out)
		}
	}
}

func TestRenderDiffSummaryShowsChangedFiles(t *testing.T) {
	out := RenderDiffSummary(DiffSummary{
		Run:         "owner/repo!7",
		FileCount:   2,
		TotalTokens: 123,
		Additions:   10,
		Deletions:   3,
		ChangedFiles: []ChangedFile{{
			Path:      "api/orders.go",
			Status:    "modified",
			Additions: 8,
			Deletions: 2,
			HasPatch:  true,
		}, {
			OldPath:   "old.go",
			Path:      "new.go",
			Status:    "renamed",
			Additions: 2,
			Deletions: 1,
		}},
	})

	for _, want := range []string{"diff owner/repo!7", "files 2 tokens 123 +10 -3", "api/orders.go", "old.go -> new.go", "no-patch"} {
		if !strings.Contains(out, want) {
			t.Fatalf("diff render missing %q:\n%s", want, out)
		}
	}
}
