package components

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func sampleCurrentIP() *model.IPInfo {
	return &model.IPInfo{
		IP:              "203.0.113.42",
		Country:         "Japan",
		CountryCode:     "JP",
		RegionName:      "Tokyo",
		City:            "Tokyo",
		ASN:             2514,
		AS:              "AS2514",
		ISP:             "Unknown",
		Organization:    "NTT PC Communications, Inc.",
		ASNOrganization: "NTT PC Communications, Inc.",
		Timezone:        "Asia/Tokyo",
		Latitude:        35.68,
		Longitude:       139.68,
	}
}

func TestLayoutCurrentIPBar_WideShowsAllClashVergeFields(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	layout := LayoutCurrentIPBar(CurrentIPBarState{
		Info:      sampleCurrentIP(),
		Countdown: 195,
	}, 140)

	plain := ansi.Strip(layout.Content)
	require.Equal(t, 1, lipgloss.Height(layout.Content))
	require.LessOrEqual(t, lipgloss.Width(layout.Content), 140)
	require.Contains(t, plain, "[JP] Japan")
	require.Contains(t, plain, "203.0.113.42 ◉")
	require.Contains(t, plain, "AS2514")
	require.Contains(t, plain, "Tokyo")
	require.NotContains(t, plain, "Tokyo, Tokyo")
	require.Contains(t, plain, "NTT PC Communications, Inc.")
	require.NotContains(t, plain, "Unknown")
	require.Contains(t, plain, "Asia/Tokyo")
	require.Contains(t, plain, "35.68, 139.68")
	require.Contains(t, plain, "↻ 195s")

	require.GreaterOrEqual(t, layout.IPStartX, 0)
	require.Equal(t, "203.0.113.42 ◉", ansi.Cut(plain, layout.IPStartX, layout.IPEndX))
	require.GreaterOrEqual(t, layout.RefreshStart, 0)
	require.Equal(t, "↻ 195s", ansi.Cut(plain, layout.RefreshStart, layout.RefreshEnd))
}

func TestLayoutCurrentIPBar_MaskedHidesIPAndAlignsHitBox(t *testing.T) {
	i18n.Init()
	layout := LayoutCurrentIPBar(CurrentIPBarState{
		Info:      sampleCurrentIP(),
		Masked:    true,
		Countdown: 240,
	}, 120)

	plain := ansi.Strip(layout.Content)
	require.NotContains(t, plain, "203.0.113.42")
	require.Contains(t, plain, maskedIPPlaceholder+" ◎")
	require.Equal(t, maskedIPPlaceholder+" ◎", ansi.Cut(plain, layout.IPStartX, layout.IPEndX))
	require.Equal(t, "↻ 240s", ansi.Cut(plain, layout.RefreshStart, layout.RefreshEnd))
}

func TestLayoutCurrentIPBar_SingleLineAcrossAllWidthsAndStates(t *testing.T) {
	i18n.Init()
	states := []CurrentIPBarState{
		{Info: sampleCurrentIP(), Countdown: 195},
		{Info: sampleCurrentIP(), Masked: true, Countdown: 42},
		{Info: sampleCurrentIP(), Loading: true},
		{Loading: true},
		{Err: "connection refused", Countdown: 30},
		{},
	}

	for _, lang := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(lang)
		for _, st := range states {
			for _, width := range []int{20, 32, 50, 72, 80, 100, 120, 160} {
				layout := LayoutCurrentIPBar(st, width)
				require.False(t, strings.Contains(layout.Content, "\n"), "lang=%s width=%d must be single line", lang, width)
				require.LessOrEqual(t, lipgloss.Width(layout.Content), width, "lang=%s width=%d overflowed", lang, width)
				if layout.IPStartX >= 0 {
					require.Less(t, layout.IPStartX, layout.IPEndX)
					require.LessOrEqual(t, layout.IPEndX, width)
				}
				if layout.RefreshStart >= 0 {
					require.Less(t, layout.RefreshStart, layout.RefreshEnd)
					require.LessOrEqual(t, layout.RefreshEnd, width)
				}
			}
		}
	}
	i18n.SetLanguageOverride("")
}
