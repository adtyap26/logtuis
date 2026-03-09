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

func doSearch(t *testing.T, m Model, pattern string) Model {
	t.Helper()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	for _, ch := range pattern {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return m
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
	m = doSearch(t, m, "ERROR")
	if len(m.matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(m.matches))
	}
}

func TestSearchNextPrev(t *testing.T) {
	content := "ERROR one\nINFO skip\nERROR two\nERROR three\n"
	lf := makePlainLog(t, content)
	m := New(lf, 80, 24)
	m = doSearch(t, m, "ERROR")

	if m.matchIdx != 0 {
		t.Errorf("initial matchIdx=%d, want 0", m.matchIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.matchIdx != 1 {
		t.Errorf("after n: matchIdx=%d, want 1", m.matchIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	if m.matchIdx != 0 {
		t.Errorf("after N: matchIdx=%d, want 0", m.matchIdx)
	}
	// wrap around backwards
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	if m.matchIdx != 2 {
		t.Errorf("after wrap N: matchIdx=%d, want 2", m.matchIdx)
	}
}

func TestSearchNoMatches(t *testing.T) {
	lf := makePlainLog(t, "INFO only lines here\n")
	m := New(lf, 80, 24)
	m = doSearch(t, m, "ERROR")
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
	m = doSearch(t, m, "ERROR")

	// first esc clears pattern
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.pattern != "" {
		t.Error("pattern should be cleared after esc")
	}
	if len(m.matches) != 0 {
		t.Error("matches should be cleared")
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

func TestFilterMode(t *testing.T) {
	content := "ERROR one\nINFO skip\nERROR two\nDEBUG ignore\n"
	lf := makePlainLog(t, content)
	m := New(lf, 80, 24)
	m = doSearch(t, m, "ERROR")

	if m.filterMode {
		t.Error("filter mode should be off initially")
	}

	// toggle filter mode on
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if !m.filterMode {
		t.Error("filter mode should be on after f")
	}
	if !strings.Contains(m.View(), "[filtered]") {
		t.Error("view should show [filtered] label in header")
	}

	// toggle filter mode off
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if m.filterMode {
		t.Error("filter mode should be off after second f")
	}
}

func TestFilterModeRequiresPattern(t *testing.T) {
	lf := makePlainLog(t, "INFO line\n")
	m := New(lf, 80, 24)

	// f without pattern should do nothing
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if m.filterMode {
		t.Error("filter mode should not activate without a pattern")
	}
}

func TestExport(t *testing.T) {
	// run in temp dir so exported file ends up there
	origDir, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	content := "ERROR one\nINFO skip\nERROR two\n"
	lf := makePlainLog(t, content)
	m := New(lf, 80, 24)
	m = doSearch(t, m, "ERROR")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if m.savedMsg == "" {
		t.Error("expected saved message after export")
	}
	if strings.Contains(m.savedMsg, "failed") {
		t.Errorf("export failed: %s", m.savedMsg)
	}

	// verify a file was created
	entries, _ := os.ReadDir(dir)
	var found bool
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".out") {
			found = true
			data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) != 2 {
				t.Errorf("expected 2 exported lines, got %d", len(lines))
			}
		}
	}
	if !found {
		t.Error("no .out file was created")
	}
}

func TestExportNoMatches(t *testing.T) {
	lf := makePlainLog(t, "INFO only\n")
	m := New(lf, 80, 24)
	m = doSearch(t, m, "ERROR")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if !strings.Contains(m.savedMsg, "nothing") {
		t.Errorf("expected 'nothing to export', got: %s", m.savedMsg)
	}
}

func TestCaseSensitiveSearch(t *testing.T) {
	content := "ERROR upper\nerror lower\nError mixed\n"
	lf := makePlainLog(t, content)
	m := New(lf, 80, 24)

	// default: case insensitive — all 3 match
	m = doSearch(t, m, "ERROR")
	if len(m.matches) != 3 {
		t.Errorf("case-insensitive: expected 3 matches, got %d", len(m.matches))
	}

	// toggle case sensitive
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.caseSensitive {
		t.Error("expected caseSensitive=true after ctrl+i")
	}
	// re-apply same pattern
	m = doSearch(t, m, "ERROR")
	if len(m.matches) != 1 {
		t.Errorf("case-sensitive: expected 1 match, got %d", len(m.matches))
	}
}

func TestCaseToggleWhileSearching(t *testing.T) {
	lf := makePlainLog(t, "ERROR line\n")
	m := New(lf, 80, 24)

	// open search bar and toggle case mid-typing
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.caseSensitive {
		t.Error("ctrl+i should toggle case while searching")
	}
}

func TestJumpToLine(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5\n"
	lf := makePlainLog(t, content)
	m := New(lf, 80, 24)

	// enter jump mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	if !m.jumping {
		t.Fatal("expected jump mode after :")
	}

	// type line number
	for _, ch := range "3" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if m.jumpInput != "3" {
		t.Errorf("expected jumpInput=3, got %q", m.jumpInput)
	}

	// confirm
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.jumping {
		t.Error("should exit jump mode after enter")
	}
	if m.jumpInput != "" {
		t.Error("jumpInput should be cleared after jump")
	}
}

func TestJumpEsc(t *testing.T) {
	lf := makePlainLog(t, "a\nb\nc\n")
	m := New(lf, 80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.jumping {
		t.Error("should exit jump mode after esc")
	}
	if m.jumpInput != "" {
		t.Error("jumpInput should be cleared after esc")
	}
}

func TestJumpIgnoresNonDigits(t *testing.T) {
	lf := makePlainLog(t, "a\nb\nc\n")
	m := New(lf, 80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m.jumpInput != "" {
		t.Errorf("non-digit should be ignored, got %q", m.jumpInput)
	}
}

func TestJumpClampsToMax(t *testing.T) {
	lf := makePlainLog(t, "a\nb\nc\n")
	m := New(lf, 80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	for _, ch := range "9999" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// should not panic — clamped to len(lines)
	if m.jumping {
		t.Error("should exit jump mode")
	}
}

func TestHighlightLine(t *testing.T) {
	line := "2024-01-01 ERROR something failed"
	result := highlightLine(line, "ERROR", false)
	if !strings.Contains(result, "ERROR") {
		t.Error("highlighted line should still contain ERROR text")
	}
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
