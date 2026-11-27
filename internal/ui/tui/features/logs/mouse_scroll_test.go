package logs

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestLogMouseUsesRenderedWindowAfterKeyScroll(t *testing.T) {
	entries := make([]model.LogEntry, 40)
	for i := range entries {
		entries[i] = model.LogEntry{Type: "info", Payload: fmt.Sprintf("log-row-%02d", i)}
	}
	s := NewState(nil).AppendLogs(entries, 0)
	for i := 0; i < 25; i++ {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	const width, height, firstY = 100, 18, logsModeSwitchHeight + 5
	const capacity = height - logsFixedLines
	page := s.ToPageState(width, height)
	want := page.SelectedLog - capacity + 1
	require.Positive(t, want)
	payload := page.Logs[page.FilteredLogIndices[want]].Payload
	lines := strings.Split(ansi.Strip(RenderLogsPage(page)), "\n")
	require.Contains(t, lines[firstY], payload)

	click := messages.PageMouseClickMsg{X: 2, Y: firstY + capacity, Width: width, Height: height}
	after, _ := s.Update(click)
	require.Equal(t, s.selectedLog, after.selectedLog)

	click.Y = firstY
	s, _ = s.Update(click)
	require.Equal(t, want, s.selectedLog)
	s, _ = s.Update(click)
	require.NotNil(t, s.detailLog)
	require.Equal(t, payload, s.detailLog.Payload)
}
