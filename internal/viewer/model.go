package viewer

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/permaditya/log-manager/internal/logs"
)

// BackMsg is sent when the user navigates back to the file list.
type BackMsg struct{}

// watchTickMsg is sent on every watch interval.
type watchTickMsg struct{}

const watchInterval = 2 * time.Second

func watchTick() tea.Cmd {
	return tea.Tick(watchInterval, func(time.Time) tea.Msg {
		return watchTickMsg{}
	})
}

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Padding(0, 1)
	footerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	searchStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	filterStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	matchStyle   = lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0"))
	currentMatch = lipgloss.NewStyle().Background(lipgloss.Color("11")).Foreground(lipgloss.Color("0")).Bold(true)
	savedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
)

// Model is the log viewer screen.
type Model struct {
	file          logs.LogFile
	viewport      viewport.Model
	lines         []string // raw lines of the file
	err           string
	ready         bool
	width         int
	height        int
	searching     bool
	pattern       string
	caseSensitive bool  // when true, search matches exact case
	matches       []int // line indices that match (in full view)
	matchIdx      int
	filterMode    bool // show only matching lines
	savedMsg      string
	showLineNums  bool // toggle line numbers
	jumping       bool // jump-to-line mode
	jumpInput     string
	watching      bool // watch mode — auto-reload every 2s
	virtual       bool // true for in-memory content (grep results), no watch
}

func New(file logs.LogFile, width, height int) Model {
	m := Model{
		file:   file,
		width:  width,
		height: height,
	}
	content, err := logs.Read(file)
	if err != nil {
		m.err = err.Error()
		return m
	}
	m.lines = strings.Split(content, "\n")
	m.viewport = viewport.New(width, height-3)
	m.ready = true
	m.refreshView()
	return m
}

// NewVirtual creates a viewer from in-memory content (e.g. grep results).
// Pass an empty string for content when results will be streamed in via Append.
// Watch mode is not available for virtual files.
func NewVirtual(title, content string, width, height int) Model {
	m := Model{
		file:    logs.LogFile{Name: title},
		width:   width,
		height:  height,
		virtual: true,
	}
	m.viewport = viewport.New(width, height-3)
	m.ready = true
	if content != "" {
		m.lines = strings.Split(content, "\n")
		m.refreshView()
	}
	return m
}

// Append adds content to a virtual viewer as streaming results arrive.
func (m *Model) Append(content string) {
	if content == "" {
		return
	}
	content = strings.TrimRight(content, "\n")
	newLines := strings.Split(content, "\n")
	savedOffset := m.viewport.YOffset
	m.lines = append(m.lines, newLines...)
	m.refreshView()
	m.viewport.SetYOffset(savedOffset)
}

