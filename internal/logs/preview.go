package logs

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/permaditya/log-manager/internal/sshclient"
)

// ReadPreview reads only the first n lines of a log file.
// Stops early — never loads the whole file into memory.
func ReadPreview(lf LogFile, n int) string {
	if lf.SSH != nil {
		return readPreviewSSH(lf, n)
	}
	return readPreviewLocal(lf, n)
}

func readPreviewLocal(lf LogFile, n int) string {
	f, err := os.Open(lf.Path)
	if err != nil {
		return ""
	}
	defer f.Close()

	var r io.Reader = f
	if lf.Compressed {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return ""
		}
		defer gz.Close()
		r = gz
	}

	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) >= n {
			break
		}
	}
	return strings.Join(lines, "\n")
}

func readPreviewSSH(lf LogFile, n int) string {
	s := lf.SSH
	var cmd string
	if lf.Compressed {
		cmd = fmt.Sprintf("zcat %s 2>/dev/null | head -n %d", lf.Path, n)
	} else {
		cmd = fmt.Sprintf("head -n %d %s", n, lf.Path)
	}
	out, err := sshclient.Default.RunCommand(s.User, s.Host, s.Port, s.Identity, s.Password, cmd)
	if err != nil {
		return fmt.Sprintf("(ssh preview error: %v)", err)
	}
	return strings.TrimRight(string(out), "\n")
}
