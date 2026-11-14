package nodes

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
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
