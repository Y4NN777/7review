package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type ConsoleView struct {
	Server       string
	Ready        bool
	Plain        bool
	Watch        bool
	RefreshedAt  time.Time
	RefreshEvery time.Duration
	Warnings     []string
	Queue        QueueView
	Dependencies []DependencyStatus
	Runs         []RunRow
	ActiveRun    *RunDetail
	Providers    []ProviderRow
	Roles        []RoleRow
	Skills       []SkillRow
	Tools        []ToolRow
}

type QueueView struct {
	Depth     int
	Capacity  int
	Available int
	Enqueued  uint64
	Completed uint64
	Failed    uint64
}

type RunRow struct {
	ID          string
	Provider    string
	ProjectID   string
	ChangeID    string
	Title       string
	Status      string
	Error       string
	WebURL      string
	UpdatedAt   time.Time
	EventCount  int
	HILApproved bool
}

type RunDetail struct {
	RunRow
	Findings    int
	DraftBytes  int
	FinalBytes  int
	ReportReady bool
	EventCount  int
	LatestEvent string
}

type ProviderRow struct {
	Name       string
	Configured bool
	BaseURL    string
	Reason     string
}

type RoleRow struct {
	Role        string
	Primary     string
	Fallbacks   []string
	MaxTokens   int
	Parallel    bool
	MaxParallel int
}

type SkillRow struct {
	Name   string
	Loaded bool
	Path   string
}

type ToolRow struct {
	Name             string `json:"name"`
	LifecycleStage   string `json:"lifecycle_stage"`
	Implemented      bool   `json:"implemented"`
	RequiresApproval bool   `json:"requires_approval"`
}

type ConsoleCommandPanel struct {
	RunID   string
	Input   string
	Help    bool
	Running bool
	Log     []string
}

type ConsoleTranscriptItem struct {
	Role string
	Text string
}

type ConsolePaletteRow struct {
	Label       string
	Usage       string
	Description string
	Annotation  string
	Disabled    bool
	Match       []int
}

type ConsoleWorkspace struct {
	View             ConsoleView
	RunID            string
	Input            string
	Help             bool
	Running          bool
	Status           string
	Transcript       []ConsoleTranscriptItem
	TranscriptScroll int
	Palette          []ConsolePaletteRow
	PaletteSelected  int
	Width            int
	Height           int
	bodyHeight       int
}

const emptyComposerHint = "message or / command"

func RenderConsole(view ConsoleView) string {
	if view.Server == "" {
		view.Server = "http://localhost:8080"
	}
	view.Dependencies = sortedDependencies(view.Dependencies)
	view.Runs = sortedRuns(view.Runs)
	view.Providers = sortedProviders(view.Providers)
	view.Skills = sortedSkills(view.Skills)
	view.Tools = sortedTools(view.Tools)

	header := renderConsoleHeader(view.Plain)
	left := renderConsoleMain(view)
	right := renderConsoleRail(view)
	body := joinColumns(left, right, 2)
	footer := "r refresh  ? help  q/ctrl+c exit  chat: 7review chat --run <run-id> --server " + view.Server
	if view.Watch && view.RefreshEvery > 0 {
		footer = fmt.Sprintf("live refresh %s  r refresh  ? help  q/ctrl+c exit  chat: 7review chat --run <run-id> --server %s", view.RefreshEvery, view.Server)
	}
	if view.Plain {
		return header + "\n" + body + "\n" + footer
	}
	style := lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(lipgloss.Color("#D0D0D0"))
	return style.Render(header + "\n" + body + "\n" + signalStyle().Render(footer))
}

