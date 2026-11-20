package notice

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestNoticeUsesOnlyTheThemeForeground(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	palette := theme.Current()
	colors := []lipgloss.Color{palette.Info, palette.Success, palette.Warning, palette.Danger}
	for level, color := range colors {
		msg := messages.NoticeMsg{Text: "已保存", Level: messages.NoticeLevel(level)}
		want := " " + lipgloss.NewStyle().Foreground(theme.Resolve(color)).Render(msg.Text)
		if got := Render(msg, 20); got != want {
			t.Fatalf("level %d got %q, want %q", level, got, want)
		}
	}
	long := messages.NoticeMsg{Text: "很长的提醒\n还有一行", Level: messages.NoticeWarning}
	row := ansi.Strip(Render(long, 8))
	if strings.ContainsRune(row, '\n') || lipgloss.Width(row) > 8 || !strings.HasSuffix(row, "…") {
		t.Fatalf("notice does not fit in a single narrow row: %q", row)
	}
}

func TestNoticePriorityAndExpiry(t *testing.T) {
	var s State
	info := messages.NoticeMsg{Text: "sync"}
	warning := messages.NoticeMsg{Text: "restart required", Level: messages.NoticeWarning}
	s.Publish(info)
	old := s.ID
	s.Publish(warning)
	current := s.ID
	s.Publish(warning)
	s.Publish(info)
	s.Expire(old)
	if s.ID != current || s.Message != warning {
		t.Fatalf("a duplicate, lower priority or stale timer replaced %v", warning)
	}
	s.Expire(current)
	if s.Message.Text != "" {
		t.Fatal("the current timer did not clear the warning")
	}
	s.Publish(messages.NoticeMsg{Text: "failed", Level: messages.NoticeError})
	s.OpenDetails(25, 40)
	if s.DetailsOpen {
		t.Fatal("click outside the notice opened its details")
	}
	s.OpenDetails(1, 40)
	s.Expire(s.ID)
	if !s.DetailsOpen || s.ExpiryCmd() != nil || s.Message.Detail != "failed" {
		t.Fatal("error details must remain accessible until dismissed")
	}
}
