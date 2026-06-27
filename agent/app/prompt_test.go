package app

import (
	"strings"
	"testing"

	"github.com/Y4NN777/7review/agent/pipeline"
	"github.com/Y4NN777/7review/agent/review"
)

func TestReviewChatSystemPromptGroundsRunInteraction(t *testing.T) {
	prompt := reviewChatSystemPrompt(pipeline.Run{
		ID:      "project!7",
		Status:  pipeline.StatusDrafted,
		WebURL:  "https://gitlab.example.com/project/-/merge_requests/7",
		Request: review.Request{Provider: "gitlab", ProjectID: "project", ChangeID: "7"},
		Findings: []review.Finding{{
			ID:       "F1",
			Severity: review.SeverityHigh,
			Title:    "Missing authorization check",
			Location: review.Location{Path: "api/orders.go", Line: 42},
		}},
		DraftReport: "draft report",
	})
	for _, want := range []string{
		"Use only the run facts provided below",
		"Do not invent files, findings, approvals",
		"Always distinguish known facts from assumptions.",
		"risk, evidence from the stored finding/report",
		"When HIL is not approved",
		"Run ID: project!7",
		"F1 high: Missing authorization check",
		"api/orders.go:42",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
