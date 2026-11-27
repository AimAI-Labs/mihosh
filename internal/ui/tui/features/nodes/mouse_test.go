package nodes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// stateCardPoint 返回第 groupIndex 张卡片内第 lineOffset 行的可点击坐标。
func stateCardPoint(s State, w, h, groupIndex, lineOffset int) (int, int) {
	return cardPointAt(s.ToPageState(w, h), groupIndex, lineOffset)
}

func TestHandleMouseLeft_GroupSelectionKeepsLinkage(t *testing.T) {
	state := State{
		Groups: map[string]model.Group{
			"g1": {All: []string{"p1", "p2"}},
			"g2": {All: []string{"p3"}},
		},
		GroupNames:    []string{"g1", "g2"},
		SelectedGroup: 0,
	}
	state.updateCurrentProxies()

	// 点击第二张（折叠）卡片的标题行
	x, y := stateCardPoint(state, 120, 24, 1, 1)
	next, cmd := state.HandleMouseLeft(x, y, 120, 24)
	if cmd != nil {
		t.Fatalf("expected nil cmd for single group click")
	}
	if next.SelectedGroup != 1 {
		t.Fatalf("expected SelectedGroup=1, got %d", next.SelectedGroup)
	}
	if len(next.CurrentProxies) != 1 || next.CurrentProxies[0] != "p3" {
		t.Fatalf("expected CurrentProxies linked to g2, got %#v", next.CurrentProxies)
	}
}

func TestHandleMouseLeft_ProxyDoubleClickExecutesSwitch(t *testing.T) {
	state := State{
		Groups: map[string]model.Group{
			"g1": {All: []string{"p1", "p2"}},
		},
		GroupNames:    []string{"g1"},
		SelectedGroup: 0,
	}
	state.updateCurrentProxies()

	// 点击展开卡片内节点网格首行
	x, y := stateCardPoint(state, 78, 24, 0, 3)
	next, cmd := state.HandleMouseLeft(x, y, 78, 24)
	if cmd != nil {
		t.Fatalf("expected nil cmd for first click")
	}

	_, cmd = next.HandleMouseLeft(x, y, 78, 24)
	if cmd == nil {
		t.Fatalf("expected non-nil cmd for proxy double click")
	}
}

func TestHandleMouseScroll_RespectsMouseFocus(t *testing.T) {
	state := State{
		CurrentProxies: []string{"p1", "p2", "p3"},
		SelectedProxy:  1,
		MouseFocus:     nodesMouseFocusGroup,
	}

	// 鼠标在模式切换区域且焦点在策略组，应滚动策略组而不是节点
	groupFocused := state.HandleMouseScroll(true, 0, 0, 120, 24)
	if groupFocused.SelectedProxy != 1 {
		t.Fatalf("expected SelectedProxy unchanged when group focused, got %d", groupFocused.SelectedProxy)
	}

	state.MouseFocus = nodesMouseFocusProxy
	// 焦点在节点，应移动节点选中项
	proxyFocused := state.HandleMouseScroll(true, 0, 0, 120, 24)
	if proxyFocused.SelectedProxy != 0 {
		t.Fatalf("expected SelectedProxy moved when proxy focused, got %d", proxyFocused.SelectedProxy)
	}
}

func TestHandleMouseScroll_GroupArea(t *testing.T) {
	state := State{
		GroupNames:    []string{"g1", "g2", "g3"},
		SelectedGroup: 1,
	}
	state.updateCurrentProxies()

	// 鼠标在第一张（折叠）卡片的标题行
	x, y := stateCardPoint(state, 120, 24, 0, 1)
	next := state.HandleMouseScroll(true, x, y, 120, 24)
	if next.SelectedGroup != 0 {
		t.Fatalf("expected SelectedGroup to be 0 after scroll up, got %d", next.SelectedGroup)
	}

	next = state.HandleMouseScroll(false, x, y, 120, 24)
	if next.SelectedGroup != 2 {
		t.Fatalf("expected SelectedGroup to be 2 after scroll down, got %d", next.SelectedGroup)
	}
}

