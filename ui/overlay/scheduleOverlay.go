package overlay

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ScheduleOverlay represents a schedule task overlay with prompt, cron, and path inputs.
type ScheduleOverlay struct {
	prompt        textarea.Model
	cronInput     textinput.Model
	pathInput     textinput.Model
	FocusIndex    int // 0=prompt, 1=cron, 2=path, 3=submit button
	Submitted     bool
	Canceled      bool
	width, height int
}

// NewScheduleOverlay creates a new schedule overlay with the given default project path.
func NewScheduleOverlay(defaultPath string) *ScheduleOverlay {
	prompt := textarea.New()
	prompt.ShowLineNumbers = false
	prompt.Prompt = ""
	prompt.Focus()
	prompt.FocusedStyle.CursorLine = lipgloss.NewStyle()
	prompt.CharLimit = 0
	prompt.MaxHeight = 0
	prompt.Placeholder = "Enter task prompt..."

	cron := textinput.New()
	cron.Placeholder = "0 9 * * 1-5"
	cron.CharLimit = 64
	cron.Blur()

	path := textinput.New()
	path.SetValue(defaultPath)
	path.CharLimit = 256
	path.Blur()

	return &ScheduleOverlay{
		prompt:     prompt,
		cronInput:  cron,
		pathInput:  path,
		FocusIndex: 0,
		Submitted:  false,
		Canceled:   false,
	}
}

func (s *ScheduleOverlay) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.prompt.SetHeight(height / 3)
}

// HandleKeyPress processes a key press and updates the state accordingly.
// Returns true if the overlay should be closed.
func (s *ScheduleOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyTab:
		s.FocusIndex = (s.FocusIndex + 1) % 4
		s.updateFocus()
		return false
	case tea.KeyShiftTab:
		s.FocusIndex = (s.FocusIndex + 3) % 4
		s.updateFocus()
		return false
	case tea.KeyEsc:
		s.Canceled = true
		return true
	case tea.KeyEnter:
		if s.FocusIndex == 3 {
			s.Submitted = true
			return true
		}
		if s.FocusIndex == 0 {
			s.prompt, _ = s.prompt.Update(msg)
		}
		return false
	default:
		switch s.FocusIndex {
		case 0:
			s.prompt, _ = s.prompt.Update(msg)
		case 1:
			s.cronInput, _ = s.cronInput.Update(msg)
		case 2:
			s.pathInput, _ = s.pathInput.Update(msg)
		}
		return false
	}
}

func (s *ScheduleOverlay) updateFocus() {
	if s.FocusIndex == 0 {
		s.prompt.Focus()
		s.cronInput.Blur()
		s.pathInput.Blur()
	} else if s.FocusIndex == 1 {
		s.prompt.Blur()
		s.cronInput.Focus()
		s.pathInput.Blur()
	} else if s.FocusIndex == 2 {
		s.prompt.Blur()
		s.cronInput.Blur()
		s.pathInput.Focus()
	} else {
		s.prompt.Blur()
		s.cronInput.Blur()
		s.pathInput.Blur()
	}
}

// Render renders the schedule overlay.
func (s *ScheduleOverlay) Render() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Bold(true)

	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	focusedButtonStyle := buttonStyle.
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("0"))

	inputWidth := s.width - 6
	s.prompt.SetWidth(inputWidth)
	s.cronInput.Width = inputWidth
	s.pathInput.Width = inputWidth

	content := titleStyle.Render("Schedule Task") + "\n"
	content += labelStyle.Render("Prompt:") + "\n"
	content += s.prompt.View() + "\n\n"
	content += labelStyle.Render("Cron:") + "  " + s.cronInput.View() + "\n"
	content += labelStyle.Render("Path:") + "  " + s.pathInput.View() + "\n\n"

	submitButton := " Schedule "
	if s.FocusIndex == 3 {
		submitButton = focusedButtonStyle.Render(submitButton)
	} else {
		submitButton = buttonStyle.Render(submitButton)
	}
	content += "       " + submitButton

	return style.Render(content)
}

// GetPrompt returns the current value of the prompt textarea.
func (s *ScheduleOverlay) GetPrompt() string {
	return s.prompt.Value()
}

// GetCronExpr returns the current value of the cron expression input.
func (s *ScheduleOverlay) GetCronExpr() string {
	return s.cronInput.Value()
}

// GetPath returns the current value of the path input.
func (s *ScheduleOverlay) GetPath() string {
	return s.pathInput.Value()
}

// IsSubmitted returns whether the form was submitted.
func (s *ScheduleOverlay) IsSubmitted() bool {
	return s.Submitted
}

// IsCanceled returns whether the form was canceled.
func (s *ScheduleOverlay) IsCanceled() bool {
	return s.Canceled
}
