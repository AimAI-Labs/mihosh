package connections

import (
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestUnlockCardsFitNarrowAndWideViews(t *testing.T) {
	timestamp := time.Date(2026, 6, 1, 20, 30, 15, 0, time.Local)
	view := unlockView{items: []unlockItem{
		{name: "Netflix", result: model.UnlockResult{Status: model.UnlockAvailable, Region: "JP", CheckedAt: timestamp}},
		{name: "Spotify"},
		{name: "YouTube Premium"},
	}}
	for width, columns := range map[int]int{38: 1, 80: 2, 120: 3} {
		popup, layout := view.popup(width, 24)
		require.Len(t, layout.grid.Widths, columns)
		screen := ansi.Strip(popup.View())
		require.Contains(t, screen, i18n.T("conns.unlock.available"))
		require.Contains(t, screen, "[JP]")
		require.Contains(t, screen, "2026-06-01 20:30:15")
		for _, row := range strings.Split(screen, "\n") {
			require.LessOrEqual(t, lipgloss.Width(row), width)
		}
	}
}
