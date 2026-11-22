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
	require.Equal(t, "◉", ansi.Cut(plain, layout.IPStartX, layout.IPEndX))
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
	require.Equal(t, "◎", ansi.Cut(plain, layout.IPStartX, layout.IPEndX))
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

func TestLayoutCurrentIPModal_WideShowsAllFieldsAndHitBoxes(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	modal := LayoutCurrentIPModal(CurrentIPBarState{
		Info:      sampleCurrentIP(),
		Countdown: 195,
	}, 100, 30, 0)

	view := ansi.Strip(modal.Dialog.View())
	require.Contains(t, view, "IP 信息")
	require.Contains(t, view, "[JP] Japan")
	require.Contains(t, view, "203.0.113.42 ◉")
	require.Contains(t, view, "AS2514")
	require.Contains(t, view, "Unknown")
	require.Contains(t, view, "NTT PC Communications, Inc.")
	require.Contains(t, view, "Tokyo")
	require.Contains(t, view, "Asia/Tokyo")
	require.Contains(t, view, "自动刷新: 195s")
	require.Contains(t, view, "JP, 35.68, 139.68")

	left, top, right, bottom := modal.Dialog.Bounds()
	require.Greater(t, right, left)
	require.Greater(t, bottom, top)

	lines := strings.Split(view, "\n")
	require.GreaterOrEqual(t, modal.RefreshX, 0)
	require.Equal(t, "↻", ansi.Cut(lines[top+modal.RefreshY], left+modal.RefreshX, left+modal.RefreshX+modal.RefreshWidth))
	require.GreaterOrEqual(t, modal.MaskX, 0)
	require.Equal(t, "203.0.113.42 ◉", ansi.Cut(lines[top+modal.MaskY], left+modal.MaskX, left+modal.MaskX+modal.MaskWidth))

	// Masked state updates IP and mask hit box cleanly.
	maskedModal := LayoutCurrentIPModal(CurrentIPBarState{
		Info:      sampleCurrentIP(),
		Masked:    true,
		Countdown: 195,
	}, 100, 30, 0)
	maskedView := ansi.Strip(maskedModal.Dialog.View())
	require.NotContains(t, maskedView, "203.0.113.42")
	mLeft, mTop, _, _ := maskedModal.Dialog.Bounds()
	mLines := strings.Split(maskedView, "\n")
	require.Equal(t, maskedIPPlaceholder+" ◎", ansi.Cut(mLines[mTop+maskedModal.MaskY], mLeft+maskedModal.MaskX, mLeft+maskedModal.MaskX+maskedModal.MaskWidth))
}

func TestLayoutCurrentIPModal_NarrowAndErrorStatesFitWithoutOverflow(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	ipv6Info := sampleCurrentIP()
	ipv6Info.IP = "2001:0db8:85a3:0000:0000:8a2e:0370:7334"

	states := []CurrentIPBarState{
		{Info: sampleCurrentIP(), Countdown: 195},
		{Info: ipv6Info, Countdown: 120},
		{Info: sampleCurrentIP(), Loading: true},
		{Loading: true},
		{Err: "ip.sb: dial tcp 127.0.0.1:7890: connect: connection refused (proxy unreachable)", Countdown: 30},
		{},
	}

	for _, lang := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(lang)
		for _, st := range states {
			for _, size := range [][2]int{{36, 22}, {48, 24}, {64, 24}, {100, 30}} {
				w, h := size[0], size[1]
				modal := LayoutCurrentIPModal(st, w, h, 0)
				left, top, right, bottom := modal.Dialog.Bounds()
				require.GreaterOrEqual(t, left, 0)
				require.GreaterOrEqual(t, top, 0)
				require.LessOrEqual(t, right, w)
				require.LessOrEqual(t, bottom, h)

				view := ansi.Strip(modal.Dialog.View())
				for _, line := range strings.Split(view, "\n") {
					require.LessOrEqual(t, lipgloss.Width(line), w, "lang=%s size=%dx%d line overflow", lang, w, h)
				}
			}
		}
	}

	// Narrow screen (48 cols) still displays full organization, timezone, and coordinates without clipping.
	i18n.SetLanguageOverride("zh-CN")
	narrow := LayoutCurrentIPModal(CurrentIPBarState{
		Info:      sampleCurrentIP(),
		Countdown: 195,
	}, 48, 24, 0)
	narrowView := ansi.Strip(narrow.Dialog.View())
	require.Contains(t, narrowView, "NTT PC Communications, Inc.")
	require.Contains(t, narrowView, "Asia/Tokyo")
	require.Contains(t, narrowView, "JP, 35.68, 139.68")

	// Short viewport enables scrolling to reach all lines.
	short := LayoutCurrentIPModal(CurrentIPBarState{
		Info:      sampleCurrentIP(),
		Countdown: 195,
	}, 44, 10, 0)
	require.Greater(t, short.MaxScroll, 0)
	scrolled := LayoutCurrentIPModal(CurrentIPBarState{
		Info:      sampleCurrentIP(),
		Countdown: 195,
	}, 44, 10, short.MaxScroll)
	require.Contains(t, ansi.Strip(scrolled.Dialog.View()), "Asia/Tokyo")
}
