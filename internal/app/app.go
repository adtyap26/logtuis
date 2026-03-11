package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/permaditya/log-manager/internal/config"
	"github.com/permaditya/log-manager/internal/filelist"
	"github.com/permaditya/log-manager/internal/logs"
	"github.com/permaditya/log-manager/internal/sourcepicker"
	"github.com/permaditya/log-manager/internal/viewer"
)

// waitForChunk schedules a cmd that blocks until the next GrepChunk arrives.
func waitForChunk(ch <-chan logs.GrepChunk, pattern string) tea.Cmd {
	return func() tea.Msg {
		chunk, ok := <-ch
		if !ok || chunk.Done {
			total := 0
			if ok {
				total = chunk.Total
			}
			return filelist.GrepDoneMsg{Pattern: pattern, Total: total}
		}
		return filelist.GrepChunkMsg{Content: chunk.Content, Pattern: pattern, Ch: ch}
	}
}

type screen int

const (
	screenSourcePicker screen = iota
	screenList
	screenViewer
)

// Model is the root application model.
type Model struct {
	dir             string
	screen          screen
	sourcePicker    sourcepicker.Model
	filelist        filelist.Model
	viewer          viewer.Model
	selectedSources []sourcepicker.Source // stored for ctrl+r re-scan
	width           int
	height          int
	err             string
}

func New(dir string) Model {
	cfg, cfgErr := config.Load()
	errMsg := ""
	if cfgErr != nil {
		errMsg = fmt.Sprintf("config error: %v", cfgErr)
	}

	var sshCfgs []logs.SSHConfig
	for _, s := range cfg.SSHSources {
		sshCfgs = append(sshCfgs, logs.SSHConfig{
			Name:     s.Name,
			Host:     s.Host,
			Port:     s.Port,
			User:     s.User,
			Identity: s.Identity,
			Password: s.Password,
			Path:     s.Path,
		})
	}

	return Model{
		dir:          dir,
		screen:       screenSourcePicker,
		sourcePicker: sourcepicker.New(dir, sshCfgs),
		err:          errMsg,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// scanSources scans all selected sources and returns the combined file list and SSH statuses.
func scanSources(sources []sourcepicker.Source) ([]logs.LogFile, []filelist.SSHStatus) {
	var files []logs.LogFile
	var statuses []filelist.SSHStatus
	for _, src := range sources {
		if src.IsLocal {
			local, _ := logs.Scan(src.Dir)
			files = append(files, local...)
		} else if src.SSH != nil {
			remote, err := logs.ScanSSH(*src.SSH)
			statuses = append(statuses, filelist.SSHStatus{
				Name:      src.Label,
				Connected: err == nil,
			})
			if err != nil {
				files = append(files, logs.LogFile{
					Name: fmt.Sprintf("[%s] (connect error: %v)", src.Label, err),
					SSH:  src.SSH,
				})
			} else {
				files = append(files, remote...)
			}
		}
	}
	return files, statuses
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case sourcepicker.ConfirmMsg:
		files, statuses := scanSources(msg.Sources)
		m.selectedSources = msg.Sources
		m.filelist = filelist.New(m.dir, files, statuses)
		m.filelist, _ = m.filelist.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		m.screen = screenList
		return m, nil

	case filelist.OpenFileMsg:
		m.viewer = viewer.New(msg.File, m.width, m.height)
		m.screen = screenViewer
		return m, nil

	case filelist.GrepStartMsg:
		m.filelist, _ = m.filelist.Update(msg)
		m.viewer = viewer.NewVirtual("shell: "+msg.Pattern+" [running…]", "", m.width, m.height)
		m.screen = screenViewer
		return m, waitForChunk(msg.Ch, msg.Pattern)

	case filelist.GrepChunkMsg:
		if m.screen == screenViewer {
			m.viewer.Append(msg.Content)
			return m, waitForChunk(msg.Ch, msg.Pattern)
		}
		return m, nil

	case filelist.GrepDoneMsg:
		if m.screen == screenViewer {
			title := fmt.Sprintf("grep: %s — %d match(es)", msg.Pattern, msg.Total)
			if msg.Total == 0 {
				m.viewer.Append(fmt.Sprintf("no matches for %q\n", msg.Pattern))
				title = fmt.Sprintf("grep: %s — no matches", msg.Pattern)
			}
			m.viewer.SetTitle(title)
		}
		return m, nil

	case viewer.BackMsg:
		m.screen = screenList
		return m, nil

	case tea.KeyMsg:
		switch m.screen {
		case screenSourcePicker:
			if msg.String() == "q" {
				return m, tea.Quit
			}
		case screenList:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "ctrl+r":
				// Go back to source picker to re-select sources.
				m.screen = screenSourcePicker
				return m, nil
			}
		}
	}

	switch m.screen {
	case screenSourcePicker:
		var cmd tea.Cmd
		m.sourcePicker, cmd = m.sourcePicker.Update(msg)
		return m, cmd
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
	case screenSourcePicker:
		return m.sourcePicker.View()
	case screenList:
		return m.filelist.View()
	case screenViewer:
		return m.viewer.View()
	}
	return ""
}
