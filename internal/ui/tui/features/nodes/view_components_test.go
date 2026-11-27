package nodes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	_ "github.com/mattn/go-runewidth"
)

// cardPointAt 返回第 groupIndex 张卡片内第 lineOffset 行（0=上边框）上的可点击坐标。
func cardPointAt(state PageState, groupIndex, lineOffset int) (int, int) {
	l := calcCardLayout(state, cardAreaHeight(state))
	b := l.Boxes[groupIndex]
	x := b.X + 3
	y := l.AreaStartY + b.Y + lineOffset - l.ScrollRows
	return x, y
}

func cardTestState(width, height int) PageState {
	return PageState{
		GroupNames:     []string{"Proxy", "Netflix"},
		SelectedGroup:  0,
		Groups:         testGroups(),
		CurrentProxies: []string{"Hong Kong 1", "Tokyo 1"},
		SelectedProxy:  0,
		Proxies:        testProxies(),
		Width:          width,
		Height:         height,
	}
}

func TestRenderCardArea_LinesHaveExactWidth(t *testing.T) {
	for _, width := range []int{78, 130, 200} {
		state := cardTestState(width, 32)
		area := renderCardArea(state, cardAreaHeight(state))
		for i, line := range strings.Split(area, "\n") {
			if got := lipgloss.Width(line); got != width {
				t.Fatalf("width=%d line %d has display width %d, want %d", width, i, got, width)
			}
		}
	}
}

func TestCalcCardLayout_AdaptsColumnCount(t *testing.T) {
	for _, tt := range []struct {
		width   int
		columns int
	}{
		{78, 1},
		{130, 2},
		{200, 3},
	} {
		state := cardTestState(tt.width, 32)
		l := calcCardLayout(state, cardAreaHeight(state))
		if l.Columns != tt.columns {
			t.Fatalf("width=%d: columns=%d, want %d", tt.width, l.Columns, tt.columns)
		}
		total := 0
		for _, w := range l.ColumnWidth {
			total += w
		}
		total += cardsColumnGap * (l.Columns - 1)
		if total != tt.width {
			t.Fatalf("width=%d: column widths sum to %d", tt.width, total)
		}
	}
}

func TestResolveMouseHit_GroupAndProxyCards(t *testing.T) {
	i18n.Init()
	state := PageState{
		GroupNames:     []string{"g1", "g2", "g3"},
		SelectedGroup:  0,
		CurrentProxies: []string{"p1", "p2", "p3", "p4"},
		SelectedProxy:  0,
		Groups: map[string]model.Group{
			"g1": {Type: "Selector", Now: "p1", All: []string{"p1", "p2", "p3", "p4"}},
			"g2": {Type: "Selector", Now: "p1", All: []string{"p1"}},
			"g3": {Type: "Selector", Now: "p1", All: []string{"p1"}},
		},
		Width:  130,
		Height: 30,
	}

	// 选中组 g1 展开：节点网格首行命中第一个节点
	x, y := cardPointAt(state, 0, 3)
	if hit := ResolveMouseHit(state, x, y); hit.Target != MouseTargetProxy || hit.Index != 0 {
		t.Fatalf("expected proxy index 0, got target=%v index=%d", hit.Target, hit.Index)
	}

	// 点击折叠卡片 g2 的标题行，命中策略组
	x, y = cardPointAt(state, 1, 1)
	if hit := ResolveMouseHit(state, x, y); hit.Target != MouseTargetGroup || hit.Index != 1 {
		t.Fatalf("expected group index 1, got target=%v index=%d", hit.Target, hit.Index)
	}
}

