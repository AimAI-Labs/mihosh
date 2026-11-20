package sub

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderedSubscriptionRowsMatchClicksAfterResize(t *testing.T) {
	var subs []profile.Profile
	for i := 0; i < 40; i++ {
		subs = append(subs, profile.Profile{UID: fmt.Sprint(i), Name: fmt.Sprintf("sub-%02d", i)})
	}
	for _, viewport := range []struct{ width, columns int }{{99, 1}, {100, 2}, {149, 2}, {150, 3}, {250, 3}} {
		width := viewport.width
		for _, height := range []int{12, 24, 48} {
			for _, selected := range []int{0, 19, 39} {
				t.Run(fmt.Sprintf("%d/%d/selected=%d", width, height, selected), func(t *testing.T) {
					state := State{}.ApplySubs(subs, "")
					state.selected, state.scrollTop = selected, 15
					ps := state.ToPageState(width, height)
					grid := layoutSubList(ps)
					require.Len(t, grid.Widths, viewport.columns)
					if viewport.columns == 2 {
						require.Equal(t, grid.Widths[0], grid.Widths[1])
					}
					found := 0
					for y, line := range strings.Split(ansi.Strip(RenderSubPage(ps)), "\n") {
						for i, sub := range subs {
							at := strings.Index(line, sub.Name)
							if at < 0 {
								continue
							}
							found++
							x := lipgloss.Width(line[:at])
							if got := grid.ItemAt(x, y-subFixedLines); got != i {
								t.Fatalf("rendered sub[%d] at y=%d resolves to %d", i, y, got)
							}
							next, _ := state.HandleMouseLeft(x, y, width, height)
							if next.selected != i {
								t.Fatalf("click at rendered sub[%d] selected %d", i, next.selected)
							}
						}
					}
					require.Positive(t, found)
				})
			}
		}
	}
}

func TestRenderSubPage_EmptyList(t *testing.T) {
	state := State{}.ToPageState(80, 24)
	out := RenderSubPage(state)
	assert.NotEmpty(t, out)
	// 空列表应显示空提示
	assert.Contains(t, out, i18n.T("sub.empty"))
}

func TestRenderSubPage_WithSubs(t *testing.T) {
	subs := sampleSubs()
	state := State{}.ApplySubs(subs, "a1").ToPageState(100, 30)
	out := RenderSubPage(state)
	require.NotEmpty(t, out)

	// 头部统计行应出现
	assert.Contains(t, out, i18n.Tf("sub.stats", len(subs)), "应渲染统计信息")

	// 订阅名称应出现在渲染结果中（原样透传）
	assert.Contains(t, out, "订阅A")
	assert.Contains(t, out, "订阅B")
}

func TestRenderSubCardUsageFitsViewport(t *testing.T) {
	up, down := int64(1024), int64(3072)
	p := profile.Profile{
		Name:   "订阅名称带有emoji 🌍 与很长的文本",
		Source: profile.SubSource{Kind: profile.SourceRemote, URL: "https://example.com/sub?token=hidden"},
		Usage:  &profile.SubscriptionUsage{UsedBytes: 4096, TotalBytes: 8192, ExpiresAt: 2000000000, UploadBytes: &up, DownloadBytes: &down},
	}
	for _, width := range []int{40, 99, 100} {
		card := ansi.Strip(renderSubCard(p, true, false, true, width, subCardHeight))
		rows := strings.Split(card, "\n")
		require.Len(t, rows, subCardHeight)
		for _, row := range rows {
			require.Equal(t, width, common.DisplayWidth(row))
		}
		assert.NotContains(t, card, common.SymbolCheck)
		assert.NotContains(t, card, common.SymbolSelectActive)
		assert.Contains(t, card, "example.com")
		assert.NotContains(t, card, "token")
		assert.Contains(t, card, utils.FormatBytes(4096)+" / "+utils.FormatBytes(8192))
		assert.Contains(t, card, "2033")
		assert.Contains(t, card, i18n.Tf("sub.card.remaining", utils.FormatBytes(4096)))
		assert.Contains(t, card, "↑"+utils.FormatBytes(up))
		assert.Contains(t, card, "↓"+utils.FormatBytes(down))
		half := (width - 4) / 2
		assert.Contains(t, card, strings.Repeat("━", half)+strings.Repeat("─", width-4-half))
	}
}

