package notice

import (
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const Height = 1

// State owns the global notice and the lifetime of its details dialog.
type State struct {
	Message     messages.NoticeMsg
	ID          uint64
	DetailsOpen bool
}

func (s *State) Publish(msg messages.NoticeMsg) {
	if msg.Level == messages.NoticeError && msg.Detail == "" {
		msg.Detail = msg.Text
	}
	if strings.TrimSpace(msg.Text) == "" || s.Message == msg || (s.Message.Text != "" && s.Message.Level > msg.Level) {
		return
	}
	s.ID++
	s.Message = msg
	s.DetailsOpen = false
}

func (s *State) Clear() {
	s.Message = messages.NoticeMsg{}
	s.DetailsOpen = false
}

func (s State) ExpiryCmd() tea.Cmd {
	if s.Message.Text == "" || s.Message.Level == messages.NoticeError {
		return nil
	}
	duration := 3 * time.Second
	if s.Message.Level == messages.NoticeWarning {
		duration = 5 * time.Second
	}
	id := s.ID
	return tea.Tick(duration, func(time.Time) tea.Msg {
		return messages.NoticeExpiredMsg{ID: id}
	})
}

func (s *State) Expire(id uint64) {
	if s.ID == id && s.Message.Level != messages.NoticeError {
		s.Clear()
	}
}

func (s *State) OpenDetails(x, width int) {
	if s.Message.Detail != "" && x >= 1 && x < 1+lipgloss.Width(line(s.Message.Text, width)) {
		s.DetailsOpen = true
	}
}

func line(text string, width int) string {
	return ansi.Truncate(strings.Join(strings.Fields(ansi.Strip(text)), " "), max(0, width-1), "…")
}

func Render(msg messages.NoticeMsg, width int) string {
	palette := theme.Current()
	color := palette.Info
	switch msg.Level {
	case messages.NoticeSuccess:
		color = palette.Success
	case messages.NoticeWarning:
		color = palette.Warning
	case messages.NoticeError:
		color = palette.Danger
	}
	return " " + lipgloss.NewStyle().Foreground(theme.Resolve(color)).Render(line(msg.Text, width))
}
