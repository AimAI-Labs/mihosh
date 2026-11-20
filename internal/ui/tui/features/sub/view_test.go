package sub

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderedSubscriptionRowsMatchClicksAfterResize(t *testing.T) {
	var subs []profile.Profile
	for i := 0; i < 40; i++ {
		subs = append(subs, profile.Profile{UID: fmt.Sprint(i), Name: fmt.Sprintf("sub-%02d", i)})
	}
	for _, width := range []int{99, 100} {
		for _, height := range []int{12, 24, 48} {
			for _, selected := range []int{0, 19, 39} {
				t.Run(fmt.Sprintf("%d/%d/selected=%d", width, height, selected), func(t *testing.T) {
					state := State{}.ApplySubs(subs, "")
					state.selected, state.scrollTop = selected, 15
					ps := state.ToPageState(width, height)
					found := 0
					for y, line := range strings.Split(RenderSubPage(ps), "\n") {
						for i, sub := range subs {
							if !strings.Contains(line, sub.Name) {
								continue
							}
							found++
							if got := resolveListItemAt(ps, y, height); got != i {
								t.Fatalf("rendered sub[%d] at y=%d resolves to %d", i, y, got)
							}
							next, _ := state.HandleMouseLeft(1, y, width, height)
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
	p := profile.Profile{
		Name:   "订阅名称带有emoji 🌍 与很长的文本",
		Source: profile.SubSource{Kind: profile.SourceRemote, URL: "https://example.com/sub?token=hidden"},
		Usage:  &profile.SubscriptionUsage{UsedBytes: 4096, TotalBytes: 8192, ExpiresAt: 2000000000},
	}
	for _, width := range []int{40, 99, 100} {
		card := renderSubCard(p, true, false, true, width, subCardHeight)
		rows := strings.Split(card, "\n")
		require.Len(t, rows, subCardHeight)
		for _, row := range rows {
			require.Equal(t, width, common.DisplayWidth(row))
		}
		assert.Contains(t, card, "✓")
		assert.Contains(t, card, "example.com")
		assert.NotContains(t, card, "token")
		assert.Contains(t, card, utils.FormatBytes(4096)+" / "+utils.FormatBytes(8192))
		assert.Contains(t, card, "2033")
		assert.Contains(t, card, strings.Repeat("━", 12)+strings.Repeat("─", 12))
	}
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

func TestResolveListItemAt(t *testing.T) {
	state := PageState{
		FilteredIdx: []int{0, 1, 2},
		ScrollTop:   0,
		Height:      30,
	}
	// 列表起始行 = subHeaderHeight + 1 = 4
	assert.Equal(t, -1, resolveListItemAt(state, 0, 30), "header 区域不应命中")
	assert.Equal(t, -1, resolveListItemAt(state, 3, 30), "header 分隔行不应命中")
	assert.Equal(t, 0, resolveListItemAt(state, 4, 30), "第一项应命中")
	assert.Equal(t, 1, resolveListItemAt(state, 4+subCardHeight, 30), "第二项应命中")
}

func TestRelativeTime(t *testing.T) {
	// 未更新（0）应返回 never_updated 标记
	assert.Equal(t, i18n.T("sub.never_updated"), relativeTime(0))
	// 过去时间戳应返回某个相对时间描述（非空）
	assert.NotEmpty(t, relativeTime(1700000000))
}
