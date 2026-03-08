package logs

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestReadPlain(t *testing.T) {
	dir := t.TempDir()
	content := "2024-01-01 INFO starting server\n2024-01-01 ERROR something failed\n"

	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	lf := LogFile{Path: path, Name: "app.log", Compressed: false}
	got, err := Read(lf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestReadGzip(t *testing.T) {
	dir := t.TempDir()
	content := "2024-01-01 INFO compressed log entry\n"

	path := filepath.Join(dir, "app.log.1.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	gz.Close()
	f.Close()

	lf := LogFile{Path: path, Name: "app.log.1.gz", Compressed: true}
	got, err := Read(lf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}
