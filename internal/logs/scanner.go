package logs

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
)

// LogFile represents a discovered log file.
type LogFile struct {
	Path       string
	Name       string
	Compressed bool
	Size       int64
	Mode       os.FileMode
	ModTime    time.Time
	Owner      string
}

// plainSuffixes matches files ending with these exact suffixes.
var plainSuffixes = []string{
	".log",
	".txt",
	".out",
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
			lf := LogFile{
				Path:       filepath.Join(dir, name),
				Name:       name,
				Compressed: compressed,
			}
			if info, err := e.Info(); err == nil {
				lf.Size = info.Size()
				lf.Mode = info.Mode()
				lf.ModTime = info.ModTime()
				lf.Owner = fileOwner(info)
			}
			files = append(files, lf)
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

// fileOwner returns the username of the file owner, or empty string if unavailable.
func fileOwner(info os.FileInfo) string {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if u, err := user.LookupId(strconv.Itoa(int(stat.Uid))); err == nil {
			return u.Username
		}
	}
	return ""
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
