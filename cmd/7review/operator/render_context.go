package operator

import (
	"fmt"
	"strings"
)

func RenderSelectedContextSummary(selected SelectedContext) string {
	run := strings.TrimSpace(selected.Run)
	if run == "" {
		run = "unknown"
	}
	lines := []string{
		"context " + run,
		fmt.Sprintf("corpus %d evidence %d skills %d", len(selected.CorpusSections), len(selected.Evidence), len(selected.SkillSections)),
	}
	if len(selected.Evidence) > 0 {
		lines = append(lines, "evidence")
		for _, item := range first(selected.Evidence, 8) {
			ref := ContextReference(item.Source, item.HeadingOrKey)
			authority := firstNonEmpty(item.Authority, "unknown")
			kind := firstNonEmpty(item.Kind, "section")
			lines = append(lines, fmt.Sprintf("%4d %-12s %-10s %s", item.Score, trimLine(authority, 12), trimLine(kind, 10), trimLine(ref, 80)))
			if item.SelectionReason != "" {
				lines = append(lines, "  reason "+trimLine(item.SelectionReason, 110))
			}
			if len(item.MatchedSignals) > 0 {
				lines = append(lines, "  signals "+trimLine(strings.Join(first(item.MatchedSignals, 4), ", "), 110))
			}
		}
		if len(selected.Evidence) > 8 {
			lines = append(lines, fmt.Sprintf("%d more evidence items", len(selected.Evidence)-8))
		}
	}
	if len(selected.SkillSections) > 0 {
		lines = append(lines, "skills")
		for _, section := range first(selected.SkillSections, 5) {
			line := ContextReference(section.Path, section.Title)
			if section.SelectionReason != "" {
				line += " reason=" + trimLine(section.SelectionReason, 70)
			}
			lines = append(lines, trimLine(line, 110))
		}
		if len(selected.SkillSections) > 5 {
			lines = append(lines, fmt.Sprintf("%d more skill sections", len(selected.SkillSections)-5))
		}
	}
	if len(selected.Warnings) > 0 {
		lines = append(lines, "warnings")
		for _, warning := range first(selected.Warnings, 4) {
			lines = append(lines, trimLine(warning, 110))
		}
	}
	return strings.Join(lines, "\n")
}

func ContextReference(source, heading string) string {
	source = strings.TrimSpace(source)
	heading = strings.TrimSpace(heading)
	switch {
	case source == "" && heading == "":
		return "unknown"
	case source == "":
		return heading
	case heading == "":
		return source
	default:
		return source + "#" + heading
	}
}
