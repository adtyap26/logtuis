package sourcepicker

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/permaditya/log-manager/internal/logs"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Padding(0, 1)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	normalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	checkedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	sshTagStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
)

// Source represents one selectable log source (local dir or SSH server).
type Source struct {
	Label   string
	Detail  string
	IsLocal bool
	Dir     string
	SSH     *logs.SSHConfig
}

// ConfirmMsg is emitted when the user confirms their selection.
type ConfirmMsg struct {
	Sources []Source
}

// Model is the source picker screen.
type Model struct {
	sources  []Source
	cursor   int
	selected map[int]bool
	width    int
	height   int
}

// New builds a source picker with local dir as the first entry, followed by SSH servers.
func New(localDir string, sshSources []logs.SSHConfig) Model {
	sources := []Source{
		{
			Label:   "local",
			Detail:  localDir,
			IsLocal: true,
			Dir:     localDir,
		},
	}
	for i := range sshSources {
		s := sshSources[i]
		label := s.Name
		if label == "" {
			label = s.Host
		}
		sources = append(sources, Source{
			Label:  label,
			Detail: fmt.Sprintf("%s@%s:%s", s.User, s.Host, s.Path),
			SSH:    &s,
		})
	}
	return Model{
		sources:  sources,
		selected: make(map[int]bool),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.sources)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case " ":
			if m.selected[m.cursor] {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = true
			}
		case "a":
			if len(m.selected) == len(m.sources) {
				m.selected = make(map[int]bool)
			} else {
				for i := range m.sources {
					m.selected[i] = true
				}
			}
		case "enter":
			if len(m.selected) == 0 {
				return m, nil
			}
			var chosen []Source
			for i, s := range m.sources {
				if m.selected[i] {
					chosen = append(chosen, s)
				}
			}
			return m, func() tea.Msg { return ConfirmMsg{Sources: chosen} }
		}
	}
	return m, nil
}

func (m Model) View() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render(" logtuis — select sources") + "\n\n")

	for i, s := range m.sources {
		check := dimStyle.Render("[ ]")
		if m.selected[i] {
			check = checkedStyle.Render("[✓]")
		}

		var label string
		if s.IsLocal {
			label = normalStyle.Render(fmt.Sprintf("  %-12s", s.Label)) + dimStyle.Render(s.Detail)
		} else {
			label = sshTagStyle.Render(fmt.Sprintf("  %-12s", s.Label)) + dimStyle.Render(s.Detail)
		}

		line := "  " + check + label
		if i == m.cursor {
			sb.WriteString(selectedStyle.Render(" >") + line + "\n")
		} else {
			sb.WriteString("   " + line + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("  j/k navigate • space select • a select all • enter confirm • q quit") + "\n")

	return sb.String()
}
