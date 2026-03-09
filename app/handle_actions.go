package app

import (
	"github.com/sachiniyer/agent-factory/keys"
	"github.com/sachiniyer/agent-factory/log"
	"github.com/sachiniyer/agent-factory/session"
	"github.com/sachiniyer/agent-factory/task"
	"github.com/sachiniyer/agent-factory/ui"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handleDefaultKeyPress handles key events in stateDefault (main interaction state).
func (m *home) handleDefaultKeyPress(msg tea.KeyMsg, name keys.KeyName) (tea.Model, tea.Cmd) {
	tw := m.contentPane.TabbedWindow()

	switch name {
	case keys.KeyHelp:
		return m.showHelpScreen(helpTypeGeneral{}, nil)

	// Sidebar navigation
	case keys.KeyUp:
		m.sidebar.Up()
		return m, m.selectionChanged()
	case keys.KeyDown:
		m.sidebar.Down()
		return m, m.selectionChanged()
	case keys.KeyLeft:
		m.sidebar.CollapseSection()
		return m, m.selectionChanged()
	case keys.KeyRight:
		m.sidebar.ExpandSection()
		return m, m.selectionChanged()
	case keys.KeyNextSection:
		m.sidebar.JumpNextSection()
		return m, m.selectionChanged()
	case keys.KeyPrevSection:
		m.sidebar.JumpPrevSection()
		return m, m.selectionChanged()

	// Instance creation
	case keys.KeyPrompt:
		return m.startNewInstance(true)

	case keys.KeyNew:
		// Context-aware: if on Schedules section, create a schedule instead
		if m.sidebar.GetSelection().Kind == ui.SectionSchedules {
			cwd, err := os.Getwd()
			if err != nil {
				cwd = "."
			}
			m.contentPane.SchedulePane().EnterCreateMode(cwd)
			m.contentPane.SetMode(ui.ContentModeSchedules)
			return m, m.selectionChanged()
		}
		return m.startNewInstance(false)

	case keys.KeySchedule:
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		m.contentPane.SchedulePane().EnterCreateMode(cwd)
		m.navigateToSection(ui.SectionSchedules)
		m.contentPane.SetMode(ui.ContentModeSchedules)
		return m, m.selectionChanged()

	case keys.KeyScheduleList:
		m.navigateToSection(ui.SectionSchedules)
		return m, m.selectionChanged()

	case keys.KeyTriggerSchedule:
		if m.sidebar.GetSelection().Kind != ui.SectionSchedules {
			return m, nil
		}
		sp := m.contentPane.SchedulePane()
		if len(sp.GetSchedules()) == 0 {
			return m, m.handleError(fmt.Errorf("no schedules to trigger"))
		}
		m.contentPane.SetMode(ui.ContentModeSchedules)
		sp.SetFocus(true)
		sp.SetPendingTrigger()
		return m, m.handleScheduleTrigger()

	case keys.KeyTasks:
		m.navigateToSection(ui.SectionTodos)
		return m, m.selectionChanged()

	case keys.KeyMicroClaw:
		if m.microclawBridge == nil || !m.microclawBridge.Available() {
			return m, m.handleError(fmt.Errorf("MicroClaw not available — set MICROCLAW_DIR or install microclaw"))
		}
		m.navigateToSection(ui.SectionMicroClaw)
		return m, m.selectionChanged()

	case keys.KeySearch:
		return m.showSearchOverlay()

	case keys.KeyAttach:
		return m.showAttachWorktreeOverlay()

	// Hooks configuration
	case keys.KeyHooks:
		m.navigateToSection(ui.SectionHooks)
		return m, m.selectionChanged()

	// PR actions
	case keys.KeyOpenPR:
		return m.handleOpenPR()
	case keys.KeyCopyPR:
		return m.handleCopyPR()

	// Scrolling
	case keys.KeyShiftUp:
		m.contentPane.ScrollUp()
		return m, m.selectionChanged()
	case keys.KeyShiftDown:
		m.contentPane.ScrollDown()
		return m, m.selectionChanged()

	// Tab cycling (instance mode only)
	case keys.KeyTab:
		if m.contentPane.GetMode() == ui.ContentModeInstance {
			tw.Toggle()
			m.menu.SetActiveTab(tw.GetActiveTab())
			return m, m.selectionChanged()
		}
		return m, nil
	case keys.KeyShiftTab:
		if m.contentPane.GetMode() == ui.ContentModeInstance {
			tw.ToggleBack()
			m.menu.SetActiveTab(tw.GetActiveTab())
			return m, m.selectionChanged()
		}
		return m, nil

	// Instance actions
	case keys.KeyKill:
		return m.handleKill()
	case keys.KeyEnter:
		return m.handleEnter()

	default:
		return m, nil
	}
}

// handleKill handles the kill/delete session action.
func (m *home) handleKill() (tea.Model, tea.Cmd) {
	selected := m.sidebar.GetSelectedInstance()
	if selected == nil || selected.Status == session.Loading {
		return m, nil
	}

	tw := m.contentPane.TabbedWindow()
	killAction := func() tea.Msg {
		tw.CleanupTerminalForInstance(selected.Title)
		m.sidebar.Kill()
		if err := m.storage.DeleteInstance(selected.Title); err != nil {
			log.ErrorLog.Printf("failed to delete instance from storage: %v", err)
		}

		// Auto-move linked board task to "done"
		if board := m.contentPane.KanbanPane().GetBoard(); board != nil {
			if linkedTask := board.FindTaskByInstance(selected.Title); linkedTask != nil {
				if err := board.MoveTask(linkedTask.ID, "done"); err == nil {
					if err := task.SaveBoard(board); err != nil {
						log.ErrorLog.Printf("failed to save board after moving task to done: %v", err)
					}
					m.contentPane.KanbanPane().SetBoard(board)
					m.sidebar.SetTaskCount(board.TaskCount())
				}
			}
		}

		return instanceChangedMsg{}
	}

	message := fmt.Sprintf("[!] Kill session '%s'?", selected.Title)
	return m, m.confirmAction(message, killAction)
}

// handleEnter handles the enter/open key action.
func (m *home) handleEnter() (tea.Model, tea.Cmd) {
	sel := m.sidebar.GetSelection()
	tw := m.contentPane.TabbedWindow()

	// Toggle expandable section headers (only Instances has children)
	if sel.IsHeader && sel.Kind == ui.SectionInstances {
		m.sidebar.ToggleSection()
		return m, m.selectionChanged()
	}
	// Instance selected
	if sel.Kind == ui.SectionInstances {
		selected := m.sidebar.GetSelectedInstance()
		if selected == nil || selected.Status == session.Loading || !selected.TmuxAlive() {
			return m, nil
		}
		if tw.IsInTerminalTab() {
			m.showHelpScreen(helpTypeInstanceAttach{}, func() {
				ch, err := tw.AttachTerminal()
				if err != nil {
					log.ErrorLog.Printf("failed to attach terminal: %v", err)
					return
				}
				<-ch
				m.state = stateDefault
			})
			return m, nil
		}
		m.showHelpScreen(helpTypeInstanceAttach{}, func() {
			ch, err := m.sidebar.Attach()
			if err != nil {
				log.ErrorLog.Printf("failed to attach: %v", err)
				return
			}
			<-ch
			m.state = stateDefault
		})
		return m, nil
	}
	// MicroClaw selected — attach
	if sel.Kind == ui.SectionMicroClaw {
		if m.microclawBridge == nil || !m.microclawBridge.Available() {
			return m, m.handleError(fmt.Errorf("MicroClaw not available"))
		}
		mc := m.contentPane.MicroClawPane()
		if mc == nil {
			return m, nil
		}
		m.showHelpScreen(helpTypeInstanceAttach{}, func() {
			ch, err := mc.Attach()
			if err != nil {
				log.ErrorLog.Printf("failed to attach microclaw: %v", err)
				return
			}
			<-ch
			m.state = stateDefault
		})
		return m, nil
	}
	return m, nil
}

// handleOpenPR opens the PR URL in the browser.
func (m *home) handleOpenPR() (tea.Model, tea.Cmd) {
	selected := m.sidebar.GetSelectedInstance()
	if selected == nil || selected.GetPRInfo() == nil {
		return m, nil
	}
	url := selected.GetPRInfo().URL
	var openCmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		openCmd = exec.Command("open", url)
	} else {
		openCmd = exec.Command("xdg-open", url)
	}
	if err := openCmd.Start(); err != nil {
		return m, m.handleError(fmt.Errorf("failed to open PR: %w", err))
	}
	return m, nil
}

// handleCopyPR copies the PR URL to the clipboard.
func (m *home) handleCopyPR() (tea.Model, tea.Cmd) {
	selected := m.sidebar.GetSelectedInstance()
	if selected == nil || selected.GetPRInfo() == nil {
		return m, nil
	}
	url := selected.GetPRInfo().URL
	var copyCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		copyCmd = exec.Command("pbcopy")
	default:
		if _, err := exec.LookPath("wl-copy"); err == nil {
			copyCmd = exec.Command("wl-copy")
		} else {
			copyCmd = exec.Command("xclip", "-selection", "clipboard")
		}
	}
	copyCmd.Stdin = strings.NewReader(url)
	if err := copyCmd.Run(); err != nil {
		return m, m.handleError(fmt.Errorf("failed to copy PR URL: %w", err))
	}
	return m, nil
}
