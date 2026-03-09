package logs

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// LogFile represents a discovered log file.
type LogFile struct {
	Path       string
	Name       string
	Compressed bool
}

// plainSuffixes matches files ending with these exact suffixes.
var plainSuffixes = []string{
	".log",
	".txt",
}

// Scan walks dir and returns all log files.
// Matches: *.log, *.txt, *.log.gz, *.log.N.gz, *.log.N (rotated, e.g. server.log.2)
func Scan(dir string) ([]LogFile, error) {
	var files []LogFile

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if match, compressed := classify(name); match {
			files = append(files, LogFile{
				Path:       filepath.Join(dir, name),
				Name:       name,
				Compressed: compressed,
			})
		}
	}

	return files, nil
}

// classify returns (isLogFile, isCompressed).
func classify(name string) (bool, bool) {
	// plain suffixes: .log, .txt
	for _, suffix := range plainSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true, false
		}
	}

	// gzip compressed: *.log.gz, *.log.N.gz
	if strings.Contains(name, ".log") && strings.HasSuffix(name, ".gz") {
		return true, true
	}

	// rotated plain: *.log.N (e.g. server.log.2, server.log.10)
	if isRotatedLog(name) {
		return true, false
	}

	return false, false
}

// isRotatedLog matches files like server.log.2, app.log.10
// Rule: contains ".log." and ends with one or more digits.
func isRotatedLog(name string) bool {
	if !strings.Contains(name, ".log.") {
		return false
	}
	suffix := name[strings.LastIndex(name, ".")+1:]
	if len(suffix) == 0 {
		return false
	}
	for _, ch := range suffix {
		if !unicode.IsDigit(ch) {
			return false
		}
	}
	return true
}
