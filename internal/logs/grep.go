package logs

import (
	"fmt"
	"strings"
)

// GrepAll searches pattern across all log files in dir.
// Returns formatted results as "filename:linenum:\tcontent\n".
func GrepAll(dir, pattern string, caseSensitive bool) string {
	if pattern == "" {
		return ""
	}

	files, err := Scan(dir)
	if err != nil {
		return fmt.Sprintf("scan error: %v\n", err)
	}

	lowerPat := strings.ToLower(pattern)
	var sb strings.Builder
	totalMatches := 0

	for _, f := range files {
		content, err := Read(f)
		if err != nil {
			continue
		}
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			var matched bool
			if caseSensitive {
				matched = strings.Contains(line, pattern)
			} else {
				matched = strings.Contains(strings.ToLower(line), lowerPat)
			}
			if matched {
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