func TestResolveMouseHit_GridScrollKeepsIndex(t *testing.T) {
	names := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		names = append(names, fmt.Sprintf("node-%02d", i))
	}
	state := PageState{
		GroupNames:     []string{"g1"},
		SelectedGroup:  0,
		CurrentProxies: names,
		SelectedProxy:  36,
		Groups:         map[string]model.Group{"g1": {Type: "Selector", Now: names[0], All: names}},
		Width:          130,
		Height:         30,
	}
	l := calcCardLayout(state, cardAreaHeight(state))
	b := l.Boxes[0]
	if b.GridScrollRow == 0 {
		t.Fatalf("expected grid to scroll for selected proxy 36, got scrollRow=0")
	}

	// 命中可见网格首行第一列，索引应等于滚动后的首个节点
	x := b.X + 3
	y := l.AreaStartY + b.DataStartY - l.ScrollRows
	hit := ResolveMouseHit(state, x, y)
	if hit.Target != MouseTargetProxy || hit.Index != b.GridScrollRow*b.GridCols {
		t.Fatalf("expected proxy index %d, got target=%v index=%d",
			b.GridScrollRow*b.GridCols, hit.Target, hit.Index)
	}
}

func TestRenderNodesPage_WideScreenRendersCardsSideBySide(t *testing.T) {
	i18n.Init()
	state := cardTestState(130, 32)
	lines := strings.Split(RenderNodesPage(state), "\n")
	var sideBySide bool
	for _, line := range lines {
		if strings.Contains(line, "Proxy") && strings.Contains(line, "Netflix") {
			sideBySide = true
			break
		}
	}
	if !sideBySide {
		t.Fatalf("expected two group cards on the same row on wide screens")
	}
}

func TestRenderNodesPage_NarrowScreenStacksCards(t *testing.T) {
	i18n.Init()
	state := cardTestState(78, 28)
	lines := strings.Split(RenderNodesPage(state), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Proxy") && strings.Contains(line, "Netflix") {
			t.Fatalf("expected stacked cards on narrow screens, got combined line %q", line)
		}
	}
}

func testGroups() map[string]model.Group {
	return map[string]model.Group{
		"Proxy":   {Type: "Selector", Now: "Hong Kong 1"},
		"Netflix": {Type: "Selector", Now: "Tokyo 1"},
	}
}

func testProxies() map[string]model.Proxy {
	return map[string]model.Proxy{
		"Hong Kong 1": {History: []model.Delay{{Delay: 36}}},
		"Tokyo 1":     {History: []model.Delay{{Delay: 97}}},
	}
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		found    bool
	}{
		{"Netflix #2ac3de", "#2ac3de", true},
		{"#f7768e", "#f7768e", true},
		{"DIRECT", "", false},
		{"My #Special #2ac3de Node", "#2ac3de", true},
		{"No #12345 Color", "", false},
	}
	for _, tt := range tests {
		color, ok := parseHexColor(tt.input)
		if ok != tt.found {
			t.Errorf("parseHexColor(%q) ok = %v, want %v", tt.input, ok, tt.found)
		}
		if string(color) != tt.expected {
			t.Errorf("parseHexColor(%q) color = %q, want %q", tt.input, string(color), tt.expected)
		}
	}
}

func TestModeBarDisplaysClickableLayoutsAtRight(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, lang := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(lang)
		const width = 120
		bar := RenderModeSwitchComponent("rule", width, LayoutColumns)
		lines := strings.Split(ansi.Strip(bar), "\n")
		for _, line := range lines {
			if lipgloss.Width(line) != width {
				t.Fatalf("%s: mode bar has incorrect width: %q", lang, line)
			}
		}
		row := lines[1]
		modes, layouts := modeSwitchLabels(), layoutSwitchLabels()
		if len(layouts) != 2 || layouts[0] != "⊞" || layouts[1] != "◫" {
			t.Fatalf("%s: unexpected layout icons: %#v", lang, layouts)
		}
		if strings.Contains(row, i18n.T("nodes.layout_cards")) || strings.Contains(row, i18n.T("nodes.layout_columns")) {
			t.Fatalf("%s: expected icon-only layout switcher, got text in %q", lang, row)
		}
		wantRight := " " + strings.Join(layouts, " │ ") + " │"
		if !strings.HasSuffix(row, wantRight) || !strings.Contains(row, strings.Join(modes, " │ ")) {
			t.Fatalf("%s: expected left mode and right layout icons: %q", lang, row)
		}
		for i, label := range layouts {
			at := strings.LastIndex(row, label)
			startX := lipgloss.Width(row[:at]) - 1
			cellW := lipgloss.Width(" " + label + " ")
			for offset := 0; offset < cellW; offset++ {
				hit := resolveModeBarMouseHit(width, startX+offset, 1)
				if hit.Target != MouseTargetLayout || hit.Index != i {
					t.Fatalf("%s: click on %q offset %d mapped to %+v", lang, label, offset, hit)
				}
			}
		}
	}
}

