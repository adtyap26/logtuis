package logs

import (
	"compress/gzip"
	"io"
	"os"
)

// Read returns the full content of a log file as a string.
// Handles both plain text and gzip-compressed files.
func Read(lf LogFile) (string, error) {
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