func RenderConsoleWorkspace(workspace ConsoleWorkspace) string {
	view := workspace.View
	if view.Server == "" {
		view.Server = "http://localhost:8080"
	}
	view.Dependencies = sortedDependencies(view.Dependencies)
	view.Runs = sortedRuns(view.Runs)
	view.Providers = sortedProviders(view.Providers)
	view.Skills = sortedSkills(view.Skills)
	view.Tools = sortedTools(view.Tools)

	width := workspace.Width
	if width <= 0 {
		width = 118
	}
	if width < 48 {
		width = 48
	}
	header := renderConsoleHeaderWidth(view.Plain, minInt(width, 82))
	composer := renderWorkspaceComposer(workspace, width)
	bodyHeight := 0
	if workspace.Height > 0 {
		bodyHeight = workspace.Height - lineCount(header) - lineCount(composer) - 1
		if bodyHeight < 1 {
			bodyHeight = 1
		}
		workspace.bodyHeight = bodyHeight
	}
	var body string
	chatActive := workspace.Running || len(workspace.Transcript) > 0
	switch {
	case width >= 118:
		railWidth := 34
		if chatActive {
			railWidth = 28
		}
		mainWidth := width - railWidth - 2
		main := renderWorkspaceMain(workspace, mainWidth)
		rail := renderWorkspaceRail(workspace, railWidth)
		if bodyHeight > 0 {
			main = trimWorkspacePane(main, bodyHeight, chatActive)
			rail = trimLines(rail, bodyHeight)
		} else if chatActive {
			rail = trimLines(rail, lineCount(main))
		}
		body = joinColumns(main, rail, 2)
	case width >= 86:
		railWidth := 28
		mainWidth := width - railWidth - 2
		main := renderWorkspaceMain(workspace, mainWidth)
		rail := renderWorkspaceRail(workspace, railWidth)
		if bodyHeight > 0 {
			main = trimWorkspacePane(main, bodyHeight, chatActive)
			rail = trimLines(rail, bodyHeight)
		} else if chatActive {
			rail = trimLines(rail, lineCount(main))
		}
		body = joinColumns(main, rail, 2)
	default:
		body = renderWorkspaceMain(workspace, width)
		if !chatActive {
			if rail := renderWorkspaceRail(workspace, width); strings.TrimSpace(stripANSINoise(rail)) != "" {
				body += "\n" + rail
			}
		}
	}
	if workspace.Height > 0 {
		if chatActive && lineCount(body) > bodyHeight {
			body = trimChatWorkspaceBody(body, bodyHeight)
		}
		body = fitLines(body, bodyHeight)
	}
	frame := header + "\n" + body + "\n" + composer
	if view.Plain {
		return frame
	}
	return paintWorkspaceFrame(frame, width, workspace.Height)
}

func renderConsoleMain(view ConsoleView) string {
	lines := []string{
		"server " + view.Server,
		"state  " + readyLabel(view.Ready),
		"",
	}
	if len(view.Runs) == 0 {
		lines = append(lines,
			"No review sessions",
			"No runs returned by "+view.Server+"/runs",
			"",
			"Start with one webhook event, or inspect readiness with:",
			"7review status --server "+view.Server,
		)
	} else {
		lines = append(lines, renderActivityLines(view)...)
		if len(view.Runs) > 8 {
			lines = append(lines, fmt.Sprintf("%d more runs", len(view.Runs)-8))
		}
	}
	if view.ActiveRun != nil {
		lines = append(lines, "", "Current run")
		lines = append(lines,
			"run        "+view.ActiveRun.ID,
			"status     "+view.ActiveRun.Status,
			"change     "+firstNonEmpty(view.ActiveRun.ProjectID, "-")+" "+firstNonEmpty(view.ActiveRun.ChangeID, "-"),
			fmt.Sprintf("findings   %d", view.ActiveRun.Findings),
			fmt.Sprintf("history    %d events", view.ActiveRun.EventCount),
			fmt.Sprintf("report     draft=%d bytes final=%d bytes", view.ActiveRun.DraftBytes, view.ActiveRun.FinalBytes),
		)
		if view.ActiveRun.LatestEvent != "" {
			lines = append(lines, "latest     "+view.ActiveRun.LatestEvent)
		}
		if view.ActiveRun.WebURL != "" {
			lines = append(lines, "url        "+view.ActiveRun.WebURL)
		}
		if view.ActiveRun.Error != "" {
			lines = append(lines, "error      "+view.ActiveRun.Error)
		}
		lines = append(lines,
			"",
			"Commands",
			"7review chat --run "+view.ActiveRun.ID+" --server "+view.Server,
			"7review history "+view.ActiveRun.ID+" --server "+view.Server,
			"7review history "+view.ActiveRun.ID+" --type chat_message --limit 20 --server "+view.Server,
		)
	}
	if len(view.Warnings) > 0 {
		lines = append(lines, "", "Warnings")
		for _, warning := range view.Warnings {
			lines = append(lines, trimTo(warning, 78))
		}
	}
	return renderConsoleSurface(lines, 82, view.Plain)
}

