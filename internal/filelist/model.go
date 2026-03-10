package filelist

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/permaditya/log-manager/internal/logs"
)

// OpenFileMsg is sent when the user selects a log file to open.
type OpenFileMsg struct {
	File logs.LogFile
}

// GrepStartMsg is sent immediately when a grep begins — opens the viewer right away.
type GrepStartMsg struct {
	Pattern string
	Ch      <-chan logs.GrepChunk
}

// GrepChunkMsg carries per-file grep results as they stream in.
type GrepChunkMsg struct {
	Content string
	Pattern string
	Ch      <-chan logs.GrepChunk
}

// GrepDoneMsg is sent once all files have been searched.
type GrepDoneMsg struct {
	Pattern string
	Total   int
}

const previewLines = 15

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
	checkedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	archiveStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
)

type inputMode int

const (
	modeNormal inputMode = iota
	modeSearch
	modeGrep
)

// archiveDoneMsg is sent when the tar.gz creation completes.
type archiveDoneMsg struct {
	path string
	err  error
}

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
	grepLoading       bool
	spinner           spinner.Model
	selecting         bool
	selected          map[int]bool // indices into m.filtered
	archiving         bool
	archiveMsg        string
}

func New(dir string, files []logs.LogFile) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))

	m := Model{
		dir:      dir,
		all:      files,
		filtered: files,
		spinner:  sp,
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
	case spinner.TickMsg:
		if m.grepLoading || m.archiving {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case GrepStartMsg:
		// viewer is about to open — clear the spinner
		m.grepLoading = false
	case archiveDoneMsg:
		m.archiving = false
		m.selecting = false
		m.selected = nil
		if msg.err != nil {
			m.archiveMsg = "archive failed: " + msg.err.Error()
		} else {
			m.archiveMsg = "saved → " + msg.path
		}
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
	// clear transient archive message on any key
	m.archiveMsg = ""

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
	case "V":
		if m.selecting {
			m.selecting = false
			m.selected = nil
		} else {
			m.selecting = true
			m.selected = make(map[int]bool)
		}
	case " ":
		if m.selecting {
			if m.selected[m.cursor] {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = true
			}
		}
	case "/":
		m.mode = modeSearch
		m.search = ""
	case "ctrl+f":
		m.mode = modeGrep
		m.search = ""
	case "esc":
		if m.selecting {
			m.selecting = false
			m.selected = nil
			return m, nil
		}
		m.search = ""
		m.mode = modeNormal
		m.applyFilter()
	case "enter":
		if m.selecting && len(m.selected) > 0 {
			var toArchive []logs.LogFile
			for idx := range m.selected {
				if idx < len(m.filtered) {
					toArchive = append(toArchive, m.filtered[idx])
				}
			}
			m.archiving = true
			return m, tea.Batch(makeArchiveCmd(toArchive), m.spinner.Tick)
		}
		if !m.selecting && len(m.filtered) > 0 {
			file := m.filtered[m.cursor]
			return m, func() tea.Msg {
				return OpenFileMsg{File: file}
			}
		}
	}
	return m, nil
}

func makeArchiveCmd(files []logs.LogFile) tea.Cmd {
	return func() tea.Msg {
		ts := time.Now().Format("20060102-150405")
		dest := fmt.Sprintf("logtuis-%s.tar.gz", ts)
		err := logs.Archive(files, dest)
		return archiveDoneMsg{path: dest, err: err}
	}
}

func (m Model) handleSearch(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Paste {
		m.search += string(msg.Runes)
		m.applyFilter()
		return m, nil
	}
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
	if msg.Paste {
		m.search += string(msg.Runes)
		return m, nil
	}
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
		m.grepLoading = true
		ch := logs.GrepStream(dir, pattern, cs)
		startCmd := func() tea.Msg {
			return GrepStartMsg{Pattern: pattern, Ch: ch}
		}
		return m, tea.Batch(startCmd, m.spinner.Tick)
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

func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if time.Since(t) > 180*24*time.Hour {
		return t.Format("_2 Jan 2006")
	}
	return t.Format("_2 Jan 15:04")
}

func (m Model) View() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render(" Log Viewer") + "\n\n")

	// input bar
	switch {
	case m.archiving:
		sb.WriteString(archiveStyle.Render(" archiving: ") + m.spinner.View() +
			statusStyle.Render(" creating tar.gz…") + "\n\n")
	case m.archiveMsg != "":
		sb.WriteString(archiveStyle.Render("  "+m.archiveMsg) + "\n\n")
	case m.selecting:
		sb.WriteString(checkedStyle.Render(fmt.Sprintf(" V select [%d selected]", len(m.selected))) +
			statusStyle.Render("  space toggle • j/k move+mark • enter archive • esc cancel") + "\n\n")
	case m.grepLoading:
		sb.WriteString(grepStyle.Render(" grep all: ") + m.spinner.View() +
			statusStyle.Render(" searching…") + "\n\n")
	case m.mode == modeSearch:
		sb.WriteString(searchStyle.Render(" / "+m.search+"█") + "\n\n")
	case m.mode == modeGrep:
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
			sb.WriteString(helpStyle.Render(" / filter • ctrl+f grep • V select+archive • j/k navigate • enter open • ctrl+r reload • q quit") + "\n\n")
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

	// Compute column widths from visible files for alignment.
	maxName, maxOwner := 0, 0
	for _, f := range m.filtered {
		if len(f.Name) > maxName {
			maxName = len(f.Name)
		}
		if len(f.Owner) > maxOwner {
			maxOwner = len(f.Owner)
		}
	}

	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	for i := start; i < end; i++ {
		f := m.filtered[i]
		nameStyle := normalStyle
		if f.Compressed {
			nameStyle = gzStyle
		}

		namePad := fmt.Sprintf("%-*s", maxName, f.Name)
		meta := ""
		if !f.ModTime.IsZero() {
			ownerPad := fmt.Sprintf("%-*s", maxOwner, f.Owner)
			meta = metaStyle.Render(fmt.Sprintf("  %s  %5s  %s  %s",
				f.Mode.String(), humanSize(f.Size), ownerPad, formatDate(f.ModTime)))
		}

		check := "  "
		if m.selecting && m.selected[i] {
			check = checkedStyle.Render("✓ ")
		}

		if i == m.cursor {
			sb.WriteString(selectedStyle.Render(" > ") + check + nameStyle.Render(namePad) + meta + "\n")
		} else {
			sb.WriteString("   " + check + nameStyle.Render(namePad) + meta + "\n")
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
