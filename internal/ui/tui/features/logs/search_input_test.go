package logs

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestLogSearchAcceptsUnicodePasteAndBackspace(t *testing.T) {
	s := NewState(nil).AppendLogs([]model.LogEntry{{Type: "info", Payload: "连接成功"}, {Type: "info", Payload: "ordinary log"}}, 0)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("连接成"), Paste: true})
	require.Equal(t, "连接成", s.logFilter)
	require.Len(t, s.filteredLogIndices, 1)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Equal(t, "连接", s.logFilter)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("成")})
	require.Equal(t, "连接成", s.logFilter)
	require.Len(t, s.filteredLogIndices, 1)
}
