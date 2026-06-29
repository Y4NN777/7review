package review

import (
	"sort"
	"strconv"
	"strings"
)

// BuildInlinePositions returns changed new-side lines that can be used for
// provider inline comments. It does not publish anything.
func BuildInlinePositions(source Source) []InlinePosition {
	diff := source.Diff
	if diff == nil {
		return nil
	}
	meta := changedFileMetadata(source)
	provider := ""
	diffRefs := DiffRefs{}
	if source.SCM != nil {
		provider = source.SCM.Provider
		diffRefs = source.SCM.DiffRefs
	}
	var out []InlinePosition
	for _, file := range diff.Files {
		lines := ChangedNewLines(file.Patch)
		if len(lines) == 0 {
			continue
		}
		fileMeta := meta[file.Path]
		newPath := firstNonEmptyInline(fileMeta.NewPath, file.Path)
		oldPath := firstNonEmptyInline(fileMeta.OldPath, newPath)
		for line := range lines {
			pos := InlinePosition{
				Path:     newPath,
				OldPath:  oldPath,
				NewPath:  newPath,
				Line:     line,
				Side:     "RIGHT",
				Provider: provider,
				DiffRefs: diffRefs,
				Valid:    true,
			}
			switch {
			case provider == "gitlab" && (diffRefs.BaseSHA == "" || diffRefs.HeadSHA == "" || diffRefs.StartSHA == ""):
				pos.Valid = false
				pos.Reason = "gitlab diff refs are incomplete"
			case provider == "github" && diffRefs.HeadSHA == "":
				pos.Valid = false
				pos.Reason = "github head SHA is missing"
			}
			out = append(out, pos)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Line < out[j].Line
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func changedFileMetadata(source Source) map[string]ChangedFile {
	out := make(map[string]ChangedFile)
	for _, file := range source.ChangedFiles {
		if file.NewPath != "" {
			out[file.NewPath] = file
		}
	}
	if source.SCM != nil {
		for _, file := range source.SCM.Files {
			if file.NewPath != "" {
				out[file.NewPath] = file
			}
		}
	}
	return out
}

// ChangedNewLines returns added/new-side line numbers from a unified diff patch.
func ChangedNewLines(patch string) map[int]bool {
	lines := make(map[int]bool)
	newLine := 0
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "@@") {
			newLine = parseInlineHunkNewStart(line)
			continue
		}
		if newLine == 0 || strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "+"):
			lines[newLine] = true
			newLine++
		case strings.HasPrefix(line, "-"):
		default:
			newLine++
		}
	}
	return lines
}

func parseInlineHunkNewStart(line string) int {
	start := strings.Index(line, "+")
	if start == -1 {
		return 0
	}
	rest := line[start+1:]
	end := len(rest)
	for i, r := range rest {
		if r == ',' || r == ' ' || r == '@' {
			end = i
			break
		}
	}
	value, err := strconv.Atoi(strings.TrimSpace(rest[:end]))
	if err != nil {
		return 0
	}
	return value
}

func firstNonEmptyInline(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
