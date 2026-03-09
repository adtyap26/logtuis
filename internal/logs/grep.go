package logs

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// GrepAll searches pattern across all log files in dir.
// Uses system grep binary if available (much faster), falls back to Go impl.
// Pattern supports full grep -E regex (e.g. "ERROR|API", "timeout.*retry").
func GrepAll(dir, pattern string, caseSensitive bool) string {
	if pattern == "" {
		return ""
	}

	files, err := Scan(dir)
	if err != nil {
		return fmt.Sprintf("scan error: %v\n", err)
	}
	if len(files) == 0 {
		return fmt.Sprintf("no log files found in %s\n", dir)
	}

	if grepBin, err := exec.LookPath("grep"); err == nil {
		return grepWithBinary(grepBin, files, pattern, caseSensitive)
	}

	return grepWithGo(files, pattern, caseSensitive)
}

// grepWithBinary shells out to the system grep binary.
func grepWithBinary(bin string, files []LogFile, pattern string, caseSensitive bool) string {
	// separate plain and compressed files — grep can't read gz natively
	var plainPaths []string
	var gzFiles []LogFile
	for _, f := range files {
		if f.Compressed {
			gzFiles = append(gzFiles, f)
		} else {
			plainPaths = append(plainPaths, f.Path)
		}
	}

	var sb strings.Builder
	totalMatches := 0

	// grep plain files
	if len(plainPaths) > 0 {
		args := []string{"-En", "--color=never"}
		if !caseSensitive {
			args = append(args, "-i")
		}
		args = append(args, pattern)
		args = append(args, plainPaths...)

		out, _ := exec.Command(bin, args...).Output()
		if len(out) > 0 {
			// shorten absolute paths to just filename for display
			for _, line := range bytes.Split(out, []byte("\n")) {
				if len(line) == 0 {
					continue
				}
				sb.Write(shortenPath(line, plainPaths))
				sb.WriteByte('\n')
				totalMatches++
			}
		}
	}

	// grep gz files via zcat | grep (or Go fallback per file)
	if len(gzFiles) > 0 {
		zcat, zcatErr := exec.LookPath("zcat")
		for _, f := range gzFiles {
			var matches []string
			if zcatErr == nil {
				matches = grepGzWithZcat(zcat, bin, f, pattern, caseSensitive)
			} else {
				re := buildRegexp(pattern, caseSensitive)
				if re != nil {
					matches = grepFileGo(f, re)
				}
			}
			for _, m := range matches {
				sb.WriteString(m)
				totalMatches++
			}
		}
	}

	if totalMatches == 0 {
		return fmt.Sprintf("no matches for %q\n", pattern)
	}

	header := fmt.Sprintf("# grep: %q — %d match(es)\n\n", pattern, totalMatches)
	return header + sb.String()
}

// grepGzWithZcat pipes zcat output into grep.
func grepGzWithZcat(zcat, grep string, f LogFile, pattern string, caseSensitive bool) []string {
	zcatCmd := exec.Command(zcat, f.Path)
	grepArgs := []string{"-En", "--color=never"}
	if !caseSensitive {
		grepArgs = append(grepArgs, "-i")
	}
	grepArgs = append(grepArgs, pattern)
	grepCmd := exec.Command(grep, grepArgs...)

	pipe, err := zcatCmd.StdoutPipe()
	if err != nil {
		return nil
	}
	grepCmd.Stdin = pipe

	var out bytes.Buffer
	grepCmd.Stdout = &out

	zcatCmd.Start()
	grepCmd.Start()
	zcatCmd.Wait()
	grepCmd.Wait()

	var matches []string
	for _, line := range bytes.Split(out.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		matches = append(matches, fmt.Sprintf("%s:%s\n", f.Name, line))
	}
	return matches
}

// shortenPath replaces the absolute file path prefix with just the filename in a grep output line.
func shortenPath(line []byte, paths []string) []byte {
	for _, p := range paths {
		if bytes.HasPrefix(line, []byte(p)) {
			// extract just the filename from path
			parts := strings.Split(p, "/")
			name := parts[len(parts)-1]
			return append([]byte(name), line[len(p):]...)
		}
	}
	return line
}

// --- Go fallback implementation ---

func buildRegexp(pattern string, caseSensitive bool) *regexp.Regexp {
	regexPat := pattern
	if !caseSensitive {
		regexPat = "(?i)" + pattern
	}
	re, err := regexp.Compile(regexPat)
	if err != nil {
		return nil
	}
	return re
}

func grepWithGo(files []LogFile, pattern string, caseSensitive bool) string {
	re := buildRegexp(pattern, caseSensitive)
	if re == nil {
		return fmt.Sprintf("invalid pattern %q\n", pattern)
	}

	type fileMatches struct {
		order   int
		matches []string
	}

	resultsCh := make(chan fileMatches, len(files))
	var wg sync.WaitGroup

	for i, f := range files {
		wg.Add(1)
		go func(order int, lf LogFile) {
			defer wg.Done()
			resultsCh <- fileMatches{order: order, matches: grepFileGo(lf, re)}
		}(i, f)
	}

	wg.Wait()
	close(resultsCh)

	all := make([]fileMatches, 0, len(files))
	for r := range resultsCh {
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].order < all[j].order })

	var sb strings.Builder
	totalMatches := 0
	for _, r := range all {
		for _, line := range r.matches {
			sb.WriteString(line)
			totalMatches++
		}
	}

	if totalMatches == 0 {
		return fmt.Sprintf("no matches for %q\n", pattern)
	}

	header := fmt.Sprintf("# grep: %q — %d match(es) across %d file(s)\n\n", pattern, totalMatches, len(files))
	return header + sb.String()
}

func grepFileGo(lf LogFile, re *regexp.Regexp) []string {
	f, err := os.Open(lf.Path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var scanner *bufio.Scanner
	if lf.Compressed {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil
		}
		defer gz.Close()
		scanner = bufio.NewScanner(gz)
	} else {
		scanner = bufio.NewScanner(f)
	}

	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	var matches []string
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, fmt.Sprintf("%s:%d:\t%s\n", lf.Name, lineNum, line))
		}
	}
	return matches
}

// splitPatterns splits a pattern on | and trims spaces — used by viewer search.
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

// lineMatchesAny returns true if line contains any of the patterns — used by viewer.
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
