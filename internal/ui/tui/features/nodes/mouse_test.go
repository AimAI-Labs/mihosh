package nodes

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

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

	// width=120 是宽屏布局，DataStartY=6 (模式切换3行 + 间距1行 + 面板标题2行)
	// Y=6 是第一条策略组数据（g1），Y=7 是第二条（g2）
	next, cmd := state.HandleMouseLeft(4, 7, 120, 24, nil)
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

	// 模式切换占 3 行，CalcNodesListMaxLines(24) -> availableHeight=11, groupMaxLines=3
	// groupListStart = 4 + 2 = 6
	// groupListLines = 1 + 1 = 2 (header + 1 group)
	// proxyHeaderStart = 6 + 2 + 1 = 9
	// proxyListStart = 9 + 2 = 11
	// proxyDataStart = 11 + 1 = 12
	next, cmd := state.HandleMouseLeft(8, 12, 78, 24, nil)
	if cmd != nil {
		t.Fatalf("expected nil cmd for first click")
	}

	next, cmd = next.HandleMouseLeft(8, 12, 78, 24, nil)
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

	// 鼠标在非列表区域（模式切换区域），但焦点在策略组，应滚动策略组
	groupFocused := state.HandleMouseScroll(true, 0, 0, 120, 24)
	if groupFocused.SelectedProxy != 1 {
		t.Fatalf("expected SelectedProxy unchanged when group focused, got %d", groupFocused.SelectedProxy)
	}

	state.MouseFocus = nodesMouseFocusProxy
	// 鼠标在非列表区域（模式切换区域），但焦点在节点，应滚动节点
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

	// 鼠标在策略组区域 (Y=8, groupDataStart = 4 + 2 + 1 = 7, 第二行 Y=8)
	next := state.HandleMouseScroll(true, 10, 8, 120, 24)
	if next.SelectedGroup != 0 {
		t.Fatalf("expected SelectedGroup to be 0 after scroll up, got %d", next.SelectedGroup)
	}

	next = state.HandleMouseScroll(false, 10, 8, 120, 24)
	if next.SelectedGroup != 2 {
		t.Fatalf("expected SelectedGroup to be 2 after scroll down, got %d", next.SelectedGroup)
	}
}
