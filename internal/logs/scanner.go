package logs

import (
	"os"
	"path/filepath"
	"strings"
)

// LogFile represents a discovered log file.
type LogFile struct {
	Path      string
	Name      string
	Compressed bool
}

// Scan walks dir and returns all log files (*.log, *.log.*.gz).
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
		if isLogFile(name) {
			files = append(files, LogFile{
				Path:       filepath.Join(dir, name),
				Name:       name,
				Compressed: isGzip(name),
			})
		}
	}

	return files, nil
}

func isLogFile(name string) bool {
	// plain: *.log
	if strings.HasSuffix(name, ".log") {
		return true
	}
	// compressed: *.log.gz, *.log.1.gz, *.log.2.gz, etc.
	if strings.Contains(name, ".log") && strings.HasSuffix(name, ".gz") {
		return true
	}
	return false
}

func isGzip(name string) bool {
	return strings.HasSuffix(name, ".gz")
}
