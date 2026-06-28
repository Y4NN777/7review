package operator

import (
	"fmt"
	"strings"
)

func RenderDiffSummary(summary DiffSummary) string {
	lines := []string{
		"diff " + summary.Run,
		fmt.Sprintf("files %d tokens %d +%d -%d", summary.FileCount, summary.TotalTokens, summary.Additions, summary.Deletions),
	}
	if len(summary.ChangedFiles) > 0 {
		lines = append(lines, "changed")
		for _, file := range first(summary.ChangedFiles, 8) {
			status := strings.TrimSpace(file.Status)
			if status == "" {
				status = "changed"
			}
			patch := "no-patch"
			if file.HasPatch {
				patch = "patch"
			}
			path := strings.TrimSpace(file.Path)
			if file.OldPath != "" && file.OldPath != file.Path {
				path = strings.TrimSpace(file.OldPath) + " -> " + path
			}
			lines = append(lines, fmt.Sprintf("%-9s +%-4d -%-4d %-8s %s", trimLine(status, 9), file.Additions, file.Deletions, patch, trimLine(path, 80)))
		}
		if len(summary.ChangedFiles) > 8 {
			lines = append(lines, fmt.Sprintf("%d more changed files", len(summary.ChangedFiles)-8))
		}
		return strings.Join(lines, "\n")
	}
	if len(summary.Files) > 0 {
		lines = append(lines, "patches")
		for _, file := range first(summary.Files, 8) {
			lines = append(lines, fmt.Sprintf("%-5d tokens %-5d lines %s", file.TokenCount, file.PatchLines, trimLine(file.Path, 80)))
		}
		if len(summary.Files) > 8 {
			lines = append(lines, fmt.Sprintf("%d more patch chunks", len(summary.Files)-8))
		}
	}
	return strings.Join(lines, "\n")
}
