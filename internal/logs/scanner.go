package logs

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/permaditya/log-manager/internal/sshclient"
)

// SSHConfig holds the connection details for a remote log source.
type SSHConfig struct {
	Name     string
	Host     string
	Port     int
	User     string
	Identity string
	Password string
	Path     string // remote directory path
}

// LogFile represents a discovered log file (local or remote).
type LogFile struct {
	Path       string
	Name       string
	Compressed bool
	Size       int64
	Mode       os.FileMode
	ModTime    time.Time
	Owner      string
	SSH        *SSHConfig // nil for local files
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

// ScanSSH lists log files on a remote server under the given path via SSH.
// It runs `ls -la <path>` on the remote host and parses the filenames.
func ScanSSH(cfg SSHConfig) ([]LogFile, error) {
	// Use `|| true` so ls always exits 0 — a missing/empty path returns empty output, not an error.
	out, err := sshclient.Default.RunCommand(cfg.User, cfg.Host, cfg.Port, cfg.Identity, cfg.Password,
		fmt.Sprintf("ls -la %s 2>/dev/null || true", cfg.Path))
	if err != nil {
		return nil, fmt.Errorf("ssh ls %s@%s:%s: %w", cfg.User, cfg.Host, cfg.Path, err)
	}

	var files []LogFile
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		// ls -la output: permissions links owner group size month day time name
		if len(fields) < 9 {
			continue
		}
		name := fields[len(fields)-1]
		if name == "." || name == ".." {
			continue
		}
		match, compressed := classify(name)
		if !match {
			continue
		}

		size, _ := strconv.ParseInt(fields[4], 10, 64)
		owner := fields[2]
		mode := parseModeString(fields[0])
		modTime := parseLsDate(fields[5], fields[6], fields[7])

		lf := LogFile{
			Path:       cfg.Path + "/" + name,
			Name:       name,
			Compressed: compressed,
			Size:       size,
			Owner:      owner,
			Mode:       mode,
			ModTime:    modTime,
			SSH:        &cfg,
		}
		files = append(files, lf)
	}
	return files, nil
}

// parseModeString converts a ls -la permission string (e.g. "-rw-r--r--") to os.FileMode.
func parseModeString(s string) os.FileMode {
	if len(s) < 10 {
		return 0
	}
	var m os.FileMode
	if s[1] == 'r' {
		m |= 0400
	}
	if s[2] == 'w' {
		m |= 0200
	}
	if s[3] == 'x' || s[3] == 's' {
		m |= 0100
	}
	if s[4] == 'r' {
		m |= 0040
	}
	if s[5] == 'w' {
		m |= 0020
	}
	if s[6] == 'x' || s[6] == 's' {
		m |= 0010
	}
	if s[7] == 'r' {
		m |= 0004
	}
	if s[8] == 'w' {
		m |= 0002
	}
	if s[9] == 'x' || s[9] == 't' {
		m |= 0001
	}
	return m
}

// parseLsDate parses the date fields from ls -la output.
// Handles both "Mar  9 05:50" (current year) and "Mar  9  2024" (past year) formats.
func parseLsDate(month, day, timeOrYear string) time.Time {
	// If timeOrYear contains ":", it's a time (HH:MM) — use current year.
	if strings.Contains(timeOrYear, ":") {
		t, err := time.Parse("Jan 2 15:04 2006",
			fmt.Sprintf("%s %s %s %d", month, day, timeOrYear, time.Now().Year()))
		if err == nil {
			return t
		}
	}
	// Otherwise it's a year.
	t, err := time.Parse("Jan 2 2006", fmt.Sprintf("%s %s %s", month, day, timeOrYear))
	if err == nil {
		return t
	}
	return time.Time{}
}