func TestColumnLayoutStacksBelowWideBreakpoint(t *testing.T) {
	i18n.Init()
	for _, dims := range [][2]int{{130, 32}, {78, 28}, {78, 16}} {
		width, height := dims[0], dims[1]
		state := cardTestState(width, height)
		state.Layout = LayoutColumns
		areaH := cardAreaHeight(state)
		metrics := calcColumnsLayoutMetrics(width, areaH)
		lines := strings.Split(ansi.Strip(RenderNodesPage(state)), "\n")
		group := metrics.DataStartY
		node := metrics.DataStartY
		if width < nodesWideBreakpoint {
			node = metrics.stackedProxyY(len(state.GroupNames)) + 2
		}
		if group >= len(lines) || node >= len(lines) {
			t.Fatalf("%dx%d: node/group rows missing at %d/%d: %q", width, height, node, group, lines)
		}
		groupRow := ansi.Cut(lines[group], 0, metrics.GroupPanelWidth)
		nodeRow := ansi.Cut(lines[node], metrics.ProxyPanelX, width)
		at := strings.Index(nodeRow, "Hong Kong 1")
		if !strings.Contains(groupRow, "Proxy") || at < 0 {
			t.Fatalf("%dx%d: expected group and node in their panels: %q / %q", width, height, groupRow, nodeRow)
		}
		nodeX := metrics.ProxyPanelX + lipgloss.Width(nodeRow[:at])
		if hit := ResolveMouseHit(state, nodeX, node); hit.Target != MouseTargetProxy || hit.Index != 0 {
			t.Fatalf("node row at %dx%d mapped to %+v", width, height, hit)
		}
	}
}

func TestNodesSearchReplacesModeBarAndKeepsWidth(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	for _, lang := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(lang)
		for _, width := range []int{32, 80, 120} {
			state := cardTestState(width, 28)
			state.TotalProxies = 5
			state.FilterMode = true
			state.FilterText = "HK"
			state.FilterEngine = FilterEngineFuzzy
			state.ActiveChain = service.ProxyChain{Enabled: true, Nodes: []string{"HK-01", "SG-01"}}

			out := ansi.Strip(RenderNodesPage(state))
			lines := strings.Split(out, "\n")
			for i := 0; i < 3; i++ {
				if lipgloss.Width(lines[i]) != width {
					t.Fatalf("%s width=%d header line %d width=%d, want %d: %q", lang, width, i, lipgloss.Width(lines[i]), width, lines[i])
				}
			}
			headerRow := lines[1]
			if !strings.Contains(headerRow, i18n.T("nodes.search")) || !strings.Contains(headerRow, "HK") {
				t.Fatalf("%s width=%d expected search prompt in top bar: %q", lang, width, headerRow)
			}
			if strings.Contains(headerRow, "⊞") || strings.Contains(headerRow, "◫") || strings.Contains(headerRow, i18n.T("nodes.mode_rule")) {
				t.Fatalf("%s width=%d expected mode/layout controls hidden during search: %q", lang, width, headerRow)
			}
			bottom := lines[len(lines)-1]
			if strings.Contains(bottom, i18n.T("nodes.search")) || strings.Contains(bottom, "HK█") {
				t.Fatalf("%s width=%d bottom line must not render search text: %q", lang, width, bottom)
			}
			if !strings.Contains(bottom, "⛓") || (width >= 80 && !strings.Contains(bottom, "HK-01")) {
				t.Fatalf("%s width=%d bottom line should still render active chain summary: %q", lang, width, bottom)
			}
		}
	}
}
