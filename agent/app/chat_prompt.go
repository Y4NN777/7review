package app

import (
	"fmt"
	"strings"

	"github.com/Y4NN777/7review/agent/pipeline"
)

func reviewChatSystemPrompt(run pipeline.Run) string {
	var b strings.Builder
	b.WriteString(strings.Join([]string{
		"You are 7review's live review copilot for one concrete PR/MR review run.",
		"You are talking to an engineer during iterative review, before or during HIL.",
		"Use only the run facts provided below plus the engineer's message.",
		"Do not invent files, findings, approvals, CI status, SCM comments, memory writes, or dependency health.",
		"Always distinguish known facts from assumptions.",
		"When discussing a finding, explain: risk, evidence from the stored finding/report, what would prove it false, and the next useful action.",
		"When the engineer asks what to do next, provide one explicit next command or endpoint.",
		"When HIL is not approved, do not say the review is final and do not propose writing memory as complete.",
	}, "\n"))
	b.WriteString("\n\nRun facts:\n")
	fmt.Fprintf(&b, "Run ID: %s\nStatus: %s\nProvider: %s\nProject: %s\nChange: %s\nURL: %s\n",
		run.ID, run.Status, run.Request.Provider, run.Request.ProjectID, run.Request.ChangeID, run.WebURL)
	if run.Error != "" {
		fmt.Fprintf(&b, "Error: %s\n", run.Error)
	}
	if len(run.Findings) > 0 {
		b.WriteString("\nValidated findings:\n")
		for _, finding := range run.Findings {
			fmt.Fprintf(&b, "- %s %s: %s", finding.ID, finding.Severity, finding.Title)
			if finding.Location.Path != "" {
				fmt.Fprintf(&b, " (%s:%d)", finding.Location.Path, finding.Location.Line)
			}
			b.WriteString("\n")
		}
	}
	if history := renderRecentRunEvents(run.Events, 8); history != "" {
		b.WriteString("\nRecent run events:\n")
		b.WriteString(history)
	}
	if run.DraftReport != "" {
		b.WriteString("\nDraft report:\n")
		b.WriteString(run.DraftReport)
	}
	return b.String()
}

func renderRecentRunEvents(events []pipeline.RunEvent, limit int) string {
	if limit <= 0 || len(events) == 0 {
		return ""
	}
	start := len(events) - limit
	if start < 0 {
		start = 0
	}
	var lines []string
	for _, event := range events[start:] {
		eventType := strings.TrimSpace(event.Type)
		if eventType == "" {
			eventType = "event"
		}
		parts := []string{eventType}
		if event.Status != "" {
			parts = append(parts, string(event.Status))
		}
		if message := truncatePromptEventMessage(event.Message); message != "" {
			parts = append(parts, message)
		}
		if role := strings.TrimSpace(event.Meta["role"]); role != "" {
			parts = append(parts, "role="+role)
		}
		lines = append(lines, "- "+strings.Join(parts, " | "))
	}
	return strings.Join(lines, "\n") + "\n"
}

func truncatePromptEventMessage(message string) string {
	message = strings.TrimSpace(message)
	const maxPromptEventMessage = 240
	if len(message) <= maxPromptEventMessage {
		return message
	}
	if maxPromptEventMessage <= 3 {
		return message[:maxPromptEventMessage]
	}
	return message[:maxPromptEventMessage-3] + "..."
}

func truncateEventMessage(message string) string {
	message = strings.TrimSpace(message)
	const maxEventMessageBytes = 2000
	if len(message) <= maxEventMessageBytes {
		return message
	}
	return message[:maxEventMessageBytes] + "..."
}