// SetTitle updates the title shown in the viewer header.
func (m *Model) SetTitle(title string) {
	m.file.Name = title
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3

	case watchTickMsg:
		if m.watching {
			m.reloadFile()
			return m, watchTick()
		}
		return m, nil

	case tea.KeyMsg:
		if m.searching {
			return m.handleSearch(msg)
		}
		if m.jumping {
			return m.handleJump(msg)
		}
		return m.handleNav(msg)
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleNav(msg tea.KeyMsg) (Model, tea.Cmd) {
	// clear transient saved message on any key
	m.savedMsg = ""

	switch msg.String() {
	case "q", "esc":
		if m.pattern != "" {
			m.pattern = ""
			m.matches = nil
			m.matchIdx = 0
			m.filterMode = false
			m.viewport.SetContent(strings.Join(m.lines, "\n"))
			return m, nil
		}
		return m, func() tea.Msg { return BackMsg{} }

	case "/":
		m.searching = true
		m.pattern = ""
		return m, nil

	case ":":
		m.jumping = true
		m.jumpInput = ""
		return m, nil

	case "tab":
		m.caseSensitive = !m.caseSensitive
		if m.pattern != "" {
			m.applySearch()
		}
		return m, nil

	case "f":
		if m.pattern != "" {
			m.filterMode = !m.filterMode
			m.refreshView()
		}
		return m, nil

	case "e":
		if m.pattern != "" {
			msg := m.exportFiltered()
			m.savedMsg = msg
		}
		return m, nil

	case "n":
		if len(m.matches) > 0 {
			m.matchIdx = (m.matchIdx + 1) % len(m.matches)
			m.viewport.SetYOffset(m.visibleOffset(m.matchIdx))
		}
		return m, nil

	case "N":
		if len(m.matches) > 0 {
			m.matchIdx = (m.matchIdx - 1 + len(m.matches)) % len(m.matches)
			m.viewport.SetYOffset(m.visibleOffset(m.matchIdx))
		}
		return m, nil

	case "g":
		m.viewport.GotoTop()
		return m, nil

	case "G":
		m.viewport.GotoBottom()
		return m, nil

	case "ctrl+d":
		m.viewport.HalfViewDown()
		return m, nil

	case "ctrl+u":
		m.viewport.HalfViewUp()
		return m, nil

	case "L":
		m.showLineNums = !m.showLineNums
		m.refreshView()
		return m, nil

	case "W":
		if m.virtual {
			return m, nil // watch not available for grep results
		}
		m.watching = !m.watching
		if m.watching {
			return m, watchTick()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) handleSearch(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.pattern = ""
		m.matches = nil
		m.matchIdx = 0
		m.filterMode = false
		m.viewport.SetContent(strings.Join(m.lines, "\n"))
	case "enter":
		m.searching = false
		m.applySearch()
	case "tab":
		m.caseSensitive = !m.caseSensitive
	case "backspace", "ctrl+h":
		if len(m.pattern) > 0 {
			m.pattern = m.pattern[:len(m.pattern)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.pattern += msg.String()
		}
	}
	return m, nil
}

func (m *Model) applySearch() {
	if m.pattern == "" {
		m.matches = nil
		m.filterMode = false
		m.viewport.SetContent(strings.Join(m.lines, "\n"))
		return
	}

	m.matches = nil
	for i, line := range m.lines {
		if m.lineMatches(line) {
			m.matches = append(m.matches, i)
		}
	}

	m.matchIdx = 0
	m.refreshView()
	if len(m.matches) > 0 {
		m.viewport.SetYOffset(m.visibleOffset(0))
	}
}

func (m Model) handleJump(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.jumping = false
		m.jumpInput = ""
	case "enter":
		m.jumping = false
		if m.jumpInput == "" {
			return m, nil
		}
		n := 0
		for _, ch := range m.jumpInput {
			if ch < '0' || ch > '9' {
				m.jumpInput = ""
				return m, nil
			}
			n = n*10 + int(ch-'0')
		}
		// clamp to valid range
		if n < 1 {
			n = 1
		}
		if n > len(m.lines) {
			n = len(m.lines)
		}
		m.viewport.SetYOffset(n - 1)
		m.jumpInput = ""
	case "backspace", "ctrl+h":
		if len(m.jumpInput) > 0 {
			m.jumpInput = m.jumpInput[:len(m.jumpInput)-1]
		}
	default:
		// only accept digits
		if len(msg.String()) == 1 && msg.String() >= "0" && msg.String() <= "9" {
			m.jumpInput += msg.String()
		}
	}
	return m, nil
}

// lineMatches checks if line contains the pattern, respecting caseSensitive.
// patterns splits the search pattern on | for OR matching.
func (m *Model) patterns() []string {
	parts := strings.Split(m.pattern, "|")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func (m *Model) lineMatches(line string) bool {
	searchLine := line
	if !m.caseSensitive {
		searchLine = strings.ToLower(line)
	}
	for _, p := range m.patterns() {
		pat := p
		if !m.caseSensitive {
			pat = strings.ToLower(p)
		}
		if strings.Contains(searchLine, pat) {
			return true
		}
	}
	return false
}

// refreshView rebuilds viewport content based on filterMode and showLineNums.
func (m *Model) refreshView() {
	width := len(fmt.Sprintf("%d", len(m.lines)))

	pats := m.patterns()

	if m.filterMode {
		var filtered []string
		for i, idx := range m.matches {
			line := highlightAll(m.lines[idx], pats, m.caseSensitive)
			filtered = append(filtered, m.prefixLine(line, idx+1, i, width))
		}
		m.viewport.SetContent(strings.Join(filtered, "\n"))
		m.viewport.GotoTop()
		return
	}

	highlighted := make([]string, len(m.lines))
	for i, line := range m.lines {
		rendered := line
		if m.pattern != "" && m.lineMatches(line) {
			rendered = highlightAll(line, pats, m.caseSensitive)
		}
		highlighted[i] = m.prefixLine(rendered, i+1, i, width)
	}
	m.viewport.SetContent(strings.Join(highlighted, "\n"))
	if len(m.matches) > 0 {
		m.viewport.SetYOffset(m.visibleOffset(m.matchIdx))
	}
}

var lineNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

// prefixLine optionally prepends a line number.
func (m *Model) prefixLine(line string, lineNum, _ int, width int) string {
	if !m.showLineNums {
		return line
	}
	num := fmt.Sprintf("%*d  ", width, lineNum)
	return lineNumStyle.Render(num) + line
}

// visibleOffset returns the line offset in the current view for a match index.
// In filter mode, match i is at line i. In full mode, it's the actual line index.
func (m *Model) visibleOffset(idx int) int {
	if idx < 0 || idx >= len(m.matches) {
		return 0
	}
	if m.filterMode {
		return idx
	}
	return m.matches[idx]
}

// exportFiltered writes matching lines to a file and returns a status message.
func (m *Model) exportFiltered() string {
	if len(m.matches) == 0 {
		return "nothing to export"
	}

	var sb strings.Builder
	for _, idx := range m.matches {
		sb.WriteString(m.lines[idx])
		sb.WriteByte('\n')
	}

	ts := time.Now().Format("20060102-150405")
	// Use just the bare file name (strip grep result metadata) and the in-viewer pattern.
	baseName := m.file.Name
	if m.virtual {
		// For grep results the name is "grep: <pattern> — N match(es)"; use "grep" prefix only.
		baseName = "grep"
	}
	filename := fmt.Sprintf("%s.%s.%s.out", baseName, m.pattern, ts)
	// Keep only alphanumerics, dots, and hyphens; collapse everything else to underscores.
	filename = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '.' {
			return r
		}
		return '_'
	}, filename)
	// Collapse consecutive underscores.
	for strings.Contains(filename, "__") {
		filename = strings.ReplaceAll(filename, "__", "_")
	}
	filename = strings.Trim(filename, "_")

	if err := os.WriteFile(filename, []byte(sb.String()), 0644); err != nil {
		return fmt.Sprintf("export failed: %v", err)
	}
	return fmt.Sprintf("saved → %s (%d lines)", filename, len(m.matches))
}

// reloadFile re-reads the file and updates the viewport, keeping scroll at bottom.
func (m *Model) reloadFile() {
	content, err := logs.Read(m.file)
	if err != nil {
		return
	}
	m.lines = strings.Split(content, "\n")
	m.refreshView()
	m.viewport.GotoBottom()
}

func caseLabel(sensitive bool) string {
	if sensitive {
		return "sensitive"
	}
	return "insensitive"
}

// highlightLine wraps matches in the line with color.
// highlightAll highlights all sub-patterns (split by |) in the line.
func highlightAll(line string, patterns []string, caseSensitive bool) string {
	result := line
	for _, p := range patterns {
		result = highlightLine(result, p, caseSensitive)
	}
	return result
}

func highlightLine(line, pattern string, caseSensitive bool) string {
	var result strings.Builder
	remaining := line
	searchIn := line
	if !caseSensitive {
		searchIn = strings.ToLower(line)
		pattern = strings.ToLower(pattern)
	}

	for {
		idx := strings.Index(searchIn, pattern)
		if idx < 0 {
			result.WriteString(remaining)
			break
		}
		result.WriteString(remaining[:idx])
		result.WriteString(matchStyle.Render(remaining[idx : idx+len(pattern)]))
		remaining = remaining[idx+len(pattern):]
		searchIn = searchIn[idx+len(pattern):]
	}
	return result.String()
}

func (m Model) View() string {
	var sb strings.Builder

	title := m.file.Name
	if m.file.Compressed {
		title += " [gz]"
	}
	if m.filterMode {
		title += " [filtered]"
	}
	sb.WriteString(headerStyle.Render(" "+title) + "\n")

	if m.err != "" {
		sb.WriteString(errorStyle.Render("  error: "+m.err) + "\n")
		return sb.String()
	}
	if !m.ready {
		sb.WriteString("  loading...\n")
		return sb.String()
	}

	sb.WriteString(m.viewport.View() + "\n")

	sep := strings.Repeat("─", m.width)
	sb.WriteString(footerStyle.Render(sep) + "\n")

	switch {
	case m.jumping:
		sb.WriteString(searchStyle.Render(" :"+m.jumpInput+"█") +
			footerStyle.Render("  enter to jump • esc cancel"))

	case m.searching:
		caseFlag := footerStyle.Render("  [tab case:" + caseLabel(m.caseSensitive) + "]")
		sb.WriteString(searchStyle.Render(" / "+m.pattern+"█") + caseFlag)

	case m.savedMsg != "":
		sb.WriteString(savedStyle.Render("  " + m.savedMsg))

	case m.pattern != "":
		matchInfo := fmt.Sprintf(" [%d/%d]", m.matchIdx+1, len(m.matches))
		if len(m.matches) == 0 {
			matchInfo = " [no matches]"
		}
		filterHint := "  f filter-only"
		if m.filterMode {
			filterHint = filterStyle.Render("  f all-lines")
		}
		caseHint := footerStyle.Render("  tab case:" + caseLabel(m.caseSensitive))
		sb.WriteString(
			searchStyle.Render(" /"+m.pattern) +
				footerStyle.Render(matchInfo+"  n/N next/prev"+filterHint+"  e export  esc clear") +
				caseHint,
		)

	default:
		pct := int(m.viewport.ScrollPercent() * 100)
		lineNumHint := "off"
		if m.showLineNums {
			lineNumHint = "on"
		}
		watchHint := ""
		if !m.virtual {
			if m.watching {
				watchHint = filterStyle.Render("  W watching…")
			} else {
				watchHint = footerStyle.Render("  W watch")
			}
		}
		sb.WriteString(footerStyle.Render(
			fmt.Sprintf("  q back • / search • : jump • L line-nums:%s • g/G top/bottom  %d%%", lineNumHint, pct),
		) + watchHint)
	}

	return sb.String()
}
