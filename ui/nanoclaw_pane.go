package ui

import (
	"claude-squad/log"
	"claude-squad/nanoclaw"
	"claude-squad/session/tmux"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var ncPaneStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

var ncFooterStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#808080", Dark: "#808080"})

// NanoClawPane manages a tmux session running the interactive nanoclaw TUI.
// It captures the tmux pane output for display in the tabbed window and
// supports attaching for full interactive use.
type NanoClawPane struct {
	mu           sync.Mutex
	bridge       *nanoclaw.Bridge
	tmuxSession  *tmux.TmuxSession
	width        int
	height       int
	content      string
	fallback     bool
	fallbackText string
	isScrolling  bool
	viewport     viewport.Model
}

// NewNanoClawPane creates a new pane backed by the given bridge.
func NewNanoClawPane(bridge *nanoclaw.Bridge) *NanoClawPane {
	return &NanoClawPane{
		bridge:   bridge,
		viewport: viewport.New(0, 0),
	}
}

func (p *NanoClawPane) SetSize(width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.width = width
	p.height = height
	p.viewport.Width = width
	p.viewport.Height = height
	if p.tmuxSession != nil {
		if err := p.tmuxSession.SetDetachedSize(width, height); err != nil {
			log.InfoLog.Printf("nanoclaw pane: failed to set detached size: %v", err)
		}
	}
}

// setFallbackState sets the pane to display a fallback message.
// Caller must hold p.mu.
func (p *NanoClawPane) setFallbackState(message string) {
	p.fallback = true
	p.fallbackText = lipgloss.JoinVertical(lipgloss.Center, FallBackText, "", message)
	p.content = ""
}

// Refresh captures the tmux pane output for display. Creates the session if needed.
func (p *NanoClawPane) Refresh() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.bridge == nil || !p.bridge.Available() {
		p.setFallbackState("NanoClaw not available.\nSet NANOCLAW_DIR or install at ~/nanoclaw.")
		return
	}

	// Skip content updates while in scroll mode
	if p.isScrolling {
		return
	}

	// Ensure the tmux session is running
	if err := p.ensureSessionLocked(); err != nil {
		p.setFallbackState(fmt.Sprintf("Failed to start NanoClaw session: %v", err))
		return
	}

	if p.tmuxSession == nil || !p.tmuxSession.DoesSessionExist() {
		p.setFallbackState("NanoClaw session not available.")
		return
	}

	content, err := p.tmuxSession.CapturePaneContent()
	if err != nil {
		p.setFallbackState(fmt.Sprintf("Failed to capture content: %v", err))
		return
	}

	p.fallback = false
	p.content = content
}

// ensureSessionLocked creates the tmux session running `cs nanoclaw` if it doesn't exist.
// Caller must hold p.mu.
func (p *NanoClawPane) ensureSessionLocked() error {
	if p.tmuxSession != nil && p.tmuxSession.DoesSessionExist() {
		return nil
	}

	// Find the binary path — use the running executable
	binary, err := os.Executable()
	if err != nil {
		binary = os.Args[0]
	}

	cmd := binary + " nanoclaw"
	ts := tmux.NewTmuxSession("nanoclaw_tui", cmd)

	// Check if session already exists (e.g. from a previous run)
	if ts.DoesSessionExist() {
		if err := ts.Restore(); err != nil {
			_ = ts.Close()
			ts = tmux.NewTmuxSession("nanoclaw_tui", cmd)
			if err := ts.Start("."); err != nil {
				return fmt.Errorf("failed to start nanoclaw session: %w", err)
			}
		}
	} else {
		if err := ts.Start("."); err != nil {
			return fmt.Errorf("failed to start nanoclaw session: %w", err)
		}
	}

	p.tmuxSession = ts

	if p.width > 0 && p.height > 0 {
		if err := ts.SetDetachedSize(p.width, p.height); err != nil {
			log.InfoLog.Printf("nanoclaw pane: failed to set size: %v", err)
		}
	}

	return nil
}

// Attach attaches to the nanoclaw tmux session for full interactive use.
func (p *NanoClawPane) Attach() (chan struct{}, error) {
	p.mu.Lock()
	if p.tmuxSession == nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("no nanoclaw session to attach to")
	}
	if !p.tmuxSession.DoesSessionExist() {
		p.mu.Unlock()
		return nil, fmt.Errorf("nanoclaw session does not exist")
	}
	ts := p.tmuxSession
	p.mu.Unlock()
	return ts.Attach()
}

// Close kills the nanoclaw tmux session.
func (p *NanoClawPane) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tmuxSession != nil {
		if err := p.tmuxSession.Close(); err != nil {
			log.InfoLog.Printf("nanoclaw pane: failed to close session: %v", err)
		}
		p.tmuxSession = nil
	}
	p.content = ""
	p.fallback = false
	p.fallbackText = ""
}

func (p *NanoClawPane) ScrollUp() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isScrolling {
		p.enterScrollMode()
		return
	}
	p.viewport.LineUp(1)
}

func (p *NanoClawPane) ScrollDown() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isScrolling {
		p.enterScrollMode()
		return
	}
	p.viewport.LineDown(1)
}

func (p *NanoClawPane) enterScrollMode() {
	if p.tmuxSession == nil || !p.tmuxSession.DoesSessionExist() {
		return
	}

	content, err := p.tmuxSession.CapturePaneContentWithOptions("-", "-")
	if err != nil {
		log.InfoLog.Printf("nanoclaw pane: failed to capture full history: %v", err)
		return
	}

	footer := ncFooterStyle.Render("ESC to exit scroll mode")
	contentWithFooter := lipgloss.JoinVertical(lipgloss.Left, content, footer)
	p.viewport.SetContent(contentWithFooter)
	p.viewport.GotoBottom()
	p.isScrolling = true
}

// IsScrolling returns whether the nanoclaw pane is in scroll mode.
func (p *NanoClawPane) IsScrolling() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.isScrolling
}

// ResetToNormalMode exits scroll mode.
func (p *NanoClawPane) ResetToNormalMode() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isScrolling {
		return
	}
	p.isScrolling = false
	p.viewport.SetContent("")
	p.viewport.GotoTop()
}

func (p *NanoClawPane) String() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	width := p.width
	height := p.height

	if width == 0 || height == 0 {
		return strings.Repeat("\n", height)
	}

	if p.isScrolling {
		return p.viewport.View()
	}

	if p.fallback {
		availableHeight := height - 3 - 4
		fallbackLines := len(strings.Split(p.fallbackText, "\n"))
		totalPadding := availableHeight - fallbackLines
		topPadding := 0
		bottomPadding := 0
		if totalPadding > 0 {
			topPadding = totalPadding / 2
			bottomPadding = totalPadding - topPadding
		}

		var lines []string
		if topPadding > 0 {
			lines = append(lines, strings.Repeat("\n", topPadding))
		}
		lines = append(lines, p.fallbackText)
		if bottomPadding > 0 {
			lines = append(lines, strings.Repeat("\n", bottomPadding))
		}

		return ncPaneStyle.
			Width(width).
			Align(lipgloss.Center).
			Render(strings.Join(lines, ""))
	}

	// Normal mode: show captured content
	lines := strings.Split(p.content, "\n")

	if height > 0 {
		if len(lines) > height {
			lines = lines[len(lines)-height:]
		} else {
			padding := height - len(lines)
			lines = append(lines, make([]string, padding)...)
		}
	}

	contentStr := strings.Join(lines, "\n")
	return ncPaneStyle.Width(width).Render(contentStr)
}
