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

	"github.com/permaditya/log-manager/internal/sshclient"
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

// ShellStream runs an arbitrary shell command via sh -c and streams the output.
// The command runs in the process's current working directory, so glob patterns
// like * expand against the files in that directory.
func ShellStream(dir, cmd string) <-chan GrepChunk {
	ch := make(chan GrepChunk, 4)
	go func() {
		defer close(ch)
		if cmd == "" {
			return
		}
		c := exec.Command("sh", "-c", cmd)
		c.Dir = dir
		out, err := c.Output()
		if len(out) > 0 {
			ch <- GrepChunk{Content: string(out)}
		} else if err != nil {
			if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
				ch <- GrepChunk{Content: string(ee.Stderr)}
			}
		}
		ch <- GrepChunk{Done: true, Total: strings.Count(string(out), "\n")}
	}()
	return ch
}

// GrepChunk holds grep results from one file, used for streaming.
// When Done is true, no more chunks will arrive and Total has the final count.
type GrepChunk struct {
	Content string
	Count   int
	Done    bool
	Total   int
}

// GrepStream searches pattern across all log files in dir concurrently,
// streaming per-file results as each goroutine completes.
// The returned channel is closed after the Done sentinel is sent.
func GrepStream(dir, pattern string, caseSensitive bool) <-chan GrepChunk {
	ch := make(chan GrepChunk, 32)
	go func() {
		defer close(ch)
		if pattern == "" {
			return
		}
		files, err := Scan(dir)
		if err != nil || len(files) == 0 {
			return
		}

		grepBin, _ := exec.LookPath("grep")
		zcatBin, _ := exec.LookPath("zcat")
		re := buildRegexp(pattern, caseSensitive)

		type result struct {
			content string
			count   int
		}
		resultCh := make(chan result, len(files))

		for _, f := range files {
			go func(lf LogFile) {
				var lines []string
				if grepBin != "" {
					switch {
					case lf.Compressed && zcatBin != "":
						lines = grepGzWithZcat(zcatBin, grepBin, lf, pattern, caseSensitive)
					case !lf.Compressed:
						lines = grepFileBin(grepBin, lf, pattern, caseSensitive)
					case re != nil:
						lines = grepFileGo(lf, re)
					}
				} else if re != nil {
					lines = grepFileGo(lf, re)
				}
				var sb strings.Builder
				for _, l := range lines {
					sb.WriteString(l)
				}
				resultCh <- result{content: sb.String(), count: len(lines)}
			}(f)
		}

		total := 0
		for range files {
			r := <-resultCh
			total += r.count
			if r.content != "" {
				ch <- GrepChunk{Content: r.content, Count: r.count}
			}
		}
		ch <- GrepChunk{Done: true, Total: total}
	}()
	return ch
}

