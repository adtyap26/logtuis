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
func NewVirtual(title, content string, width, height int) Model {
	m := Model{
		file:   logs.LogFile{Name: title},
		width:  width,
		height: height,
	}
	m.lines = strings.Split(content, "\n")
	m.viewport = viewport.New(width, height-3)
	m.ready = true
	m.refreshView()
	return m
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

	case tea.KeyMsg:
		if m.searching {
			return m.handleSearch(msg)
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

// lineMatches checks if line contains the pattern, respecting caseSensitive.
func (m *Model) lineMatches(line string) bool {
	if m.caseSensitive {
		return strings.Contains(line, m.pattern)
	}
	return strings.Contains(strings.ToLower(line), strings.ToLower(m.pattern))
}

// refreshView rebuilds viewport content based on filterMode and showLineNums.
func (m *Model) refreshView() {
	width := len(fmt.Sprintf("%d", len(m.lines)))

	if m.filterMode {
		var filtered []string
		for i, idx := range m.matches {
			line := highlightLine(m.lines[idx], m.pattern, m.caseSensitive)
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
			rendered = highlightLine(line, m.pattern, m.caseSensitive)
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
	filename := fmt.Sprintf("%s.%s.%s.out", m.file.Name, m.pattern, ts)
	// sanitize filename
	filename = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, filename)

	if err := os.WriteFile(filename, []byte(sb.String()), 0644); err != nil {
		return fmt.Sprintf("export failed: %v", err)
	}
	return fmt.Sprintf("saved → %s (%d lines)", filename, len(m.matches))
}

func caseLabel(sensitive bool) string {
	if sensitive {
		return "sensitive"
	}
	return "insensitive"
}

// highlightLine wraps matches in the line with color.
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
		sb.WriteString(footerStyle.Render(
			fmt.Sprintf("  q back • / search • ctrl+f grep • L line-nums:%s • g/G top/bottom  %d%%", lineNumHint, pct),
		))
	}

	return sb.String()
}
