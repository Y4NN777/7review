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

type ConsoleWorkspace struct {
	View             ConsoleView
	RunID            string
	Input            string
	Help             bool
	Running          bool
	Status           string
	Transcript       []ConsoleTranscriptItem
	TranscriptScroll int
	Height           int
}

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

	header := renderConsoleHeader(view.Plain)
	rail := renderConsoleRail(view)
	main := renderWorkspaceMain(workspace, 82)
	body := joinColumns(main, rail, 2)
	composer := renderWorkspaceComposer(workspace, 118)
	if workspace.Height > 0 {
		reserved := lineCount(header) + lineCount(composer) + 1
		body = trimLines(body, workspace.Height-reserved)
	}
	if view.Plain {
		return header + "\n" + body + "\n" + composer
	}
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(lipgloss.Color("#D0D0D0")).
		Render(header + "\n" + body + "\n" + composer)
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
	lines := []string{
		"",
		centerText("7review", 82),
		centerText("review agent operator console", 82),
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
	if transcriptHeight < 6 {
		transcriptHeight = 6
	}
	if transcriptHeight > 24 {
		transcriptHeight = 24
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
	lines = append(lines, "", transcriptTitle(workspace, transcriptHeight))
	if len(workspace.Transcript) == 0 {
		lines = append(lines,
			"No messages yet.",
			"Type /help, /sessions, /status, /history, /diff, /draft, or ask about the active review.",
		)
	} else {
		lines = append(lines, visibleTranscriptLines(workspace.Transcript, width, transcriptHeight, workspace.TranscriptScroll)...)
	}
	return renderConsoleSurface(lines, width, view.Plain)
}

func transcriptTitle(workspace ConsoleWorkspace, height int) string {
	total := len(renderTranscriptLines(workspace.Transcript, 80))
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
	for _, item := range items {
		role := strings.ToLower(strings.TrimSpace(item.Role))
		if role == "" {
			role = "agent"
		}
		for i, line := range wrappedLines(item.Text, width-8) {
			if i == 0 {
				lines = append(lines, transcriptPrefix(role)+line)
			} else {
				lines = append(lines, "       "+line)
			}
		}
	}
	return lines
}

func transcriptPrefix(role string) string {
	label := fmt.Sprintf("%-7s", role+">")
	switch role {
	case "you", "user", "engineer":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#58A6FF")).Bold(true).Render(label)
	case "agent":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#00E676")).Bold(true).Render(label)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB800")).Bold(true).Render(label)
	}
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
		input = "/help"
	}
	lines := []string{
		"state " + state,
		"> " + input,
		"enter send  up/down scroll  pgup/pgdown page  r refresh  ? help  q exit",
	}
	if workspace.Help {
		lines = append(lines,
			"/status  /sessions  /run  /history  /history chat_message 20",
			"/diff  /draft  /memory  /approve --report-file final.md  /publish-final --report-file final.md",
		)
	}
	return renderCommandSurface(lines, width)
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
	state := "ATTENTION"
	if view.Ready {
		state = "READY"
	}
	title := "No active run"
	if view.ActiveRun != nil {
		title = firstNonEmpty(view.ActiveRun.Title, view.ActiveRun.ID)
	}
	lines := []string{
		trimTo(title, 28),
		"",
		"Context",
		"server " + trimTo(view.Server, 24),
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
		lines = append(lines, fmt.Sprintf("%-10s %s", trimTo(provider.Name, 10), status))
	}
	if len(view.Roles) > 0 {
		lines = append(lines, "", "Roles")
		for _, role := range view.Roles {
			lines = append(lines, fmt.Sprintf("%-10s %s", trimTo(role.Role, 10), trimTo(role.Primary, 22)))
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
			lines = append(lines, fmt.Sprintf("%-22s %s", trimTo(skill.Name, 22), status))
		}
	}
	return renderConsoleSurface(lines, 34, view.Plain)
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
		if line != stripANSINoise(line) && len(stripANSINoise(line)) <= width {
			lines = append(lines, line+strings.Repeat(" ", width-len(stripANSINoise(line))))
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

func renderCommandSurface(body []string, width int) string {
	if width < 40 {
		width = 40
	}
	var lines []string
	for _, line := range body {
		lines = append(lines, padRight(trimTo(stripANSINoise(line), width), width))
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D0D7DE")).
		Background(lipgloss.Color("#050505")).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#4AA3FF")).
		Padding(0, 1).
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
	spacer := strings.Repeat(" ", gap)
	for i := 0; i < height; i++ {
		out = append(out, padRight(leftLines[i], leftWidth)+spacer+rightLines[i])
	}
	return strings.Join(out, "\n")
}

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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