func renderConsoleHeader(plain bool) string {
	return renderConsoleHeaderWidth(plain, 82)
}

func renderConsoleHeaderWidth(plain bool, width int) string {
	if width < 24 {
		width = 24
	}
	lines := []string{
		"",
		centerText("7review", width),
		centerText(trimTo("review agent operator console", width), width),
		"",
	}
	if plain {
		return strings.Join(lines, "\n")
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F0F6FC")).
		Background(lipgloss.Color("#000000")).
		Bold(true).
		Render(strings.Join(lines, "\n"))
}

func RenderConsoleCommandPanel(panel ConsoleCommandPanel) string {
	lines := []string{
		"Command",
	}
	if panel.RunID != "" {
		lines = append(lines, "run    "+trimTo(panel.RunID, 70))
	} else {
		lines = append(lines, "run    none selected")
	}
	state := "ready"
	if panel.Running {
		state = "running"
	}
	lines = append(lines,
		"state  "+state,
		"",
		"input  "+firstNonEmpty(panel.Input, "/help"),
	)
	if panel.Help {
		lines = append(lines,
			"",
			"Slash commands",
			"/help",
			"/status",
			"/sessions",
			"/sessions drafted 5",
			"/run",
			"/history",
			"/history chat_message 20",
			"/diff",
			"/draft",
			"/memory",
			"/approve --report-file final.md",
			"/publish-final --report-file final.md",
			"",
			"Type a normal message to chat with the model for the active run.",
		)
	}
	if len(panel.Log) > 0 {
		lines = append(lines, "", "Recent output")
		for _, item := range lastStrings(panel.Log, 4) {
			for _, line := range wrappedLines(item, 86) {
				lines = append(lines, line)
			}
		}
	}
	return renderCommandSurface(lines, 92)
}

func renderWorkspaceMain(workspace ConsoleWorkspace, width int) string {
	view := workspace.View
	runID := firstNonEmpty(workspace.RunID, activeRunID(view))
	transcriptHeight := workspace.Height - 14
	if workspace.bodyHeight > 0 {
		transcriptHeight = workspace.bodyHeight - workspaceSummaryLineCount(workspace)
	}
	if transcriptHeight < 6 {
		transcriptHeight = 6
	}
	if transcriptHeight > 40 {
		transcriptHeight = 40
	}
	lines := []string{"Review workspace"}
	if runID != "" {
		lines = append(lines, "run    "+trimTo(runID, width-7))
	} else {
		lines = append(lines, "run    none selected")
	}
	if view.ActiveRun != nil {
		lines = append(lines,
			"status "+firstNonEmpty(view.ActiveRun.Status, "-"),
			fmt.Sprintf("report draft=%d bytes final=%d bytes", view.ActiveRun.DraftBytes, view.ActiveRun.FinalBytes),
		)
		if view.ActiveRun.LatestEvent != "" {
			lines = append(lines, "latest "+trimTo(view.ActiveRun.LatestEvent, width-7))
		}
	} else if len(view.Runs) == 0 {
		lines = append(lines, "sessions none")
	} else {
		lines = append(lines, fmt.Sprintf("sessions %d", len(view.Runs)))
	}
	if len(view.Warnings) > 0 {
		lines = append(lines, "", "Warnings")
		for _, warning := range view.Warnings {
			lines = append(lines, trimTo(warning, width))
		}
	}
	transcriptWidth := conversationWidth(width)
	lines = append(lines, "", transcriptTitle(workspace, transcriptHeight, transcriptWidth))
	if len(workspace.Transcript) == 0 {
		lines = append(lines,
			"No messages yet.",
			"Type /help, /sessions, /status, /history, /diff, /draft, or ask about the active review.",
		)
	} else {
		lines = append(lines, visibleTranscriptLines(workspace.Transcript, transcriptWidth, transcriptHeight, workspace.TranscriptScroll)...)
	}
	return renderConsoleSurface(lines, width, view.Plain)
}

func workspaceSummaryLineCount(workspace ConsoleWorkspace) int {
	count := 5
	if workspace.View.ActiveRun != nil {
		count += 2
		if workspace.View.ActiveRun.LatestEvent != "" {
			count++
		}
	} else {
		count++
	}
	if len(workspace.View.Warnings) > 0 {
		count += 1 + len(workspace.View.Warnings)
	}
	return count
}

func transcriptTitle(workspace ConsoleWorkspace, height int, width int) string {
	total := len(renderTranscriptLines(workspace.Transcript, width))
	if total == 0 {
		return "Transcript"
	}
	if total <= height {
		return fmt.Sprintf("Transcript %d lines", total)
	}
	scroll := workspace.TranscriptScroll
	maxScroll := total - height
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	start := total - height - scroll + 1
	end := total - scroll
	position := "latest"
	if scroll > 0 {
		position = fmt.Sprintf("scroll %d/%d", scroll, maxScroll)
	}
	return fmt.Sprintf("Transcript %d-%d/%d %s", start, end, total, position)
}

func visibleTranscriptLines(items []ConsoleTranscriptItem, width int, height int, scroll int) []string {
	lines := renderTranscriptLines(items, width)
	if len(lines) <= height {
		return lines
	}
	maxScroll := len(lines) - height
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	start := len(lines) - height - scroll
	return lines[start : start+height]
}

func renderTranscriptLines(items []ConsoleTranscriptItem, width int) []string {
	var lines []string
	for index, item := range items {
		if index > 0 && len(lines) > 0 {
			lines = append(lines, "")
		}
		role := strings.ToLower(strings.TrimSpace(item.Role))
		if role == "" {
			role = "agent"
		}
		text := item.Text
		if strings.TrimSpace(text) == "" && role == "agent" {
			text = "..."
		}
		for i, line := range wrappedLines(text, width-8) {
			if i == 0 {
				lines = append(lines, transcriptPrefix(role)+transcriptText(line))
			} else {
				lines = append(lines, transcriptText("       "+line))
			}
		}
	}
	return lines
}

func conversationWidth(width int) int {
	if width > 104 {
		return 104
	}
	if width < 32 {
		return 32
	}
	return width
}

func transcriptPrefix(role string) string {
	label := fmt.Sprintf("%-7s", role+">")
	base := lipgloss.NewStyle().Background(lipgloss.Color("#000000")).Bold(true)
	switch role {
	case "you", "user", "engineer":
		return base.Foreground(lipgloss.Color("#58A6FF")).Render(label)
	case "agent":
		return base.Foreground(lipgloss.Color("#00E676")).Render(label)
	case "error":
		return base.Foreground(lipgloss.Color("#FF5C57")).Render(label)
	case "system", "status":
		return base.Foreground(lipgloss.Color("#FFB800")).Render(label)
	default:
		return base.Foreground(lipgloss.Color("#FFB800")).Render(label)
	}
}

func transcriptText(text string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C9D1D9")).
		Background(lipgloss.Color("#000000")).
		Render(text)
}

func renderWorkspaceComposer(workspace ConsoleWorkspace, width int) string {
	if width < 48 {
		width = 48
	}
	state := firstNonEmpty(workspace.Status, "ready")
	if workspace.Running {
		state = "running"
	}
	input := workspace.Input
	if strings.TrimSpace(input) == "" {
		input = emptyComposerHint
	}
	prompt := "> "
	if strings.TrimSpace(workspace.Input) == "" {
		prompt = "· "
	}
	lines := []string{
		"state " + state,
		prompt + input,
		"enter send  / commands  up/down scroll  pgup/pgdown page  r refresh  esc clear  q exit",
	}
	if workspace.Help {
		lines = append(lines,
			"/status  /sessions  /run  /history  /history chat_message 20",
			"/diff  /draft  /memory  /approve --report-file final.md  /publish-final --report-file final.md",
		)
	}
	if len(workspace.Palette) > 0 {
		lines = append(renderPaletteLines(workspace.Palette, workspace.PaletteSelected, width-4), lines...)
	}
	return renderCommandSurface(lines, width)
}

func renderPaletteLines(rows []ConsolePaletteRow, selected int, width int) []string {
	if width < 32 {
		width = 32
	}
	limit := len(rows)
	if limit > 8 {
		limit = 8
	}
	lines := []string{"Commands"}
	for i := 0; i < limit; i++ {
		row := rows[i]
		marker := " "
		if i == selected {
			marker = ">"
		}
		label := renderPaletteLabel(row)
		usage := row.Usage
		if usage == "" {
			usage = row.Label
		}
		meta := row.Description
		if row.Annotation != "" {
			meta = firstNonEmpty(meta, row.Annotation) + " [" + row.Annotation + "]"
		}
		plainPrefix := marker + " " + row.Label + "  "
		remaining := width - len(stripANSINoise(plainPrefix))
		if remaining < 10 {
			remaining = 10
		}
		line := fmt.Sprintf("%s %s  %s", marker, label, trimTo(firstNonEmpty(usage, meta), remaining))
		if meta != "" && width-len(stripANSINoise(line))-2 > 12 {
			line += "  " + trimTo(meta, width-len(stripANSINoise(line))-2)
		}
		if row.Disabled {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D8590")).Render(line)
		}
		lines = append(lines, line)
	}
	if len(rows) > limit {
		lines = append(lines, fmt.Sprintf("%d more matches", len(rows)-limit))
	}
	lines = append(lines, "")
	return lines
}

func renderPaletteLabel(row ConsolePaletteRow) string {
	if len(row.Match) == 0 {
		return row.Label
	}
	matched := make(map[int]bool, len(row.Match))
	for _, index := range row.Match {
		matched[index] = true
	}
	var b strings.Builder
	for index, r := range row.Label {
		part := string(r)
		if matched[index] {
			part = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB800")).Bold(true).Render(part)
		}
		b.WriteString(part)
	}
	return b.String()
}

func readyLabel(ready bool) string {
	if ready {
		return "READY"
	}
	return "ATTENTION"
}

func renderActivityLines(view ConsoleView) []string {
	lines := []string{"Activity"}
	for _, run := range firstRuns(view.Runs, 8) {
		status := run.Status
		if run.HILApproved {
			status += " approved"
		}
		marker := " "
		if view.ActiveRun != nil && run.ID == view.ActiveRun.ID {
			marker = ">"
		}
		title := trimTo(firstNonEmpty(run.Title, run.ID), 30)
		updated := formatActivityTime(run.UpdatedAt)
		history := ""
		if run.EventCount > 0 {
			history = fmt.Sprintf(" history=%d", run.EventCount)
		}
		line := fmt.Sprintf("%s %-19s %-9s %-14s %-10s%s %s", marker, trimTo(run.ID, 19), trimTo(run.Provider, 9), trimTo(status, 14), updated, history, title)
		lines = append(lines, line)
	}
	return lines
}

func formatActivityTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.UTC().Format("15:04:05Z")
}

