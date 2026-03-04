package nanoclaw

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	tuiSenderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)
	tuiTimestampStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#808080", Dark: "#808080"})
	tuiBotStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#36CFC9"))
	tuiMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})
	tuiStatusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Bold(true)
	tuiHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#808080", Dark: "#808080"})
)

// refreshMsg signals the TUI to re-fetch messages.
type refreshMsg struct{}

// tuiModel is the bubbletea model for the interactive nanoclaw TUI.
type tuiModel struct {
	bridge   *Bridge
	group    Group
	meta     *MessageMeta
	messages []Message
	status   string
	viewport viewport.Model
	input    textarea.Model
	width    int
	height   int
	err      error
	ready    bool
}

// NewTUIModel creates a new interactive TUI model for a nanoclaw group.
func NewTUIModel(bridge *Bridge, group Group, meta *MessageMeta) tuiModel {
	ti := textarea.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 2000
	ti.SetHeight(3)
	ti.Focus()

	return tuiModel{
		bridge:   bridge,
		group:    group,
		meta:     meta,
		input:    ti,
		viewport: viewport.New(0, 0),
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.refreshCmd(),
	)
}

func (m tuiModel) refreshCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return refreshMsg{}
	})
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			// Submit message (Shift+Enter for newline is handled by textarea)
			text := strings.TrimSpace(m.input.Value())
			if text != "" {
				if err := m.bridge.SendMessage(m.group.Folder, text, m.meta); err != nil {
					m.err = err
				} else {
					m.input.Reset()
					m.fetchMessages()
					m.renderMessages()
				}
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Layout: status(1) + separator(1) + viewport + input(5) + help(1)
		inputHeight := 5
		headerHeight := 2
		helpHeight := 1
		vpHeight := m.height - inputHeight - headerHeight - helpHeight
		if vpHeight < 1 {
			vpHeight = 1
		}

		m.viewport.Width = m.width
		m.viewport.Height = vpHeight
		m.input.SetWidth(m.width - 2)

		m.fetchMessages()
		m.renderMessages()
		return m, nil

	case refreshMsg:
		m.fetchMessages()
		m.renderMessages()
		return m, m.refreshCmd()
	}

	// Forward to textarea
	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	cmds = append(cmds, inputCmd)

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) fetchMessages() {
	msgs, err := m.bridge.GetMessagesForGroup(m.group.Folder, 100)
	if err != nil {
		m.err = err
		return
	}
	m.err = nil
	m.messages = msgs

	status, err := m.bridge.Status()
	if err == nil {
		m.status = status
	}
}

func (m *tuiModel) renderMessages() {
	if m.width == 0 {
		return
	}

	var sb strings.Builder

	if len(m.messages) == 0 {
		sb.WriteString("\n  No messages yet.\n")
	} else {
		for _, msg := range m.messages {
			ts := formatTS(msg.Timestamp)
			sender := msg.SenderName
			if sender == "" {
				sender = msg.Sender
			}

			sStyle := tuiSenderStyle
			if msg.IsBotMessage == 1 || msg.IsFromMe == 1 {
				sStyle = tuiBotStyle.Bold(true)
			}

			header := sStyle.Render(sender) + " " + tuiTimestampStyle.Render(ts)
			sb.WriteString(header + "\n")

			style := tuiMsgStyle
			if msg.IsFromMe == 1 || msg.IsBotMessage == 1 {
				style = tuiBotStyle
			}

			wrapped := wrapTextTUI(msg.Content, m.width-2)
			sb.WriteString(style.Render("  "+strings.ReplaceAll(wrapped, "\n", "\n  ")) + "\n\n")
		}
	}

	m.viewport.SetContent(sb.String())
	m.viewport.GotoBottom()
}

func (m tuiModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	var sections []string

	// Status bar
	statusLine := tuiStatusStyle.Render(fmt.Sprintf("NanoClaw — %s — %s", m.group.Name, m.status))
	sections = append(sections, statusLine)
	sections = append(sections, strings.Repeat("─", m.width))

	// Messages viewport
	sections = append(sections, m.viewport.View())

	// Input area
	sections = append(sections, m.input.View())

	// Help line
	help := tuiHelpStyle.Render("enter: send | ctrl-c: quit")
	if m.err != nil {
		help = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(fmt.Sprintf("Error: %v", m.err))
	}
	sections = append(sections, help)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func formatTS(ts string) string {
	if len(ts) >= 16 {
		return ts[5:16]
	}
	return ts
}

func wrapTextTUI(text string, width int) string {
	if width <= 0 {
		return text
	}
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if len(line) <= width {
			lines = append(lines, line)
			continue
		}
		for len(line) > width {
			cut := width
			for i := width; i > 0; i-- {
				if line[i] == ' ' {
					cut = i
					break
				}
			}
			lines = append(lines, line[:cut])
			line = line[cut:]
			if len(line) > 0 && line[0] == ' ' {
				line = line[1:]
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// RunTUI starts the interactive nanoclaw TUI.
func RunTUI(bridge *Bridge, group Group, meta *MessageMeta) error {
	p := tea.NewProgram(
		NewTUIModel(bridge, group, meta),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
