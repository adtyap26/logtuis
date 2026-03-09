package logs

import (
	"bufio"
	"compress/gzip"
	"io"
	"os"
	"strings"
)

// ReadPreview reads only the first n lines of a log file.
// Stops early — never loads the whole file into memory.
func ReadPreview(lf LogFile, n int) string {
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