func TestHandleMouseLeft_GroupHeaderDelayIconTriggersGroupBatchTest(t *testing.T) {
	state := State{
		Groups: map[string]model.Group{
			"g1": {Type: "Selector", Now: "p1", All: []string{"p1", "p2"}},
			"g2": {Type: "Selector", Now: "p3", All: []string{"p3", "p4"}},
		},
		Proxies: map[string]model.Proxy{
			"p1": {History: []model.Delay{{Delay: 42}}},
		},
		GroupNames:    []string{"g1", "g2"},
		SelectedGroup: 0,
	}
	state.updateCurrentProxies()

	const w, h = 120, 24
	ps := state.ToPageState(w, h)
	l := calcCardLayout(ps, cardAreaHeight(ps))

	// Click delay number "42" on expanded group g1.
	b0 := l.Boxes[0]
	x0 := b0.X + 2 + b0.InnerWidth - 1
	y0 := l.AreaStartY + b0.Y + 1 - l.ScrollRows
	next, cmd := state.HandleMouseLeft(x0, y0, w, h)
	if cmd == nil || !next.Testing || !next.batchMode {
		t.Fatalf("clicking delay badge on expanded group did not start batch test: testing=%v batch=%v cmd=%v", next.Testing, next.batchMode, cmd != nil)
	}
	if _, total := next.testBatch.Progress(); total != 2 {
		t.Fatalf("expected g1 batch total=2, got %d", total)
	}

	// Click lightning icon "⚡" on collapsed group g2 without switching expanded group.
	b1 := l.Boxes[1]
	x1 := b1.X + 2 + b1.InnerWidth - 1
	y1 := l.AreaStartY + b1.Y + 1 - l.ScrollRows
	next, cmd = state.HandleMouseLeft(x1, y1, w, h)
	if cmd == nil || !next.Testing || !next.batchMode {
		t.Fatalf("clicking lightning icon on collapsed group did not start batch test: testing=%v batch=%v cmd=%v", next.Testing, next.batchMode, cmd != nil)
	}
	if next.SelectedGroup != 0 {
		t.Fatalf("expected SelectedGroup to remain 0, got %d", next.SelectedGroup)
	}
	if _, total := next.testBatch.Progress(); total != 2 {
		t.Fatalf("expected g2 batch total=2, got %d", total)
	}
}

func TestNodesLayoutSwitchesWithTopBarAndKeyboard(t *testing.T) {
	state := State{SelectedGroup: 1, SelectedProxy: 2}
	const width, height = 120, 30
	state, cmd := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	if cmd != nil || state.ToPageState(width, height).Layout != LayoutColumns {
		t.Fatal("v did not select the column layout")
	}
	for _, target := range []NodesLayout{LayoutCards, LayoutColumns} {
		clicked := false
		for x := width - 1; x > 0; x-- {
			hit := ResolveMouseHit(state.ToPageState(width, height), x, 1)
			if hit.Target == MouseTargetLayout && hit.Index == int(target) {
				state, cmd = state.HandleMouseLeft(x, 1, width, height)
				clicked = true
				break
			}
		}
		if !clicked || cmd != nil || state.Layout != target || state.SelectedGroup != 1 || state.SelectedProxy != 2 {
			t.Fatalf("top bar did not select layout %d and retain selections", target)
		}
	}
	state, _ = state.Update(messages.CoreTargetChangedMsg{Reset: true})
	if state.Layout != LayoutColumns {
		t.Fatal("changing the core target reset the chosen layout")
	}
}

