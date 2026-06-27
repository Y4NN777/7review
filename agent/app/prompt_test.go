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
		Events: []pipeline.RunEvent{
			{Type: "chat_message", Status: pipeline.StatusDrafted, Message: "explain F1", Meta: map[string]string{"role": "engineer"}},
			{Type: "chat_response", Status: pipeline.StatusDrafted, Message: "F1 is about missing auth", Meta: map[string]string{"role": "agent"}},
		},
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
		"Recent run events:",
		"chat_message | drafted | explain F1 | role=engineer",
		"chat_response | drafted | F1 is about missing auth | role=agent",
		"F1 high: Missing authorization check",
		"api/orders.go:42",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestRenderRecentRunEventsKeepsLatestBoundedHistory(t *testing.T) {
	var events []pipeline.RunEvent
	for _, message := range []string{"old", "one", "two"} {
		events = append(events, pipeline.RunEvent{
			Type:    "chat_message",
			Status:  pipeline.StatusDrafted,
			Message: message,
			Meta:    map[string]string{"role": "engineer"},
		})
	}
	out := renderRecentRunEvents(events, 2)
	for _, want := range []string{"one", "two", "role=engineer"} {
		if !strings.Contains(out, want) {
			t.Fatalf("recent events missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "old") {
		t.Fatalf("recent events included old entry:\n%s", out)
	}
}
