package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type DependencyStatus struct {
	Name   string
	URL    string
	Ready  bool
	Detail string
}

type ConfigItem struct {
	Key   string
	Value string
}

type StatusView struct {
	Title        string
	Config       []ConfigItem
	Dependencies []DependencyStatus
	Plain        bool
}

func RenderStatus(view StatusView) string {
	title := view.Title
	if title == "" {
		title = "7review status"
	}
	deps := append([]DependencyStatus(nil), view.Dependencies...)
	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })

	var lines []string
	lines = append(lines, renderDashboardTitle(title, overallReady(deps), view.Plain))
	if len(view.Config) > 0 {
		lines = append(lines, renderPanel("CONFIG", renderConfigLines(view.Config), true, view.Plain))
	}
	if len(deps) == 0 {
		return strings.Join(lines, "\n")
	}
	var runtime []string
	for _, dep := range deps {
		state := "ok"
		if !dep.Ready {
			state = "down"
		}
		line := fmt.Sprintf("%-10s %-5s %s", dep.Name, state, dep.URL)
		if dep.Detail != "" {
			line += " - " + dep.Detail
		}
		runtime = append(runtime, line)
	}
	lines = append(lines, renderPanel("RUNTIME", runtime, overallReady(deps), view.Plain))
	return strings.Join(lines, "\n")
}

func renderDashboardTitle(title string, ready bool, plain bool) string {
	state := "READY"
	if !ready {
		state = "ATTENTION"
	}
	line := title + " | " + state
	if plain {
		return line
	}
	color := lipgloss.Color("42")
	if !ready {
		color = lipgloss.Color("196")
	}
	return lipgloss.NewStyle().Bold(true).Foreground(color).Render(line)
}

func renderPanel(title string, body []string, ready bool, plain bool) string {
	if len(body) == 0 {
		body = []string{"none"}
	}
	width := len(title) + 8
	for _, line := range body {
		plainLine := stripANSINoise(line)
		if len(plainLine) > width {
			width = len(plainLine)
		}
	}
	if width < 58 {
		width = 58
	}
	top := "+-- " + title + " " + strings.Repeat("-", width-len(title)-5) + "+"
	bottom := "+" + strings.Repeat("-", width) + "+"
	var lines []string
	lines = append(lines, top)
	for _, line := range body {
		lines = append(lines, "| "+padRight(stripANSINoise(line), width-2)+" |")
	}
	lines = append(lines, bottom)
	out := strings.Join(lines, "\n")
	if plain {
		return out
	}
	color := lipgloss.Color("39")
	if !ready {
		color = lipgloss.Color("196")
	}
	return lipgloss.NewStyle().Foreground(color).Render(out)
}

func renderLine(line string, ready bool, plain bool) string {
	if plain {
		return line
	}
	color := lipgloss.Color("42")
	if !ready {
		color = lipgloss.Color("196")
	}
	return lipgloss.NewStyle().Foreground(color).Render(line)
}

func renderConfigLines(items []ConfigItem) []string {
	items = append([]ConfigItem(nil), items...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	lines := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item.Value)
		if value == "" {
			value = "unset"
		}
		lines = append(lines, fmt.Sprintf("%-16s %s", item.Key, value))
	}
	return lines
}

func overallReady(deps []DependencyStatus) bool {
	if len(deps) == 0 {
		return true
	}
	for _, dep := range deps {
		if !dep.Ready {
			return false
		}
	}
	return true
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

func stripANSINoise(value string) string {
	// renderPanel is ASCII-first; nested colored lines are reduced before padding.
	var b strings.Builder
	inEscape := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if inEscape {
			if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
				inEscape = false
			}
			continue
		}
		if ch == 0x1b {
			inEscape = true
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}
