package connections

import (
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"
)

func TestUnlockCardsFitNarrowAndWideViews(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })

	timestamp := time.Date(2026, 6, 1, 20, 30, 15, 0, time.Local)
	view := unlockView{items: []unlockItem{
		{name: "Netflix", result: model.UnlockResult{Status: model.UnlockAvailable, Region: "JP", CheckedAt: timestamp}},
		{name: "Spotify"},
		{name: "YouTube Premium"},
	}}
	frame := theme.Frame()
	for _, viewport := range []struct{ width, height, columns int }{{20, 5, 0}, {38, 24, 1}, {80, 24, 2}, {120, 24, 3}} {
		popup, layout := view.popup(viewport.width, viewport.height)
		require.Len(t, layout.grid.Widths, viewport.columns)
		rendered := popup.Overlay("", true)
		require.Contains(t, rendered, "38;2;")
		require.NotContains(t, rendered, "48;2;")
		screen := ansi.Strip(rendered)
		if viewport.columns > 0 {
			require.Contains(t, screen, i18n.T("conns.unlock.available"))
			require.Contains(t, screen, "[JP]")
			require.Contains(t, screen, "2026-06-01 20:30:15")
		}
		rows := strings.Split(screen, "\n")
		left, top, right, bottom := popup.Bounds()
		require.True(t, strings.HasPrefix(ansi.Cut(rows[top], left, right), frame.TopLeft))
		require.True(t, strings.HasSuffix(ansi.Cut(rows[bottom-1], left, right), frame.BottomRight))
		for y := top + 1; y < bottom-1; y++ {
			line := ansi.Cut(rows[y], left, right)
			require.True(t, strings.HasPrefix(line, frame.Left), line)
			require.True(t, strings.HasSuffix(line, frame.Right), line)
		}
		for _, row := range rows {
			require.LessOrEqual(t, lipgloss.Width(row), viewport.width)
		}
	}
}