func renderConsoleRail(view ConsoleView) string {
	return renderConsoleRailWidth(view, 34)
}

func renderWorkspaceRail(workspace ConsoleWorkspace, width int) string {
	if workspace.Running || len(workspace.Transcript) > 0 {
		return renderConsoleRailCompact(workspace.View, width)
	}
	return renderConsoleRailWidth(workspace.View, width)
}

func renderConsoleRailCompact(view ConsoleView, width int) string {
	if width < 24 {
		width = 24
	}
	state := "ATTENTION"
	if view.Ready {
		state = "READY"
	}
	title := "Operator chat"
	if view.ActiveRun != nil {
		title = firstNonEmpty(view.ActiveRun.Title, view.ActiveRun.ID)
	}
	lines := []string{
		trimTo(title, width-2),
		"",
		"Status",
		"state  " + state,
		"server " + trimTo(view.Server, width-8),
	}
	if view.Queue.Capacity > 0 {
		lines = append(lines, fmt.Sprintf("queue  %d/%d", view.Queue.Depth, view.Queue.Capacity))
	}
	var readyDeps, totalDeps int
	for _, dep := range view.Dependencies {
		totalDeps++
		if dep.Ready {
			readyDeps++
		}
	}
	if totalDeps > 0 {
		lines = append(lines, fmt.Sprintf("deps   %d/%d", readyDeps, totalDeps))
	}
	if len(view.Providers) > 0 {
		var configured int
		for _, provider := range view.Providers {
			if provider.Configured {
				configured++
			}
		}
		lines = append(lines, fmt.Sprintf("models %d/%d", configured, len(view.Providers)))
	}
	if view.ActiveRun != nil {
		lines = append(lines,
			"",
			"Run",
			"id     "+trimTo(view.ActiveRun.ID, width-7),
			"state  "+trimTo(view.ActiveRun.Status, width-7),
			fmt.Sprintf("draft  %d bytes", view.ActiveRun.DraftBytes),
		)
	} else {
		lines = append(lines, "", "Run", "none selected")
	}
	lines = append(lines, "", "Commands", "/status", "/sessions", "/providers")
	return renderConsoleSurface(lines, width, view.Plain)
}

