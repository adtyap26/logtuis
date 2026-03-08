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
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Padding(0, 1)
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
)

// Model is the log viewer screen.
type Model struct {
	file     logs.LogFile
	viewport viewport.Model
	err      string
	ready    bool
	width    int
	height   int
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
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return BackMsg{} }
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
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
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
	pct := int(m.viewport.ScrollPercent() * 100)
	sb.WriteString(footerStyle.Render(sep) + "\n")
	sb.WriteString(footerStyle.Render(
		fmt.Sprintf("  q/esc back • j/k scroll • ctrl+d/u half page • g/G top/bottom  %d%%", pct),
	))

	return sb.String()
}
