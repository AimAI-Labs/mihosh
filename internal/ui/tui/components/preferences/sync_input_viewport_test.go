package preferences

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestSyncInputKeepsCursorVisibleAndMapsMouseToText(t *testing.T) {
	value := strings.Repeat("宽", 35) + "tail"
	line := ansi.Strip(renderModalInputLine(value, len([]rune(value)), 24, true))
	require.Contains(t, line, "tail▏")
	const width, height = 40, 24
	s := State{syncModal: SyncModalState{Open: true, Mode: syncModePGConfig, DSNValue: value, FormCursor: len([]rune(value))}}
	modal := buildSyncModal(s.ToPageState(nil, nil), width, height)
	left, top, _, _ := modal.Bounds()
	next, _ := s.handleSyncModalClick(left+3, top+modal.bodyTop+1, width, height)
	require.Greater(t, next.syncModal.FormCursor, 0, "the click should hit the visible end of the field")
	require.Less(t, next.syncModal.FormCursor, len([]rune(value)))
}

func TestBusySyncCanCloseAndReopenWithoutStartingAnotherAction(t *testing.T) {
	s := State{syncModal: SyncModalState{Open: true, Mode: syncModeImportPath, Busy: true}}
	require.False(t, s.InputCapture().Interrupt, "Ctrl+C should reach global quit")
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.False(t, s.syncModal.Open)
	require.True(t, s.syncModal.Busy)
	s = s.OpenSyncModal()
	require.True(t, s.syncModal.Open)
	require.True(t, s.syncModal.Busy)
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	s = s.CompleteSync(messages.SyncDoneMsg{Summary: "done"})
	require.False(t, s.syncModal.Busy)
	require.Equal(t, "done", s.syncModal.StatusText)
}
