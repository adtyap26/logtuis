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

const previewLines = 10

var (
	titleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Padding(0, 1)
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	normalStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	gzStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	searchStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	grepStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
	statusStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	previewHdrStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	previewStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
)

type inputMode int

const (
	modeNormal inputMode = iota
	modeSearch
	modeGrep
)

// Model is the file list screen.
type Model struct {
	dir               string
	all               []logs.LogFile
	filtered          []logs.LogFile
	cursor            int
	search            string
	mode              inputMode
	grepCaseSensitive bool
	width             int
	height            int
	preview           string // cached preview of selected file
}

func New(dir string, files []logs.LogFile) Model {
	m := Model{
		dir:      dir,
		all:      files,
		filtered: files,
	}
	m.updatePreview()
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
			m.updatePreview()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.updatePreview()
		}
	case "g":
		m.cursor = 0
		m.updatePreview()
	case "G":
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
			m.updatePreview()
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
	case "tab":
		m.grepCaseSensitive = !m.grepCaseSensitive
	case "enter":
		if m.search == "" {
			m.mode = modeNormal
			return m, nil
		}
		pattern := m.search
		dir := m.dir
		cs := m.grepCaseSensitive
		m.mode = modeNormal
		m.search = ""
		return m, func() tea.Msg {
			content := logs.GrepAll(dir, pattern, cs)
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
	} else {
		query := strings.ToLower(m.search)
		var result []logs.LogFile
		for _, f := range m.all {
			if strings.Contains(strings.ToLower(f.Name), query) {
				result = append(result, f)
			}
		}
		m.filtered = result
	}
	m.cursor = 0
	m.updatePreview()
}

func (m *Model) updatePreview() {
	if len(m.filtered) == 0 {
		m.preview = ""
		return
	}
	m.preview = logs.ReadPreview(m.filtered[m.cursor], previewLines)
}

func (m Model) View() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render(" Log Viewer") + "\n\n")

	// input bar
	switch m.mode {
	case modeSearch:
		sb.WriteString(searchStyle.Render(" / "+m.search+"█") + "\n\n")
	case modeGrep:
		caseLabel := "insensitive"
		if m.grepCaseSensitive {
			caseLabel = "sensitive"
		}
		sb.WriteString(grepStyle.Render(" grep all: "+m.search+"█") +
			statusStyle.Render("  enter search • tab case:"+caseLabel+" • esc cancel") + "\n\n")
	default:
		if m.search != "" {
			sb.WriteString(searchStyle.Render(" / "+m.search) + statusStyle.Render("  (esc to clear)") + "\n\n")
		} else {
			sb.WriteString(helpStyle.Render(" / filter • ctrl+f grep all • j/k navigate • enter open • ctrl+r reload • q quit") + "\n\n")
		}
	}

	if len(m.filtered) == 0 {
		sb.WriteString(statusStyle.Render("  no log files found\n"))
		return sb.String()
	}

	// reserve space: header(2) + input(2) + separator(1) + status(2) + preview header(1) + preview lines
	reserved := 8 + previewLines
	listH := m.height - reserved
	if listH < 3 {
		listH = 3
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

	// status bar
	sb.WriteString("\n" + statusStyle.Render(sep) + "\n")
	sb.WriteString(statusStyle.Render(fmt.Sprintf("  %d/%d files", len(m.filtered), len(m.all))) + "\n")

	// preview pane
	if m.preview != "" {
		selected := m.filtered[m.cursor]
		sb.WriteString(previewHdrStyle.Render(" Preview: "+selected.Name) + "\n")
		for _, line := range strings.Split(m.preview, "\n") {
			// truncate long lines to terminal width
			if m.width > 4 && len(line) > m.width-4 {
				line = line[:m.width-4] + "…"
			}
			sb.WriteString(previewStyle.Render("  "+line) + "\n")
		}
	}

	return sb.String()
}
