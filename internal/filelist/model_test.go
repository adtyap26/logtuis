package filelist

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/permaditya/log-manager/internal/logs"
)

func makeFiles(names ...string) []logs.LogFile {
	var files []logs.LogFile
	for _, n := range names {
		files = append(files, logs.LogFile{
			Name:       n,
			Path:       "/tmp/" + n,
			Compressed: len(n) > 3 && n[len(n)-3:] == ".gz",
		})
	}
	return files
}

func TestNew(t *testing.T) {
	files := makeFiles("app.log", "redis.log.1.gz")
	m := New("/tmp", files)
	if len(m.all) != 2 {
		t.Errorf("expected 2 files, got %d", len(m.all))
	}
	if len(m.filtered) != 2 {
		t.Errorf("expected 2 filtered, got %d", len(m.filtered))
	}
}

func TestNavigation(t *testing.T) {
	files := makeFiles("a.log", "b.log", "c.log")
	m := New("/tmp", files)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 1 {
		t.Errorf("cursor after j: got %d, want 1", m.cursor)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 2 {
		t.Errorf("cursor after jj: got %d, want 2", m.cursor)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 2 {
		t.Errorf("cursor should stay at 2, got %d", m.cursor)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.cursor != 1 {
		t.Errorf("cursor after k: got %d, want 1", m.cursor)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if m.cursor != 2 {
		t.Errorf("cursor after G: got %d, want 2", m.cursor)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if m.cursor != 0 {
		t.Errorf("cursor after g: got %d, want 0", m.cursor)
	}
}

func TestFuzzySearch(t *testing.T) {
	files := makeFiles("app.log", "redis.log", "nginx.log", "redis.log.1.gz")
	m := New("/tmp", files)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if m.mode != modeSearch {
		t.Fatal("expected modeSearch")
	}

	for _, ch := range "redis" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if len(m.filtered) != 2 {
		t.Errorf("expected 2 redis files, got %d", len(m.filtered))
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.search != "" {
		t.Error("search should be cleared after esc")
	}
	if len(m.filtered) != 4 {
		t.Errorf("expected all 4 files after clear, got %d", len(m.filtered))
	}
}

func TestGrepMode(t *testing.T) {
	files := makeFiles("app.log")
	m := New("/tmp", files)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if m.mode != modeGrep {
		t.Fatal("expected modeGrep after ctrl+f")
	}

	for _, ch := range "ERROR" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if m.search != "ERROR" {
		t.Errorf("expected search=ERROR, got %q", m.search)
	}

	// esc cancels grep mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Error("expected modeNormal after esc")
	}
	if m.search != "" {
		t.Error("expected search cleared")
	}
}

func TestGrepModeEmit(t *testing.T) {
	files := makeFiles("app.log")
	m := New("/tmp", files)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	for _, ch := range "ERROR" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd on enter in grep mode")
	}
	if !m.grepLoading {
		t.Error("expected grepLoading=true after enter")
	}
	// cmd is a tea.Batch — run each until we find GrepResultMsg
	msgs := tea.Batch(cmd)()
	batchMsgs, ok := msgs.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", msgs)
	}
	var found bool
	for _, fn := range batchMsgs {
		if msg, ok := fn().(GrepResultMsg); ok {
			found = true
			if msg.Title == "" {
				t.Error("expected non-empty title")
			}
		}
	}
	if !found {
		t.Error("no GrepResultMsg found in batch")
	}
}

func TestOpenFile(t *testing.T) {
	files := makeFiles("app.log")
	m := New("/tmp", files)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command when pressing enter")
	}
	msg := cmd()
	openMsg, ok := msg.(OpenFileMsg)
	if !ok {
		t.Fatalf("expected OpenFileMsg, got %T", msg)
	}
	if openMsg.File.Name != "app.log" {
		t.Errorf("expected app.log, got %s", openMsg.File.Name)
	}
}

func TestOpenFileEmpty(t *testing.T) {
	m := New("/tmp", nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected no command when no files")
	}
}
