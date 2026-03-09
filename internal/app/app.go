package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/permaditya/log-manager/internal/filelist"
	"github.com/permaditya/log-manager/internal/logs"
	"github.com/permaditya/log-manager/internal/viewer"
)

type screen int

const (
	screenList screen = iota
	screenViewer
)

// Model is the root application model.
type Model struct {
	dir      string
	screen   screen
	filelist filelist.Model
	viewer   viewer.Model
	width    int
	height   int
	err      string
}

func New(dir string) Model {
	files, err := logs.Scan(dir)
	errMsg := ""
	if err != nil {
		errMsg = fmt.Sprintf("scan error: %v", err)
	}
	return Model{
		dir:      dir,
		screen:   screenList,
		filelist: filelist.New(dir, files),
		err:      errMsg,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case filelist.OpenFileMsg:
		m.viewer = viewer.New(msg.File, m.width, m.height)
		m.screen = screenViewer
		return m, nil

	case filelist.GrepResultMsg:
		m.viewer = viewer.NewVirtual(msg.Title, msg.Content, m.width, m.height)
		m.screen = screenViewer
		return m, nil

	case viewer.BackMsg:
		m.screen = screenList
		return m, nil

	case tea.KeyMsg:
		if m.screen == screenList {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "ctrl+r":
				files, err := logs.Scan(m.dir)
				if err != nil {
					m.err = fmt.Sprintf("scan error: %v", err)
				} else {
					m.err = ""
					m.filelist = filelist.New(m.dir, files)
					m.filelist, _ = m.filelist.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
				}
				return m, nil
			}
		}
	}

	switch m.screen {
	case screenList:
		var cmd tea.Cmd
		m.filelist, cmd = m.filelist.Update(msg)
		return m, cmd
	case screenViewer:
		var cmd tea.Cmd
		m.viewer, cmd = m.viewer.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.err != "" {
		return "error: " + m.err + "\n"
	}
	switch m.screen {
	case screenList:
		return m.filelist.View()
	case screenViewer:
		return m.viewer.View()
	}
	return ""
}
