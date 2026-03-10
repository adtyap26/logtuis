package logs

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"github.com/permaditya/log-manager/internal/sshclient"
)

// Read returns the full content of a log file as a string.
// Handles both plain text, gzip-compressed, and remote SSH files.
func Read(lf LogFile) (string, error) {
	if lf.SSH != nil {
		return readSSH(lf)
	}
	return readLocal(lf)
}

func readLocal(lf LogFile) (string, error) {
	f, err := os.Open(lf.Path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var r io.Reader = f
	if lf.Compressed {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return "", err
		}
		defer gz.Close()
		r = gz
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func readSSH(lf LogFile) (string, error) {
	s := lf.SSH
	var cmd string
	if lf.Compressed {
		cmd = fmt.Sprintf("zcat %s 2>/dev/null || gzip -dc %s", lf.Path, lf.Path)
	} else {
		cmd = fmt.Sprintf("cat %s", lf.Path)
	}
	out, err := sshclient.Default.RunCommand(s.User, s.Host, s.Port, s.Identity, s.Password, cmd)
	if err != nil {
		return "", fmt.Errorf("ssh read %s: %w", lf.Path, err)
	}
	return string(out), nil
}
