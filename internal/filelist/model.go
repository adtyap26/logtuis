package filelist

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/permaditya/log-manager/internal/logs"
)

// OpenFileMsg is sent when the user selects a log file to open.
type OpenFileMsg struct {
	File logs.LogFile
}

// GrepResultMsg is sent when the user runs a global grep.
type GrepResultMsg struct {
	Title   string
	Content string
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Padding(0, 1)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	normalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	gzStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	searchStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	grepStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

type inputMode int

const (
	modeNormal inputMode = iota
	modeSearch           // / fuzzy filter
	modeGrep             // ctrl+f global grep
)

// Model is the file list screen.
type Model struct {
	dir      string
	all      []logs.LogFile
	filtered []logs.LogFile
	cursor   int
	search   string
	mode     inputMode
	width    int
	height   int
}

func New(dir string, files []logs.LogFile) Model {
	return Model{
		dir:      dir,
		all:      files,
		filtered: files,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch m.mode {
		case modeSearch:
			return m.handleSearch(msg)
		case modeGrep:
			return m.handleGrepInput(msg)
		default:
			return m.handleNav(msg)
		}
	}
	return m, nil
}

func (m Model) handleNav(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g":
		m.cursor = 0
	case "G":
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		}
	case "/":
		m.mode = modeSearch
		m.search = ""
	case "ctrl+f":
		m.mode = modeGrep
		m.search = ""
	case "esc":
		m.search = ""
		m.mode = modeNormal
		m.applyFilter()
	case "enter":
		if len(m.filtered) > 0 {
			file := m.filtered[m.cursor]
			return m, func() tea.Msg {
				return OpenFileMsg{File: file}
			}
		}
	}
	return m, nil
}

func (m Model) handleSearch(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.search = ""
		m.applyFilter()
	case "enter":
		m.mode = modeNormal
	case "backspace", "ctrl+h":
		if len(m.search) > 0 {
			m.search = m.search[:len(m.search)-1]
			m.applyFilter()
		}
	default:
		if len(msg.String()) == 1 {
			m.search += msg.String()
			m.applyFilter()
		}
	}
	return m, nil
}

func (m Model) handleGrepInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.search = ""
	case "enter":
		if m.search == "" {
			m.mode = modeNormal
			return m, nil
		}
		pattern := m.search
		dir := m.dir
		m.mode = modeNormal
		m.search = ""
		return m, func() tea.Msg {
			content := logs.GrepAll(dir, pattern, false)
			return GrepResultMsg{
				Title:   "grep: " + pattern,
				Content: content,
			}
		}
	case "backspace", "ctrl+h":
		if len(m.search) > 0 {
			m.search = m.search[:len(m.search)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.search += msg.String()
		}
	}
	return m, nil
}

func (m *Model) applyFilter() {
	if m.search == "" {
		m.filtered = m.all
		m.cursor = 0
		return
	}
	query := strings.ToLower(m.search)
	var result []logs.LogFile
	for _, f := range m.all {
		if strings.Contains(strings.ToLower(f.Name), query) {
			result = append(result, f)
		}
	}
	m.filtered = result
	m.cursor = 0
}

func (m Model) View() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render(" Log Viewer") + "\n\n")

	switch m.mode {
	case modeSearch:
		sb.WriteString(searchStyle.Render(" / "+m.search+"█") + "\n\n")
	case modeGrep:
		sb.WriteString(grepStyle.Render(" grep all: "+m.search+"█") +
			statusStyle.Render("  enter to search • esc cancel") + "\n\n")
	default:
		if m.search != "" {
			sb.WriteString(searchStyle.Render(" / "+m.search) + statusStyle.Render("  (esc to clear)") + "\n\n")
		} else {
			sb.WriteString(helpStyle.Render(" / filter • ctrl+f grep all files • j/k navigate • enter open • r reload • q quit") + "\n\n")
		}
	}

	if len(m.filtered) == 0 {
		sb.WriteString(statusStyle.Render("  no log files found\n"))
		return sb.String()
	}

	listH := m.height - 6
	if listH < 1 {
		listH = len(m.filtered)
	}
	start := m.cursor - listH/2
	if start < 0 {
		start = 0
	}
	end := start + listH
	if end > len(m.filtered) {
		end = len(m.filtered)
		start = end - listH
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		f := m.filtered[i]
		nameStyle := normalStyle
		if f.Compressed {
			nameStyle = gzStyle
		}
		if i == m.cursor {
			sb.WriteString(selectedStyle.Render(" > ") + nameStyle.Render(f.Name) + "\n")
		} else {
			sb.WriteString("   " + nameStyle.Render(f.Name) + "\n")
		}
	}

	sep := strings.Repeat("─", m.width)
	sb.WriteString("\n" + statusStyle.Render(sep) + "\n")
	sb.WriteString(statusStyle.Render(fmt.Sprintf("  %d/%d files", len(m.filtered), len(m.all))))

	return sb.String()
}