func renderConsoleRailWidth(view ConsoleView, width int) string {
	if width < 24 {
		width = 24
	}
	state := "ATTENTION"
	if view.Ready {
		state = "READY"
	}
	title := "No active run"
	if view.ActiveRun != nil {
		title = firstNonEmpty(view.ActiveRun.Title, view.ActiveRun.ID)
	}
	lines := []string{
		trimTo(title, width-6),
		"",
		"Context",
		"server " + trimTo(view.Server, width-8),
		"state  " + state,
	}
	if !view.RefreshedAt.IsZero() {
		lines = append(lines, "refreshed "+view.RefreshedAt.UTC().Format(time.RFC3339))
	}
	if view.Watch && view.RefreshEvery > 0 {
		lines = append(lines, "refresh "+view.RefreshEvery.String())
	}
	lines = append(lines, "", "Runtime")
	for _, dep := range view.Dependencies {
		status := "down"
		if dep.Ready {
			status = "ok"
		}
		lines = append(lines, fmt.Sprintf("%-10s %s", trimTo(dep.Name, 10), status))
	}
	if view.Queue.Capacity > 0 {
		lines = append(lines,
			"",
			"Queue",
			fmt.Sprintf("depth     %d/%d", view.Queue.Depth, view.Queue.Capacity),
			fmt.Sprintf("done      %d", view.Queue.Completed),
			fmt.Sprintf("failed    %d", view.Queue.Failed),
		)
	}
	lines = append(lines, "", "Providers")
	for _, provider := range firstProviders(view.Providers, 7) {
		status := "missing"
		if provider.Configured {
			status = "configured"
		}
		lines = append(lines, fmt.Sprintf("%-10s %s", trimTo(provider.Name, 10), trimTo(status, width-13)))
	}
	if len(view.Roles) > 0 {
		lines = append(lines, "", "Roles")
		for _, role := range view.Roles {
			lines = append(lines, fmt.Sprintf("%-10s %s", trimTo(role.Role, 10), trimTo(role.Primary, width-13)))
		}
	}
	if view.ActiveRun != nil {
		lines = append(lines, "", "Review")
		for _, item := range reviewTodoLines(*view.ActiveRun) {
			lines = append(lines, item)
		}
	}
	lines = append(lines,
		"",
		"Catalog",
		fmt.Sprintf("skills    %d", len(view.Skills)),
		fmt.Sprintf("tools     %d", len(view.Tools)),
	)
	if len(view.Skills) > 0 {
		lines = append(lines, "", "Skills")
		for _, skill := range firstSkills(view.Skills, 4) {
			status := "loaded"
			if !skill.Loaded {
				status = "off"
			}
			lines = append(lines, fmt.Sprintf("%-18s %s", trimTo(skill.Name, width-11), status))
		}
	}
	return renderConsoleSurface(lines, width, view.Plain)
}

