package sub

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSubPage_EmptyList(t *testing.T) {
	state := State{}.ToPageState(80, 24)
	out := RenderSubPage(state)
	assert.NotEmpty(t, out)
	// 空列表应显示空提示
	assert.Contains(t, out, "sub.empty") // i18n key 在无 locale 加载时回退为键名
}

func TestRenderSubPage_WithSubs(t *testing.T) {
	subs := sampleSubs()
	state := State{}.ApplySubs(subs, "a1").ToPageState(100, 30)
	out := RenderSubPage(state)
	require.NotEmpty(t, out)

	// 头部统计行应出现
	assert.True(t, strings.Contains(out, "sub.stats") || strings.Contains(out, "sub.stats_remote"),
		"应渲染统计信息")

	// 订阅名称应出现在渲染结果中（原样透传）
	assert.Contains(t, out, "订阅A")
	assert.Contains(t, out, "订阅B")
}

// TestRenderSubEntry_RowWidth 防止列宽计算回归：每行整行（含选择符前缀）的显示
// 宽度必须严格等于列表区宽度（width - subScrollWidth），且只能渲染为单行，
// 否则会溢出滚动条列或破坏列对齐。
func TestRenderSubEntry_RowWidth(t *testing.T) {
	listWidth := 80 - subScrollWidth

	cases := []struct {
		name string
		prof profile.Profile
	}{
		{
			name: "long url",
			prof: profile.Profile{UID: "a1", Name: "订阅A", Source: profile.SubSource{Kind: profile.SourceRemote, URL: "https://example.com/long/path/sub.yaml"}},
		},
		{
			name: "short url",
			prof: profile.Profile{UID: "a1", Name: "订阅A", Source: profile.SubSource{Kind: profile.SourceRemote, URL: "https://example.com/sub"}},
		},
		{
			name: "long name",
			prof: profile.Profile{UID: "a1", Name: "这是一个非常非常长的订阅名称用来测试截断逻辑", Source: profile.SubSource{Kind: profile.SourceLocal, URL: "/local/path"}},
		},
	}

	for _, tc := range cases {
		for _, selected := range []bool{false, true} {
			entry := renderSubEntry(tc.prof, 1, false, true, false, selected, listWidth)
			// 1. 不允许换行：单行布局，换行会破坏列对齐
			assert.NotContains(t, entry, "\n", "%s selected=%v: 行不应换行", tc.name, selected)
			// 2. 整行宽度严格等于列表区宽度
			assert.Equal(t, listWidth, common.DisplayWidth(entry),
				"%s selected=%v: 整行宽度必须等于列表区宽度 %d", tc.name, selected, listWidth)
		}
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
	assert.Equal(t, 1, resolveListItemAt(state, 5, 30), "第二项应命中")
}

func TestRelativeTime(t *testing.T) {
	// 未更新（0）应返回 never_updated 标记
	assert.Contains(t, relativeTime(0), "never_updated")
	// 过去时间戳应返回某个相对时间描述（非空）
	assert.NotEmpty(t, relativeTime(1700000000))
}
