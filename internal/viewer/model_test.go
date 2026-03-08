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
		t.Error("expected ready for gz file")
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
	if _, ok := cmd().(BackMsg); !ok {
		t.Error("expected BackMsg")
	}
}

func TestEscBack(t *testing.T) {
	lf := makePlainLog(t, "log content\n")
	m := New(lf, 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected command on esc")
	}
	if _, ok := cmd().(BackMsg); !ok {
		t.Error("expected BackMsg")
	}
}

func TestViewContainsFilename(t *testing.T) {
	lf := makePlainLog(t, "some log line\n")
	m := New(lf, 80, 24)
	if !strings.Contains(m.View(), "test.log") {
		t.Error("view should contain filename")
	}
}

func TestViewGzLabel(t *testing.T) {
	lf := makeGzLog(t, "gz content\n")
	m := New(lf, 80, 24)
	if !strings.Contains(m.View(), "[gz]") {
		t.Error("view should contain [gz] label")
	}
}

func TestSearchFindsMatches(t *testing.T) {
	content := "INFO starting\nERROR failed\nINFO done\nERROR timeout\n"
	lf := makePlainLog(t, content)
	m := New(lf, 80, 24)

	// open search
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !m.searching {
		t.Fatal("expected searching mode")
	}

	// type "ERROR"
	for _, ch := range "ERROR" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if len(m.matches) != 2 {
		t.Errorf("expected 2 matches for ERROR, got %d", len(m.matches))
	}
}

func TestSearchNextPrev(t *testing.T) {
	content := "ERROR one\nINFO skip\nERROR two\nERROR three\n"
	lf := makePlainLog(t, content)
	m := New(lf, 80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	for _, ch := range "ERROR" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.matchIdx != 0 {
		t.Errorf("expected matchIdx=0, got %d", m.matchIdx)
	}

	// n advances
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.matchIdx != 1 {
		t.Errorf("expected matchIdx=1 after n, got %d", m.matchIdx)
	}

	// N goes back
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	if m.matchIdx != 0 {
		t.Errorf("expected matchIdx=0 after N, got %d", m.matchIdx)
	}

	// wraps around
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	if m.matchIdx != 2 {
		t.Errorf("expected matchIdx=2 (wrap), got %d", m.matchIdx)
	}
}

func TestSearchNoMatches(t *testing.T) {
	lf := makePlainLog(t, "INFO only lines here\n")
	m := New(lf, 80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	for _, ch := range "ERROR" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if len(m.matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(m.matches))
	}
	if !strings.Contains(m.View(), "no matches") {
		t.Error("view should show 'no matches'")
	}
}

func TestSearchEscClears(t *testing.T) {
	content := "ERROR line\nINFO line\n"
	lf := makePlainLog(t, content)
	m := New(lf, 80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	for _, ch := range "ERROR" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// esc clears pattern, stays in viewer
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.pattern != "" {
		t.Error("pattern should be cleared after esc")
	}
	if len(m.matches) != 0 {
		t.Error("matches should be cleared after esc")
	}

	// second esc sends BackMsg
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected BackMsg cmd on second esc")
	}
	if _, ok := cmd().(BackMsg); !ok {
		t.Error("expected BackMsg")
	}
}

func TestHighlightLine(t *testing.T) {
	line := "2024-01-01 ERROR something failed"
	result := highlightLine(line, "ERROR", false)
	if !strings.Contains(result, "ERROR") {
		t.Error("highlighted line should still contain ERROR text")
	}
	// non-match portion should still be present
	if !strings.Contains(result, "2024-01-01") {
		t.Error("highlighted line should preserve non-matching content")
	}
}

func TestHighlightLineCaseInsensitive(t *testing.T) {
	line := "2024-01-01 error something failed"
	result := highlightLine(line, "ERROR", false)
	if !strings.Contains(result, "error") {
		t.Error("highlighted line should preserve original casing")
	}
}
