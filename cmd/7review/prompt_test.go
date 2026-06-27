package main

import (
	"strings"
	"testing"

	"github.com/Y4NN777/7review/agent/config"
)

func TestChatSystemPromptDefinesOperationalContract(t *testing.T) {
	prompt := chatSystemPrompt(&config.Config{
		InstructionsPath: "../../agent/instructions.md",
		HeadroomURL:      "http://headroom:8787",
		MemPalaceURL:     "http://mempalace:8788",
		GitLabURL:        "https://gitlab.example.com",
		GitHubAPIURL:     "https://api.github.com",
	})
	for _, want := range []string{
		"7review Agent Instructions",
		"Always separate known state from assumptions.",
		"Never invent runtime state",
		"Prefer one clear next command",
		"Do not claim final approval",
		"Headroom and MemPalace as required dependencies",
		"REVIEW_API_TOKEN",
		"7review status --server <agent-url>",
		"7review chat --run <run-id> --server <agent-url>",
		"7review approve",
		"7review publish-final",
		"curl <agent-url>/ready",
		"get_run",
		"approve_run",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
