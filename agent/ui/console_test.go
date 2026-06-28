package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
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
	for _, want := range []string{"7review", "Review workspace", "run    owner/repo!7", "Transcript 4 lines", "you>", "/sessions", "agent>", "sessions 1", "> /history", "/ commands", "/history chat_message 20"} {
		if !strings.Contains(out, want) {
			t.Fatalf("workspace output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "chat: 7review chat") {
		t.Fatalf("interactive workspace should not push users to a separate chat command:\n%s", out)
	}
}

func TestRenderConsoleWorkspaceIdleComposerUsesChatHint(t *testing.T) {
	out := RenderConsoleWorkspace(ConsoleWorkspace{
		View:   ConsoleView{Server: "http://agent", Ready: true, Plain: true},
		Status: "ready",
		Width:  80,
		Height: 20,
	})
	if !strings.Contains(out, "message or / command") || strings.Contains(out, "> /help") {
		t.Fatalf("idle composer should show a neutral chat hint:\n%s", out)
	}
}

func TestRenderConsoleWorkspaceIdleWideKeepsRailWithoutStrayBodyEllipsis(t *testing.T) {
	out := RenderConsoleWorkspace(ConsoleWorkspace{
		View: ConsoleView{
			Server: "http://agent",
			Ready:  true,
			Plain:  true,
			Queue:  QueueView{Depth: 0, Capacity: 32},
			Dependencies: []DependencyStatus{
				{Name: "agent", Ready: true},
				{Name: "queue", Ready: true},
				{Name: "run_store", Ready: true},
				{Name: "headroom", Ready: true},
				{Name: "mempalace", Ready: true},
				{Name: "orchestrator", Ready: true},
				{Name: "pipeline", Ready: true},
			},
			Providers: []ProviderRow{
				{Name: "anthropic"},
				{Name: "openai"},
			},
		},
		Status: "updated",
		Width:  188,
		Height: 44,
	})
	for _, want := range []string{"No active run", "Context", "Runtime", "Providers", "message or / command"} {
		if !strings.Contains(out, want) {
			t.Fatalf("idle wide workspace missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\n...\n┌") || strings.Contains(out, "\n...  ") {
		t.Fatalf("idle wide workspace should not leave a stray body ellipsis before composer:\n%s", out)
	}
}

func TestRenderConsoleWorkspaceInteractivePaintsWholeFrame(t *testing.T) {
	out := RenderConsoleWorkspace(ConsoleWorkspace{
		View:   ConsoleView{Server: "http://agent", Ready: true},
		Status: "updated",
		Width:  140,
		Height: 28,
	})
	if !strings.Contains(out, ansiTrueBlackBG) || !strings.Contains(out, "\x1b[K") {
		t.Fatalf("interactive workspace should paint and clear every frame line, got %q", out)
	}
	if got := lineCount(out); got < 28 {
		t.Fatalf("interactive workspace should fill terminal height, got %d lines:\n%q", got, out)
	}
	for i, line := range strings.Split(out, "\n") {
		if width := lipgloss.Width(stripANSINoise(line)); width < 140 {
			t.Fatalf("interactive workspace line %d should be padded to terminal width, got %d: %q", i, width, line)
		}
	}
}

func TestPaintWorkspaceFrameReassertsBlackBeforePadding(t *testing.T) {
	out := paintWorkspaceFrame("\x1b[1m7review\x1b[0m", 20, 1)
	want := "\x1b[0m" + ansiTrueBlackBG + strings.Repeat(" ", 13)
	if !strings.Contains(out, want) {
		t.Fatalf("styled line padding should repaint black after ANSI reset:\n%q", out)
	}
	if strings.Contains(out, "\x1b[40m") {
		t.Fatalf("workspace frame should use truecolor black, not terminal palette black:\n%q", out)
	}
}

func TestRenderConsoleWorkspaceShowsTranscriptScrollPosition(t *testing.T) {
	var transcript []ConsoleTranscriptItem
	for i := 1; i <= 20; i++ {
		transcript = append(transcript, ConsoleTranscriptItem{Role: "agent", Text: fmt.Sprintf("line %02d", i)})
	}
	out := RenderConsoleWorkspace(ConsoleWorkspace{
		View:             ConsoleView{Server: "http://agent", Ready: true, Plain: true},
		Transcript:       transcript,
		Height:           30,
		TranscriptScroll: 2,
	})
	for _, want := range []string{"Transcript", "scroll 2/", "agent>", "line 13"} {
		if !strings.Contains(out, want) {
			t.Fatalf("workspace output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "line 20") {
		t.Fatalf("scrolled transcript should not show latest line:\n%s", out)
	}
}

func TestRenderConsoleWorkspaceChatKeepsCompactRailAndTurnSpacing(t *testing.T) {
	out := RenderConsoleWorkspace(ConsoleWorkspace{
		View: ConsoleView{
			Server: "http://agent",
			Ready:  true,
			Plain:  true,
			Queue:  QueueView{Depth: 1, Capacity: 32},
			Dependencies: []DependencyStatus{
				{Name: "agent", Ready: true},
				{Name: "headroom", Ready: true},
				{Name: "mempalace", Ready: true},
			},
		},
		Width:   132,
		Height:  32,
		Status:  "running",
		Running: true,
		Transcript: []ConsoleTranscriptItem{
			{Role: "you", Text: "what can you do?"},
			{Role: "agent", Text: "I can chat through the configured model and help operate 7review."},
		},
	})
	for _, want := range []string{"Review workspace", "run    none selected", "sessions none", "you>", "agent>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("chat workspace missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Providers\n") || strings.Contains(out, "Catalog\n") {
		t.Fatalf("chat workspace should keep compact rail instead of full dashboard rail:\n%s", out)
	}
	lines := renderTranscriptLines([]ConsoleTranscriptItem{
		{Role: "you", Text: "what can you do?"},
		{Role: "agent", Text: "I can chat through the configured model."},
	}, 80)
	if len(lines) < 3 || strings.TrimSpace(lines[1]) != "" {
		t.Fatalf("chat turns should be separated by a blank transcript row: %#v", lines)
	}
}

func TestRenderConsoleWorkspaceStreamingKeepsLatestAgentTextVisible(t *testing.T) {
	var transcript []ConsoleTranscriptItem
	for i := 1; i <= 8; i++ {
		transcript = append(transcript, ConsoleTranscriptItem{Role: "you", Text: fmt.Sprintf("prompt %02d", i)})
		transcript = append(transcript, ConsoleTranscriptItem{Role: "agent", Text: fmt.Sprintf("reply %02d", i)})
	}
	transcript = append(transcript,
		ConsoleTranscriptItem{Role: "you", Text: "latest prompt"},
		ConsoleTranscriptItem{Role: "agent", Text: "streaming latest response token"},
	)
	out := RenderConsoleWorkspace(ConsoleWorkspace{
		View: ConsoleView{
			Server: "http://agent",
			Ready:  true,
			Plain:  true,
		},
		Width:      96,
		Height:     16,
		Status:     "running",
		Running:    true,
		Transcript: transcript,
	})
	for _, want := range []string{"latest prompt", "streaming latest response token", "state running"} {
		if !strings.Contains(out, want) {
			t.Fatalf("streaming workspace should keep latest chat visible, missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "reply 01") {
		t.Fatalf("short streaming workspace should trim old transcript before latest output:\n%s", out)
	}
}

func TestRenderConsoleSurfaceDoesNotPadANSITranscriptRowsIntoBars(t *testing.T) {
	line := transcriptPrefix("you") + "Hello bro"
	out := renderConsoleSurface([]string{line}, 120, false)
	stripped := stripANSINoise(out)
	if strings.Contains(stripped, "Hello bro"+strings.Repeat(" ", 20)) {
		t.Fatalf("ANSI transcript row should not be padded into a full-width band:\n%q", stripped)
	}
}

func TestJoinColumnsStylesPaddingGap(t *testing.T) {
	out := joinColumns("left", "right", 2)
	if out == "left  right" || !strings.Contains(out, "\x1b[") {
		t.Fatalf("column gap/padding should be styled, got %q", out)
	}
}

func TestRenderConsoleWorkspaceRendersPaletteAndErrorRole(t *testing.T) {
	out := RenderConsoleWorkspace(ConsoleWorkspace{
		View:   ConsoleView{Server: "http://agent", Ready: true, Plain: true},
		Input:  "/h",
		Status: "ready",
		Width:  92,
		Height: 28,
		Transcript: []ConsoleTranscriptItem{
			{Role: "you", Text: "/bad"},
			{Role: "error", Text: "unknown command"},
		},
		Palette: []ConsolePaletteRow{
			{Label: "/help", Usage: "/help", Description: "Show slash commands.", Match: []int{1}},
			{Label: "/history", Usage: "/history [type] [limit]", Description: "Show timeline.", Disabled: true, Annotation: "needs run", Match: []int{1}},
		},
	})
	for _, want := range []string{"Commands", "/help", "/history", "needs run", "error>", "unknown command", "> /h"} {
		if !strings.Contains(out, want) {
			t.Fatalf("workspace output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderConsoleWorkspaceNarrowLayoutDoesNotUseFixedWideComposer(t *testing.T) {
	out := RenderConsoleWorkspace(ConsoleWorkspace{
		View: ConsoleView{
			Server: "http://agent",
			Ready:  true,
			Plain:  true,
			ActiveRun: &RunDetail{
				RunRow:     RunRow{ID: "owner/repo!7", Status: "drafted"},
				DraftBytes: 20,
			},
			Providers: []ProviderRow{{Name: "openrouter", Configured: true}},
		},
		RunID:  "owner/repo!7",
		Input:  "/history",
		Status: "ready",
		Width:  64,
		Height: 26,
		Transcript: []ConsoleTranscriptItem{
			{Role: "agent", Text: strings.Repeat("narrow transcript text ", 4)},
		},
	})
	for _, line := range strings.Split(out, "\n") {
		if lipgloss.Width(stripANSINoise(line)) > 70 {
			t.Fatalf("narrow workspace line overflowed: width=%d line=%q\n%s", lipgloss.Width(stripANSINoise(line)), line, out)
		}
	}
	for _, want := range []string{"Review workspace", "agent>", "> /history"} {
		if !strings.Contains(out, want) {
			t.Fatalf("narrow workspace missing %q:\n%s", want, out)
		}
	}
}

func TestRenderWorkspaceComposerFitsOuterWidth(t *testing.T) {
	out := renderWorkspaceComposer(ConsoleWorkspace{
		Status: "last error: " + strings.Repeat("unauthorized ", 12),
	}, 80)
	for _, line := range strings.Split(out, "\n") {
		if width := lipgloss.Width(stripANSINoise(line)); width > 80 {
			t.Fatalf("composer line overflowed outer width: width=%d line=%q\n%s", width, line, out)
		}
	}
}