func TestSubscriptionActiveBorderSurvivesSelection(t *testing.T) {
	colorProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(colorProfile) })
	palette := theme.Current()
	profile := sampleSubs()[0]
	for _, tc := range []struct {
		active   bool
		selected bool
		color    lipgloss.Color
	}{{false, false, palette.Border}, {false, true, palette.Primary}, {true, false, palette.Success}, {true, true, palette.Success}} {
		card := renderSubCard(profile, tc.active, false, tc.selected, 48, subCardHeight)
		corner := lipgloss.NewStyle().Foreground(theme.Resolve(tc.color)).Render(theme.Frame().TopLeft)
		require.True(t, strings.HasPrefix(card, strings.TrimSuffix(corner, ansi.SGR(0))), card)
	}
}

func TestSubscriptionCardUsesThemeForegroundAndRefreshesClickedProfile(t *testing.T) {
	previousTheme, profileMode := theme.CurrentName(), lipgloss.ColorProfile()
	t.Cleanup(func() {
		theme.SetTheme(previousTheme)
		lipgloss.SetColorProfile(profileMode)
	})
	lipgloss.SetColorProfile(termenv.TrueColor)
	subs := sampleSubs()
	state := State{}.ApplySubs(subs, "a1")
	page := state.ToPageState(150, 25)
	grid := layoutSubList(page)
	require.True(t, theme.SetTheme("dracula"))
	dracula := renderList(page, grid)
	require.True(t, theme.SetTheme("solarized-light"))
	solarized := renderList(page, grid)
	require.NotEqual(t, dracula, solarized)
	require.Contains(t, solarized, "38;2;")
	require.NotContains(t, solarized, "48;2;")
	require.NotContains(t, dracula, "48;2;")
	require.Equal(t, ansi.Strip(dracula), ansi.Strip(solarized))

	x, y, width := grid.CardOrigin(2)
	require.Positive(t, x, "the third card should be beside the first")
	state, cmd := state.HandleMouseLeft(x+width-3, y+subFixedLines+1, page.Width, page.Height)
	require.NotNil(t, cmd)
	assert.Equal(t, 2, state.selected)
	assert.Equal(t, page.Subs[2].UID, cmd().(messages.SubFetchErrorMsg).UID)
}

func TestRenderSubPage_AddFormOverlay(t *testing.T) {
	state := PageState{
		Subs:        sampleSubs(),
		FilteredIdx: []int{0, 1, 2},
		Width:       100,
		Height:      30,
		ShowAddForm: true,
		AddForm:     newAddForm(),
	}
	out := RenderSubPage(state)
	assert.NotEmpty(t, out)
}

func TestRenderSubPage_DeleteConfirmOverlay(t *testing.T) {
	state := PageState{
		Subs: []profile.Profile{
			{UID: "a1", Name: "订阅A", Source: profile.SubSource{Kind: profile.SourceRemote, URL: "https://a.io"}},
		},
		FilteredIdx:    []int{0},
		ActiveUID:      "a1",
		Width:          100,
		Height:         30,
		ShowDeleteConf: true,
		DeleteUID:      "a1",
	}
	out := RenderSubPage(state)
	assert.NotEmpty(t, out)
}

func TestSubscriptionGridHitBoundaries(t *testing.T) {
	state := PageState{
		FilteredIdx: []int{0, 1, 2},
		ScrollTop:   0,
		Height:      30,
		Width:       100,
	}
	grid := layoutSubList(state)
	assert.Equal(t, -1, grid.ItemAt(1, -1))
	assert.Equal(t, -1, grid.ItemAt(grid.Widths[0], 0))
	assert.Equal(t, 0, grid.ItemAt(1, 0))
	assert.Equal(t, 1, grid.ItemAt(grid.Widths[0]+common.CardGap, 0))
	assert.Equal(t, 2, grid.ItemAt(1, subCardHeight))
	assert.Equal(t, -1, grid.ItemAt(grid.Widths[0]+common.CardGap, subCardHeight))
}

func TestRelativeTime(t *testing.T) {
	// 未更新（0）应返回 never_updated 标记
	assert.Equal(t, i18n.T("sub.never_updated"), relativeTime(0))
	// 过去时间戳应返回某个相对时间描述（非空）
	assert.NotEmpty(t, relativeTime(1700000000))
}
