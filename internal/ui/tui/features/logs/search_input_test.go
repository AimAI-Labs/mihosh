package logs

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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

func TestLogSearchBlankClickEnginesAndEscapeFlow(t *testing.T) {
	i18n.Init()
	const width, height = 100, 24
	s := NewState(nil).AppendLogs([]model.LogEntry{
		{Type: "info", Payload: "alpha-01 connection"},
		{Type: "info", Payload: "beta-99 timeout"},
	}, 0)

	// Click blank space between level tabs and pause button to enter search mode.
	barRow := strings.Split(ansi.Strip(renderLevelBar(s.logLevel, width, s.paused, s.pausedNewCount)), "\n")[1]
	pauseIdx := strings.Index(barRow, i18n.T("logs.pause_btn"))
	require.Greater(t, pauseIdx, 10)
	blankX := ansi.StringWidth(barRow[:pauseIdx]) - 8
	s, _ = s.HandleMouseLeft(1, blankX, width, height)
	require.True(t, s.logFilterMode)

	// Toggle Regex (Ctrl+R) and filter.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	for _, r := range "^alpha-[0-9]+" {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	require.Len(t, s.filteredLogIndices, 1)

	// Toggle Fuzzy (Ctrl+F) on same query -> no match, then clear and test fuzzy match.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	s.logFilter = "bt9"
	s.updateFilteredLogs()
	require.Len(t, s.filteredLogIndices, 1)

	// Enter keeps filter active and exits edit mode; clicking search bar re-enters edit mode.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.False(t, s.logFilterMode)
	require.Equal(t, "bt9", s.logFilter)
	s, _ = s.HandleMouseLeft(1, 5, width, height)
	require.True(t, s.logFilterMode)

	// Clicking below header exits edit mode while keeping filter.
	s, _ = s.HandleMouseLeft(logsHeaderLines, 5, width, height)
	require.False(t, s.logFilterMode)
	require.Equal(t, "bt9", s.logFilter)

	// Esc in normal mode clears filter and restores level/pause bar.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.Empty(t, s.logFilter)
	require.Len(t, s.filteredLogIndices, 2)
	require.Contains(t, ansi.Strip(RenderLogsPage(s.ToPageState(width, height))), i18n.T("logs.pause_btn"))
}
