package viewer

import (
	"fmt"
	"strings"

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
	matchStyle   = lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0"))
	currentMatch = lipgloss.NewStyle().Background(lipgloss.Color("11")).Foreground(lipgloss.Color("0")).Bold(true)
)

// Model is the log viewer screen.
type Model struct {
	file      logs.LogFile
	viewport  viewport.Model
	lines     []string // raw lines of the file
	err       string
	ready     bool
	width     int
	height    int
	searching bool
	pattern   string
	matches   []int // line indices that match
	matchIdx  int   // current match position
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
	vp := viewport.New(width, height-3)
	vp.SetContent(content)
	m.viewport = vp
	m.ready = true
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
	switch msg.String() {
	case "q", "esc":
		if m.pattern != "" {
			// first esc clears search
			m.pattern = ""
			m.matches = nil
			m.matchIdx = 0
			m.viewport.SetContent(strings.Join(m.lines, "\n"))
			return m, nil
		}
		return m, func() tea.Msg { return BackMsg{} }
	case "/":
		m.searching = true
		return m, nil
	case "n":
		if len(m.matches) > 0 {
			m.matchIdx = (m.matchIdx + 1) % len(m.matches)
			m.viewport.SetYOffset(m.matches[m.matchIdx])
		}
		return m, nil
	case "N":
		if len(m.matches) > 0 {
			m.matchIdx = (m.matchIdx - 1 + len(m.matches)) % len(m.matches)
			m.viewport.SetYOffset(m.matches[m.matchIdx])
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
		m.viewport.SetContent(strings.Join(m.lines, "\n"))
	case "enter":
		m.searching = false
		m.applySearch()
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
		m.viewport.SetContent(strings.Join(m.lines, "\n"))
		return
	}

	lower := strings.ToLower(m.pattern)
	var highlighted []string
	m.matches = nil

	for i, line := range m.lines {
		if strings.Contains(strings.ToLower(line), lower) {
			m.matches = append(m.matches, i)
			highlighted = append(highlighted, highlightLine(line, m.pattern, i == 0))
		} else {
			highlighted = append(highlighted, line)
		}
	}

	m.matchIdx = 0
	m.viewport.SetContent(strings.Join(highlighted, "\n"))
	if len(m.matches) > 0 {
		m.viewport.SetYOffset(m.matches[0])
	}
}

// highlightLine wraps matches in the line with color.
func highlightLine(line, pattern string, isCurrent bool) string {
	lower := strings.ToLower(line)
	lowerPat := strings.ToLower(pattern)

	style := matchStyle
	if isCurrent {
		style = currentMatch
	}

	var result strings.Builder
	for {
		idx := strings.Index(lower, lowerPat)
		if idx < 0 {
			result.WriteString(line)
			break
		}
		result.WriteString(line[:idx])
		result.WriteString(style.Render(line[idx : idx+len(pattern)]))
		line = line[idx+len(pattern):]
		lower = lower[idx+len(pattern):]
	}
	return result.String()
}

func (m Model) View() string {
	var sb strings.Builder

	title := m.file.Name
	if m.file.Compressed {
		title += " [gz]"
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

	if m.searching {
		sb.WriteString(searchStyle.Render(" / " + m.pattern + "█"))
	} else if m.pattern != "" {
		matchInfo := fmt.Sprintf("  [%d/%d matches]", m.matchIdx+1, len(m.matches))
		if len(m.matches) == 0 {
			matchInfo = "  [no matches]"
		}
		sb.WriteString(searchStyle.Render(" /"+m.pattern) + footerStyle.Render(matchInfo+
			"  n/N next/prev • esc clear"))
	} else {
		pct := int(m.viewport.ScrollPercent() * 100)
		sb.WriteString(footerStyle.Render(
			fmt.Sprintf("  q back • / search • j/k scroll • ctrl+d/u page • g/G top/bottom  %d%%", pct),
		))
	}

	return sb.String()
}