func reviewTodoLines(run RunDetail) []string {
	items := []string{
		"draft     " + doneOrOpen(run.DraftBytes > 0),
		"findings  " + doneOrOpen(run.Findings > 0),
		"hil       " + doneOrOpen(run.HILApproved),
		"final     " + doneOrOpen(run.FinalBytes > 0),
	}
	return items
}

func doneOrOpen(done bool) string {
	if done {
		return "done"
	}
	return "open"
}

func renderConsoleSurface(body []string, width int, plain bool) string {
	if width < 24 {
		width = 24
	}
	var lines []string
	for _, line := range body {
		stripped := trimTo(stripANSINoise(line), width)
		if isTranscriptRow(stripped) {
			if line != stripANSINoise(line) && len(stripANSINoise(line)) <= width {
				lines = append(lines, line)
			} else {
				lines = append(lines, stripped)
			}
			continue
		}
		if line != stripANSINoise(line) && len(stripANSINoise(line)) <= width {
			lines = append(lines, line)
			continue
		}
		lines = append(lines, padRight(stripped, width))
	}
	out := strings.Join(lines, "\n")
	if plain {
		return out
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C9D1D9")).
		Background(lipgloss.Color("#000000")).
		Render(out)
}

func isTranscriptRow(line string) bool {
	line = strings.TrimLeft(line, " ")
	return strings.HasPrefix(line, "you>") ||
		strings.HasPrefix(line, "user>") ||
		strings.HasPrefix(line, "engineer>") ||
		strings.HasPrefix(line, "agent>") ||
		strings.HasPrefix(line, "error>") ||
		strings.HasPrefix(line, "system>") ||
		strings.HasPrefix(line, "status>")
}

func renderCommandSurface(body []string, width int) string {
	if width < 40 {
		width = 40
	}
	innerWidth := width - 4
	if innerWidth < 36 {
		innerWidth = 36
	}
	var lines []string
	for _, line := range body {
		stripped := trimTo(stripANSINoise(line), innerWidth)
		if line != stripANSINoise(line) && lipgloss.Width(stripANSINoise(line)) <= innerWidth {
			lines = append(lines, line)
			continue
		}
		lines = append(lines, padRight(stripped, innerWidth))
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D0D7DE")).
		Background(lipgloss.Color("#050505")).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#4AA3FF")).
		Padding(0, 1).
		Width(innerWidth + 2).
		Render(strings.Join(lines, "\n"))
}

func joinColumns(left, right string, gap int) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	height := len(leftLines)
	if len(rightLines) > height {
		height = len(rightLines)
	}
	leftWidth := maxLineWidth(leftLines)
	for len(leftLines) < height {
		leftLines = append(leftLines, "")
	}
	for len(rightLines) < height {
		rightLines = append(rightLines, "")
	}
	out := make([]string, 0, height)
	spacer := blackSpace(gap)
	for i := 0; i < height; i++ {
		out = append(out, padRightStyled(leftLines[i], leftWidth)+spacer+rightLines[i])
	}
	return strings.Join(out, "\n")
}

func padRightStyled(value string, width int) string {
	visible := len(stripANSINoise(value))
	if visible >= width {
		return value
	}
	return value + blackSpace(width-visible)
}

func blackSpace(width int) string {
	if width <= 0 {
		return ""
	}
	return ansiTrueBlackBG + strings.Repeat(" ", width) + "\x1b[0m"
}

const ansiTrueBlackBG = "\x1b[48;2;0;0;0m"

func signalStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#58A6FF")).
		Background(lipgloss.Color("#000000"))
}

