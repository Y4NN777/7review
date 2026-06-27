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
	for _, want := range []string{"7review", "No runs returned by http://agent/runs", "headroom", "mempalace", "7review chat --run <run-id> --server http://agent"} {
		if !strings.Contains(out, want) {
			t.Fatalf("console output missing %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{"Implementing signup", "pocket", "OpenCode Zen"} {
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
			RunRow:     RunRow{ID: "owner/repo!7", Status: "drafted", ProjectID: "owner/repo", ChangeID: "7"},
			Findings:   2,
			DraftBytes: 144,
		},
		Providers: []ProviderRow{{Name: "openrouter", Configured: true}},
		Roles:     []RoleRow{{Role: "reasoner", Primary: "openrouter/deepseek"}},
		Skills:    []SkillRow{{Name: "traceability-review", Loaded: true}},
		Tools:     []ToolRow{{Name: "list_runs", LifecycleStage: "observe", Implemented: true}},
	})
	for _, want := range []string{"owner/repo!7", "Fix validation", "findings   2", "depth     1/8", "openrouter", "reasoner", "skills    1", "tools     1", "refreshed 2026-06-27T12:01:02Z", "watch every 5s"} {
		if !strings.Contains(out, want) {
			t.Fatalf("console output missing %q:\n%s", want, out)
		}
	}
}
