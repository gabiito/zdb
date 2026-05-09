package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ClearStatusMsg is sent by the auto-clear timer. Exported so core.App can type-alias it.
type ClearStatusMsg struct{}

// StatusBarModel shows transient messages in the status bar.
// Messages auto-clear after 6 seconds.
type StatusBarModel struct {
	msg    string
	isErr  bool
	width  int
}

// SetMsg sets a non-error status message.
func (s *StatusBarModel) SetMsg(text string) tea.Cmd {
	s.msg = text
	s.isErr = false
	return s.autoClearCmd()
}

// SetErr sets an error message.
func (s *StatusBarModel) SetErr(err error) tea.Cmd {
	if err == nil {
		s.msg = ""
		s.isErr = false
		return nil
	}
	s.msg = err.Error()
	s.isErr = true
	return s.autoClearCmd()
}

// ErrCmd returns a tea.Cmd that emits a clearStatusMsg after setting the error.
func (s *StatusBarModel) ErrCmd(err error) tea.Cmd {
	return s.SetErr(err)
}

// ClearMsg handles the auto-clear tick.
func (s *StatusBarModel) ClearMsg() {
	s.msg = ""
	s.isErr = false
}

// SetWidth updates the width for rendering.
func (s *StatusBarModel) SetWidth(w int) { s.width = w }

// View renders the status bar.
func (s *StatusBarModel) View() string {
	if s.msg == "" {
		return StyleStatusBar.Width(s.width).Render("")
	}
	style := StyleStatusBar
	if s.isErr {
		style = StyleStatusBarError
	}
	msg := s.msg
	// Truncate to fit width
	maxLen := s.width - 2
	if maxLen > 0 && len(msg) > maxLen {
		msg = msg[:maxLen-1] + "…"
	}
	return style.Width(s.width).Render(msg)
}

// Update handles status bar messages.
func (s *StatusBarModel) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case ClearStatusMsg:
		s.ClearMsg()
	}
	return nil
}

func (s *StatusBarModel) autoClearCmd() tea.Cmd {
	return tea.Tick(6*time.Second, func(_ time.Time) tea.Msg {
		return ClearStatusMsg{}
	})
}

// Msg returns the current status bar text (for testing).
func (s *StatusBarModel) Msg() string { return s.msg }

// IsErr returns whether the current message is an error.
func (s *StatusBarModel) IsErr() bool { return s.isErr }

// StatusMsg creates a non-error status message command.
func StatusMsg(text string) tea.Cmd {
	return func() tea.Msg {
		return StatusSetMsg{Text: text, IsErr: false}
	}
}

// ErrStatusMsg creates an error status message command.
func ErrStatusMsg(format string, args ...any) tea.Cmd {
	return func() tea.Msg {
		return StatusSetMsg{Text: fmt.Sprintf(format, args...), IsErr: true}
	}
}

// StatusSetMsg is a message for the status bar. Exported so core.App can type-alias it.
type StatusSetMsg struct {
	Text  string
	IsErr bool
}
