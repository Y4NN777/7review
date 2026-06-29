package tools

import "strings"

const botMarkerPrefix = "<!-- 7review:bot-report"
const inlineMarkerPrefix = "<!-- 7review:inline-comment"

func reportWithBotMarker(report string, kind string) string {
	report = strings.TrimSpace(stripBotMarkers(report))
	marker := botMarkerPrefix + " kind=" + kind + " -->"
	if report == "" {
		return marker
	}
	return marker + "\n" + report
}

func hasBotMarkerKind(body string, kind string) bool {
	return strings.Contains(body, botMarkerPrefix) && strings.Contains(body, "kind="+kind)
}

func hasLegacyBotMarker(body string) bool {
	return strings.Contains(body, botMarkerPrefix) && !strings.Contains(body, "kind=")
}

func stripBotMarkers(report string) string {
	var lines []string
	for _, line := range strings.Split(report, "\n") {
		if strings.Contains(line, botMarkerPrefix) {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func inlineCommentMarker(findingID string) string {
	findingID = strings.TrimSpace(findingID)
	if findingID == "" {
		findingID = "unknown"
	}
	return inlineMarkerPrefix + " finding=" + findingID + " -->"
}

func inlineCommentBody(commentBody string, findingID string) string {
	commentBody = strings.TrimSpace(commentBody)
	marker := inlineCommentMarker(findingID)
	if commentBody == "" {
		return marker
	}
	return marker + "\n" + commentBody
}

func hasInlineCommentMarker(body string, findingID string) bool {
	return strings.Contains(body, inlineMarkerPrefix) && strings.Contains(body, "finding="+strings.TrimSpace(findingID))
}
