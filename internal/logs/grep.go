package logs

import (
	"fmt"
	"strings"
)

// splitPatterns splits a pattern on | and trims spaces, same as viewer search.
func splitPatterns(pattern string) []string {
	parts := strings.Split(pattern, "|")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// lineMatchesAny returns true if line contains any of the patterns.
func lineMatchesAny(line string, patterns []string, caseSensitive bool) bool {
	searchLine := line
	if !caseSensitive {
		searchLine = strings.ToLower(line)
	}
	for _, p := range patterns {
		pat := p
		if !caseSensitive {
			pat = strings.ToLower(p)
		}
		if strings.Contains(searchLine, pat) {
			return true
		}
	}
	return false
}

// GrepAll searches pattern across all log files in dir.
// Supports OR matching with | separator (e.g. "ERROR|WARN|API").
// Returns formatted results as "filename:linenum:\tcontent\n".
func GrepAll(dir, pattern string, caseSensitive bool) string {
	if pattern == "" {
		return ""
	}

	patterns := splitPatterns(pattern)
	files, err := Scan(dir)
	if err != nil {
		return fmt.Sprintf("scan error: %v\n", err)
	}

	var sb strings.Builder
	totalMatches := 0

	for _, f := range files {
		content, err := Read(f)
		if err != nil {
			continue
		}
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if lineMatchesAny(line, patterns, caseSensitive) {
				sb.WriteString(fmt.Sprintf("%s:%d:\t%s\n", f.Name, i+1, line))
				totalMatches++
			}
		}
	}

	if totalMatches == 0 {
		return fmt.Sprintf("no matches for %q in %s\n", pattern, dir)
	}

	header := fmt.Sprintf("# grep: %q — %d match(es) across %d file(s)\n\n", pattern, totalMatches, len(files))
	return header + sb.String()
}