// grepFileBin runs system grep on a single plain file, returning formatted match lines.
func grepFileBin(bin string, lf LogFile, pattern string, caseSensitive bool) []string {
	args := []string{"-En", "--color=never"}
	if !caseSensitive {
		args = append(args, "-i")
	}
	args = append(args, pattern, lf.Path)
	out, _ := exec.Command(bin, args...).Output()
	var lines []string
	for _, line := range bytes.Split(out, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		lines = append(lines, string(shortenPath(line, []string{lf.Path}))+"\n")
	}
	return lines
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

// UniqueSSHSources returns one SSHConfig per distinct host+path from a file list.
func UniqueSSHSources(files []LogFile) []SSHConfig {
	seen := map[string]bool{}
	var result []SSHConfig
	for _, f := range files {
		if f.SSH == nil {
			continue
		}
		key := fmt.Sprintf("%s@%s:%d:%s", f.SSH.User, f.SSH.Host, f.SSH.Port, f.SSH.Path)
		if !seen[key] {
			seen[key] = true
			result = append(result, *f.SSH)
		}
	}
	return result
}

// GrepStreamFiles searches pattern across a pre-built file list (local + SSH).
func GrepStreamFiles(files []LogFile, pattern string, caseSensitive bool) <-chan GrepChunk {
	ch := make(chan GrepChunk, 32)
	go func() {
		defer close(ch)
		if pattern == "" || len(files) == 0 {
			ch <- GrepChunk{Done: true}
			return
		}

		var localFiles []LogFile
		sshGroups := map[string][]LogFile{}
		sshCfgMap := map[string]SSHConfig{}
		for _, f := range files {
			if f.SSH == nil {
				localFiles = append(localFiles, f)
			} else {
				key := fmt.Sprintf("%s@%s:%d:%s", f.SSH.User, f.SSH.Host, f.SSH.Port, f.SSH.Path)
				sshGroups[key] = append(sshGroups[key], f)
				sshCfgMap[key] = *f.SSH
			}
		}

		type result struct {
			content string
			count   int
		}
		numWorkers := len(sshGroups)
		if len(localFiles) > 0 {
			numWorkers++
		}
		if numWorkers == 0 {
			ch <- GrepChunk{Done: true}
			return
		}

		resultCh := make(chan result, numWorkers)

		// Local grep — reuse existing per-file workers.
		if len(localFiles) > 0 {
			go func() {
				grepBin, _ := exec.LookPath("grep")
				zcatBin, _ := exec.LookPath("zcat")
				re := buildRegexp(pattern, caseSensitive)
				inner := make(chan result, len(localFiles))
				for _, f := range localFiles {
					go func(lf LogFile) {
						var lines []string
						if grepBin != "" {
							switch {
							case lf.Compressed && zcatBin != "":
								lines = grepGzWithZcat(zcatBin, grepBin, lf, pattern, caseSensitive)
							case !lf.Compressed:
								lines = grepFileBin(grepBin, lf, pattern, caseSensitive)
							case re != nil:
								lines = grepFileGo(lf, re)
							}
						} else if re != nil {
							lines = grepFileGo(lf, re)
						}
						var sb strings.Builder
						for _, l := range lines {
							sb.WriteString(l)
						}
						inner <- result{content: sb.String(), count: len(lines)}
					}(f)
				}
				var sb strings.Builder
				total := 0
				for range localFiles {
					r := <-inner
					total += r.count
					sb.WriteString(r.content)
				}
				resultCh <- result{content: sb.String(), count: total}
			}()
		}

		// SSH grep — one goroutine per server.
		for key, sshFiles := range sshGroups {
			cfg := sshCfgMap[key]
			go func(c SSHConfig, fs []LogFile) {
				content, count := grepSSH(c, fs, pattern, caseSensitive)
				resultCh <- result{content: content, count: count}
			}(cfg, sshFiles)
		}

		total := 0
		for i := 0; i < numWorkers; i++ {
			r := <-resultCh
			total += r.count
			if r.content != "" {
				ch <- GrepChunk{Content: r.content, Count: r.count}
			}
		}
		ch <- GrepChunk{Done: true, Total: total}
	}()
	return ch
}

// grepSSH runs grep on a remote server for the given files.
func grepSSH(cfg SSHConfig, files []LogFile, pattern string, caseSensitive bool) (string, int) {
	pathToName := map[string]string{}
	var plainPaths, gzPaths []string
	for _, f := range files {
		pathToName[f.Path] = f.Name
		if f.Compressed {
			gzPaths = append(gzPaths, f.Path)
		} else {
			plainPaths = append(plainPaths, f.Path)
		}
	}

	flags := "-En --color=never"
	if !caseSensitive {
		flags += " -i"
	}
	quoted := shellQuote(pattern)

	var parts []string
	if len(plainPaths) > 0 {
		parts = append(parts, fmt.Sprintf("grep %s %s %s 2>/dev/null",
			flags, quoted, strings.Join(plainPaths, " ")))
	}
	for _, gz := range gzPaths {
		name := pathToName[gz]
		parts = append(parts, fmt.Sprintf("zcat %s 2>/dev/null | grep %s %s | sed 's|^|%s:|'",
			gz, flags, quoted, name))
	}
	if len(parts) == 0 {
		return "", 0
	}

	cmd := strings.Join(parts, "; ") + " || true"
	out, err := sshclient.Default.RunCommand(cfg.User, cfg.Host, cfg.Port, cfg.Identity, cfg.Password, cmd)
	if err != nil || len(out) == 0 {
		return "", 0
	}

	var sb strings.Builder
	count := 0
	prefix := "[" + cfg.Name + "] "
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		// Shorten full remote path to just filename.
		for path, name := range pathToName {
			if strings.HasPrefix(line, path) {
				line = name + line[len(path):]
				break
			}
		}
		sb.WriteString(prefix + line + "\n")
		count++
	}
	return sb.String(), count
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ShellStreamAll runs a shell command locally and on all SSH sources.
// On each SSH source, the command runs with the log path as working directory.
func ShellStreamAll(localDir string, sshSources []SSHConfig, cmd string) <-chan GrepChunk {
	ch := make(chan GrepChunk, 4)
	go func() {
		defer close(ch)
		if cmd == "" {
			return
		}

		var sb strings.Builder
		multiSource := len(sshSources) > 0

		// Local.
		if localDir != "" {
			c := exec.Command("sh", "-c", cmd)
			c.Dir = localDir
			out, err := c.Output()
			if len(out) > 0 {
				if multiSource {
					sb.WriteString("# local\n")
				}
				sb.Write(out)
			} else if err != nil {
				if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
					sb.Write(ee.Stderr)
				}
			}
		}

		// SSH sources.
		for _, src := range sshSources {
			remoteCmd := fmt.Sprintf("cd %s 2>/dev/null && %s || true", src.Path, cmd)
			out, _ := sshclient.Default.RunCommand(src.User, src.Host, src.Port, src.Identity, src.Password, remoteCmd)
			if len(out) > 0 {
				sb.WriteString(fmt.Sprintf("# %s\n", src.Name))
				sb.Write(out)
			}
		}

		content := sb.String()
		if content != "" {
			ch <- GrepChunk{Content: content}
		}
		ch <- GrepChunk{Done: true, Total: strings.Count(content, "\n")}
	}()
	return ch
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