func TestColumnsMouseSelectionAndIndependentScroll(t *testing.T) {
	for _, width := range []int{78, 130} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			const height = 22
			names := make([]string, 30)
			groupNames := make([]string, 30)
			groups := make(map[string]model.Group)
			for i := range names {
				names[i] = fmt.Sprintf("node-%02d", i)
				groupNames[i] = fmt.Sprintf("group-%02d", i)
			}
			for _, group := range groupNames {
				groups[group] = model.Group{Type: "Selector", All: names}
			}
			state := State{Layout: LayoutColumns}.ApplyGroups(groups, groupNames)
			page := state.ToPageState(width, height)
			metrics := calcColumnsLayoutMetrics(width, cardAreaHeight(page))

			state, cmd := state.HandleMouseLeft(3, metrics.DataStartY+1, width, height)
			if cmd != nil || state.SelectedGroup != 1 || len(state.CurrentProxies) != len(names) {
				t.Fatal("group row click did not update the node list")
			}
			for range metrics.GroupMaxLines {
				state = state.HandleMouseScroll(false, 3, metrics.AreaStartY+1, width, height)
			}
			if state.GroupScrollTop == 0 || state.SelectedProxy != 0 {
				t.Fatal("scrolling over the group panel did not move its window independently")
			}
			first := ResolveMouseHit(state.ToPageState(width, height), 3, metrics.DataStartY)
			if first.Target != MouseTargetGroup || first.Index != state.GroupScrollTop {
				t.Fatalf("scrolled group row maps to %+v, want %d", first, state.GroupScrollTop)
			}

			proxyX, proxyY := metrics.ProxyPanelX+3, metrics.DataStartY
			if !metrics.Wide {
				proxyY = metrics.stackedProxyY(len(groupNames)) + 2
			}
			selectedGroup := state.SelectedGroup
			for range metrics.ProxyMaxLines {
				state = state.HandleMouseScroll(false, proxyX, proxyY-1, width, height)
			}
			if state.ProxyScrollTop == 0 || state.SelectedGroup != selectedGroup {
				t.Fatal("scrolling over the node panel moved the group selection")
			}
			node := ResolveMouseHit(state.ToPageState(width, height), proxyX, proxyY)
			if node.Target != MouseTargetProxy || node.Index != state.ProxyScrollTop {
				t.Fatalf("scrolled proxy row maps to %+v, want %d", node, state.ProxyScrollTop)
			}
			state, cmd = state.HandleMouseLeft(proxyX, proxyY, width, height)
			if cmd != nil || state.SelectedProxy != node.Index {
				t.Fatal("first click did not select the visible node")
			}
			_, cmd = state.HandleMouseLeft(proxyX, proxyY, width, height)
			if cmd == nil {
				t.Fatal("double click did not dispatch proxy selection")
			}

			state = state.ApplyGroups(nil, nil)
			if len(state.CurrentProxies) != 0 || state.SelectedGroup != 0 || state.SelectedProxy != 0 ||
				state.GroupScrollTop != 0 || state.ProxyScrollTop != 0 {
				t.Fatal("old row positions survived a refresh with no groups")
			}
		})
	}
}

func TestNodesClickModeBarBlankEntersSearchAndToggleFocus(t *testing.T) {
	i18n.Init()
	const width, height = 100, 24
	state := State{
		Mode:       "rule",
		GroupNames: []string{"g1"},
		Groups:     map[string]model.Group{"g1": {Type: "Selector", Now: "HK-01", All: []string{"HK-01", "SG-01"}}},
	}
	state.updateCurrentProxies()

	// Find blank cell between Direct and ⊞ in mode bar row 1.
	barRow := strings.Split(ansi.Strip(RenderModeSwitchComponent(state.Mode, width, state.Layout)), "\n")[1]
	idx := strings.Index(barRow, "⊞")
	if idx < 0 {
		t.Fatalf("failed to locate ⊞ in mode bar: %q", barRow)
	}
	blankX := ansi.StringWidth(barRow[:idx]) - 3
	if blankX <= 0 {
		t.Fatalf("failed to locate blank gap in mode bar: %q", barRow)
	}
	next, _ := state.HandleMouseLeft(blankX, 1, width, height)
	if !next.NodeFilterMode {
		t.Fatal("clicking blank space in mode bar did not enter NodeFilterMode")
	}

	// Type filter and confirm with Enter; top bar stays in search state (unfocused).
	for _, r := range "hk" {
		next, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if next.NodeFilterMode || next.NodeFilter != "hk" {
		t.Fatalf("unexpected filter state after Enter: mode=%v filter=%q", next.NodeFilterMode, next.NodeFilter)
	}

	// Clicking search bar re-enters edit mode.
	next, _ = next.HandleMouseLeft(5, 1, width, height)
	if !next.NodeFilterMode {
		t.Fatal("clicking filtered search bar did not re-enter NodeFilterMode")
	}

	// Clicking below header exits edit mode without selecting cards/changing group.
	next, _ = next.HandleMouseLeft(5, 8, width, height)
	if next.NodeFilterMode || next.NodeFilter != "hk" {
		t.Fatalf("clicking below header should exit edit mode and keep filter, got mode=%v filter=%q", next.NodeFilterMode, next.NodeFilter)
	}

	// Escape clears filter and restores mode/layout bar.
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if next.NodeFilter != "" {
		t.Fatalf("expected Esc to clear NodeFilter, got %q", next.NodeFilter)
	}
	restored := ansi.Strip(RenderNodesPage(next.ToPageState(width, height)))
	if !strings.Contains(restored, "⊞") {
		t.Fatalf("expected mode/layout bar restored after clearing search:\n%s", restored)
	}
}
