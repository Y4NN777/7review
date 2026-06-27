package ui

import (
	"strings"
	"testing"
	"time"
)

func TestRenderConsoleIdleUsesRealEmptyState(t *testing.T) {
	out := RenderConsole(ConsoleView{
		Server: "http://agent",
		Ready:  true,
		Plain:  true,
		Dependencies: []DependencyStatus{
			{Name: "headroom", Ready: true},
			{Name: "mempalace", Ready: true},
		},
	})
	for _, want := range []string{"7review", "review agent operator console", "state  READY", "No review sessions", "No runs returned by http://agent/runs", "headroom", "mempalace", "chat: 7review chat --run <run-id> --server http://agent"} {
		if !strings.Contains(out, want) {
			t.Fatalf("console output missing %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{"Implementing signup", "pocket", "OpenCode Zen", "tab switch view", "/chat use"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("console fabricated screenshot content %q:\n%s", forbidden, out)
		}
	}
}

func TestRenderConsolePopulatedShowsRunAndRail(t *testing.T) {
	refreshedAt := time.Date(2026, 6, 27, 12, 1, 2, 0, time.UTC)
	out := RenderConsole(ConsoleView{
		Server:       "http://agent",
		Ready:        true,
		Plain:        true,
		Watch:        true,
		RefreshedAt:  refreshedAt,
		RefreshEvery: 5 * time.Second,
		Queue:        QueueView{Depth: 1, Capacity: 8, Completed: 3, Failed: 1},
		Runs: []RunRow{{
			ID:        "owner/repo!7",
			Provider:  "github",
			ProjectID: "owner/repo",
			ChangeID:  "7",
			Title:     "Fix validation",
			Status:    "drafted",
			UpdatedAt: time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
		}},
		ActiveRun: &RunDetail{
			RunRow:      RunRow{ID: "owner/repo!7", Status: "drafted", ProjectID: "owner/repo", ChangeID: "7"},
			Findings:    2,
			DraftBytes:  144,
			EventCount:  3,
			LatestEvent: "status_changed drafted",
		},
		Providers: []ProviderRow{{Name: "openrouter", Configured: true}},
		Roles:     []RoleRow{{Role: "reasoner", Primary: "openrouter/deepseek"}},
		Skills:    []SkillRow{{Name: "traceability-review", Loaded: true}},
		Tools:     []ToolRow{{Name: "list_runs", LifecycleStage: "observe", Implemented: true}},
	})
	for _, want := range []string{"7review", "Activity", "Current run", "owner/repo!7", "Fix validation", "findings   2", "history    3 events", "latest     status_changed drafted", "Commands", "7review chat --run owner/repo!7 --server http://agent", "7review history owner/repo!7 --type chat_message --limit 20 --server http://agent", "Review", "draft     done", "hil       open", "depth     1/8", "openrouter", "reasoner", "skills    1", "tools     1", "refreshed 2026-06-27T12:01:02Z", "live refresh 5s"} {
		if !strings.Contains(out, want) {
			t.Fatalf("console output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "| 7review") || strings.Contains(out, "+---") || strings.Contains(out, "Active run") || strings.Contains(out, "Runs") {
		t.Fatalf("console should not render boxed dashboard or right-rail app heading:\n%s", out)
	}
}

func TestRenderConsoleCommandPanelShowsInputActionsAndOutput(t *testing.T) {
	out := RenderConsoleCommandPanel(ConsoleCommandPanel{
		RunID: "owner/repo!7",
		Input: "/history chat_message 20",
		Help:  true,
		Log: []string{
			"> /status",
			"agent: status ready",
		},
	})
	for _, want := range []string{"Command", "run    owner/repo!7", "input  /history chat_message 20", "Slash commands", "/sessions drafted 5", "/approve --report-file final.md", "Type a normal message to chat with the model for the active run.", "Recent output", "agent: status ready"} {
		if !strings.Contains(out, want) {
			t.Fatalf("command panel missing %q:\n%s", want, out)
		}
	}
}

func TestRenderConsoleWorkspaceShowsTranscriptRailAndComposer(t *testing.T) {
	out := RenderConsoleWorkspace(ConsoleWorkspace{
		View: ConsoleView{
			Server: "http://agent",
			Ready:  true,
			Plain:  true,
			ActiveRun: &RunDetail{
				RunRow:      RunRow{ID: "owner/repo!7", Status: "drafted"},
				DraftBytes:  20,
				FinalBytes:  0,
				LatestEvent: "status_changed drafted",
			},
			Providers: []ProviderRow{{Name: "openrouter", Configured: true}},
		},
		RunID:  "owner/repo!7",
		Input:  "/history",
		Help:   true,
		Status: "ready",
		Height: 24,
		Transcript: []ConsoleTranscriptItem{
			{Role: "you", Text: "/sessions"},
			{Role: "agent", Text: "sessions 1\nowner/repo!7 drafted"},
		},
	})
	for _, want := range []string{"7review", "Review workspace", "run    owner/repo!7", "Transcript", "you>", "/sessions", "agent>", "sessions 1", "Context", "openrouter", "> /history", "/history chat_message 20"} {
		if !strings.Contains(out, want) {
			t.Fatalf("workspace output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "chat: 7review chat") {
		t.Fatalf("interactive workspace should not push users to a separate chat command:\n%s", out)
	}
}
