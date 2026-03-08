package viewer

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/permaditya/log-manager/internal/logs"
)

func makePlainLog(t *testing.T, content string) logs.LogFile {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return logs.LogFile{Path: path, Name: "test.log", Compressed: false}
}

func makeGzLog(t *testing.T, content string) logs.LogFile {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log.1.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	gz.Write([]byte(content))
	gz.Close()
	f.Close()
	return logs.LogFile{Path: path, Name: "test.log.1.gz", Compressed: true}
}

func TestNewPlain(t *testing.T) {
	lf := makePlainLog(t, "hello log\n")
	m := New(lf, 80, 24)
	if !m.ready {
		t.Error("expected ready")
	}
	if m.err != "" {
		t.Errorf("unexpected error: %s", m.err)
	}
}

func TestNewGzip(t *testing.T) {
	lf := makeGzLog(t, "compressed log content\n")
	m := New(lf, 80, 24)
	if !m.ready {
		t.Error("expected ready")
	}
}

func TestNewMissingFile(t *testing.T) {
	lf := logs.LogFile{Path: "/nonexistent/path.log", Name: "path.log", Compressed: false}
	m := New(lf, 80, 24)
	if m.ready {
		t.Error("should not be ready for missing file")
	}
	if m.err == "" {
		t.Error("expected error message")
	}
}

func TestBackKey(t *testing.T) {
	lf := makePlainLog(t, "log content\n")
	m := New(lf, 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected command on q")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestEscBack(t *testing.T) {
	lf := makePlainLog(t, "log content\n")
	m := New(lf, 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected command on esc")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestViewContainsFilename(t *testing.T) {
	lf := makePlainLog(t, "some log line\n")
	m := New(lf, 80, 24)
	view := m.View()
	if !strings.Contains(view, "test.log") {
		t.Error("view should contain filename")
	}
}

func TestViewGzLabel(t *testing.T) {
	lf := makeGzLog(t, "gz content\n")
	m := New(lf, 80, 24)
	view := m.View()
	if !strings.Contains(view, "[gz]") {
		t.Error("view should contain [gz] label for compressed files")
	}
}