func centerText(text string, width int) string {
	if len(text) >= width {
		return text
	}
	pad := (width - len(text)) / 2
	return strings.Repeat(" ", pad) + text
}

func maxLineWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		if len(stripANSINoise(line)) > width {
			width = len(stripANSINoise(line))
		}
	}
	return width
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sortedDependencies(items []DependencyStatus) []DependencyStatus {
	out := append([]DependencyStatus(nil), items...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedRuns(items []RunRow) []RunRow {
	out := append([]RunRow(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

func sortedProviders(items []ProviderRow) []ProviderRow {
	out := append([]ProviderRow(nil), items...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedSkills(items []SkillRow) []SkillRow {
	out := append([]SkillRow(nil), items...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedTools(items []ToolRow) []ToolRow {
	out := append([]ToolRow(nil), items...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func firstRuns(items []RunRow, limit int) []RunRow {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func firstProviders(items []ProviderRow, limit int) []ProviderRow {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func firstSkills(items []SkillRow, limit int) []SkillRow {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func trimTo(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func wrappedLines(value string, width int) []string {
	value = strings.TrimSpace(stripANSINoise(value))
	if value == "" {
		return nil
	}
	var out []string
	for _, raw := range strings.Split(value, "\n") {
		line := strings.TrimSpace(raw)
		for len(line) > width {
			out = append(out, line[:width])
			line = strings.TrimSpace(line[width:])
		}
		out = append(out, line)
	}
	return out
}

func lastStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[len(values)-limit:]
}

func lastTranscriptItems(values []ConsoleTranscriptItem, limit int) []ConsoleTranscriptItem {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[len(values)-limit:]
}

func activeRunID(view ConsoleView) string {
	if view.ActiveRun != nil {
		return strings.TrimSpace(view.ActiveRun.ID)
	}
	if len(view.Runs) > 0 {
		return strings.TrimSpace(view.Runs[0].ID)
	}
	return ""
}

func lineCount(value string) int {
	if value == "" {
		return 0
	}
	return len(strings.Split(value, "\n"))
}

func trimLines(value string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(value, "\n")
	if len(lines) <= maxLines {
		return value
	}
	if maxLines == 1 {
		return "..."
	}
	return strings.Join(append(lines[:maxLines-1], "..."), "\n")
}

func trimWorkspacePane(value string, maxLines int, preserveTail bool) string {
	if preserveTail {
		return trimChatWorkspaceBody(value, maxLines)
	}
	return trimLines(value, maxLines)
}

func trimChatWorkspaceBody(value string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(value, "\n")
	if len(lines) <= maxLines {
		return value
	}
	if maxLines <= 4 {
		return strings.Join(lastStrings(lines, maxLines), "\n")
	}
	head := 4
	if maxLines <= 8 {
		head = 2
	}
	if head > maxLines-2 {
		head = maxLines / 2
	}
	tail := maxLines - head - 1
	out := append([]string{}, lines[:head]...)
	out = append(out, "...")
	out = append(out, lines[len(lines)-tail:]...)
	return strings.Join(out, "\n")
}

func fitLines(value string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(value, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func paintWorkspaceFrame(value string, width int, height int) string {
	if width < 1 {
		width = 1
	}
	lines := strings.Split(value, "\n")
	if height > 0 {
		for len(lines) < height {
			lines = append(lines, "")
		}
	}
	for i, line := range lines {
		visible := lipgloss.Width(stripANSINoise(line))
		if visible >= width {
			lines[i] = ansiTrueBlackBG + line + ansiTrueBlackBG + "\x1b[K\x1b[0m"
			continue
		}
		lines[i] = ansiTrueBlackBG + line + ansiTrueBlackBG + strings.Repeat(" ", width-visible) + ansiTrueBlackBG + "\x1b[K\x1b[0m"
	}
	return strings.Join(lines, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
