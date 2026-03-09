package logs

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadPreviewPlain(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nline3\nline4\nline5\nline6\n"
	path := filepath.Join(dir, "app.log")
	os.WriteFile(path, []byte(content), 0644)

	lf := LogFile{Path: path, Name: "app.log"}
	got := ReadPreview(lf, 3)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line1" {
		t.Errorf("expected line1, got %q", lines[0])
	}
}

func TestReadPreviewGzip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log.gz")
	f, _ := os.Create(path)
	gz := gzip.NewWriter(f)
	gz.Write([]byte("a\nb\nc\nd\ne\n"))
	gz.Close()
	f.Close()

	lf := LogFile{Path: path, Name: "app.log.gz", Compressed: true}
	got := ReadPreview(lf, 2)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestReadPreviewFewerLinesThanN(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	os.WriteFile(path, []byte("only\ntwo\n"), 0644)

	lf := LogFile{Path: path, Name: "app.log"}
	got := ReadPreview(lf, 10)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %v", len(lines), lines)
	}
}

func TestReadPreviewMissingFile(t *testing.T) {
	lf := LogFile{Path: "/nonexistent/file.log", Name: "file.log"}
	got := ReadPreview(lf, 5)
	if got != "" {
		t.Errorf("expected empty string for missing file, got %q", got)
	}
}
