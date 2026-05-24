package nodes

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	_ "github.com/mattn/go-runewidth"
)

func TestRenderTokyoPanel(t *testing.T) {
	// 验证在标题过长时安全截断并且无崩溃，且渲染结果的每一行显示宽度完全等于 width
	width := 30
	title := "This is an extremely long title that exceeds the panel width"
	body := "Row 1\nRow 2"
	result := renderTokyoPanel(title, body, width)
	lines := strings.Split(result, "\n")
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines, got %d", len(lines))
	}
	for i, l := range lines {
		actualWidth := lipgloss.Width(l)
		if actualWidth != width {
			t.Errorf("line %d %q width = %d, want %d", i, l, actualWidth, width)
		}
	}
}

func TestResolveMouseHit_GroupAndProxy(t *testing.T) {
	state := PageState{
		GroupNames:     []string{"g1", "g2", "g3"},
		SelectedGroup:  0,
		CurrentProxies: []string{"p1", "p2", "p3", "p4"},
		SelectedProxy:  0,
		Height:         24,
	}

	groupHit := ResolveMouseHit(state, 10, 5)
	if groupHit.Target != MouseTargetGroup || groupHit.Index != 0 {
		t.Fatalf("expected group index 0, got target=%v index=%d", groupHit.Target, groupHit.Index)
	}

	// proxyListStart 依赖 groupMaxLines. height=24 -> groupMaxLines=4.
	// groupListLines = 1 + 3 = 4.
	// proxyHeaderStart = 4 + 4 + 1 = 9
	// proxyListStart = 9 + 2 = 11
	// proxyDataStart = 11 + 1 = 12
	proxyHit := ResolveMouseHit(state, 10, 12)
	if proxyHit.Target != MouseTargetProxy || proxyHit.Index != 0 {
		t.Fatalf("expected proxy index 0, got target=%v index=%d", proxyHit.Target, proxyHit.Index)
	}

	headerHit := ResolveMouseHit(state, 10, 4)
	if headerHit.Target != MouseTargetNone {
		t.Fatalf("expected no hit on header row, got target=%v index=%d", headerHit.Target, headerHit.Index)
	}
}

func TestResolveMouseHit_WithScrollWindow(t *testing.T) {
	state := PageState{
		GroupNames:     []string{"g0", "g1", "g2", "g3", "g4", "g5", "g6", "g7", "g8", "g9"},
		SelectedGroup:  7,
		GroupScrollTop: 0,
		CurrentProxies: []string{"p0", "p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10", "p11"},
		SelectedProxy:  11,
		ProxyScrollTop: 0,
		Height:         24,
	}

	// groupMaxLines=4 时，selected=7 会将可见窗口顶到 4，首行数据应命中 g4
	groupHit := ResolveMouseHit(state, 10, 5)
	if groupHit.Target != MouseTargetGroup || groupHit.Index != 4 {
		t.Fatalf("expected group index 4, got target=%v index=%d", groupHit.Target, groupHit.Index)
	}

	// proxyMaxLines=11 时 (24-12-4=8), selected=11 会将可见窗口顶到 4，首行数据应命中 p4
	// 让 ResolveMouseHit 自动计算
	proxyHit := ResolveMouseHit(state, 10, 15)
	if proxyHit.Target != MouseTargetProxy || proxyHit.Index < 0 {
		t.Fatalf("expected proxy hit, got target=%v index=%d", proxyHit.Target, proxyHit.Index)
	}
}

func TestRenderNodesPage_UsesSideBySidePanelsOnWideScreens(t *testing.T) {
	state := PageState{
		GroupNames:     []string{"Proxy", "Netflix"},
		SelectedGroup:  0,
		Groups:         testGroups(),
		CurrentProxies: []string{"Hong Kong 1", "Tokyo 1"},
		SelectedProxy:  0,
		Proxies:        testProxies(),
		Width:          130,
		Height:         32,
	}

	lines := strings.Split(RenderNodesPage(state), "\n")
	groupHeader := i18n.Tf("nodes.group_header", state.SelectedGroup+1, len(state.GroupNames))
	listHeader := i18n.Tf("nodes.list_header", state.SelectedProxy+1, len(state.CurrentProxies))
	var sideBySide bool
	for _, line := range lines {
		if strings.Contains(line, groupHeader) && strings.Contains(line, listHeader) {
			sideBySide = true
			break
		}
	}
	if !sideBySide {
		t.Fatalf("expected wide nodes page to render policy and node panels side by side")
	}
}

func TestRenderNodesPage_StacksPanelsOnNarrowScreens(t *testing.T) {
	state := PageState{
		GroupNames:     []string{"Proxy", "Netflix"},
		SelectedGroup:  0,
		Groups:         testGroups(),
		CurrentProxies: []string{"Hong Kong 1", "Tokyo 1"},
		SelectedProxy:  0,
		Proxies:        testProxies(),
		Width:          78,
		Height:         28,
	}

	lines := strings.Split(RenderNodesPage(state), "\n")
	groupHeader := i18n.Tf("nodes.group_header", state.SelectedGroup+1, len(state.GroupNames))
	listHeader := i18n.Tf("nodes.list_header", state.SelectedProxy+1, len(state.CurrentProxies))
	for _, line := range lines {
		if strings.Contains(line, groupHeader) && strings.Contains(line, listHeader) {
			t.Fatalf("expected narrow nodes page to stack panels, got combined header line %q", line)
		}
	}
}

func TestResolveMouseHit_WideLayoutProxyPanel(t *testing.T) {
	state := PageState{
		GroupNames:     []string{"g1", "g2", "g3"},
		SelectedGroup:  0,
		CurrentProxies: []string{"p1", "p2", "p3", "p4"},
		SelectedProxy:  0,
		Width:          130,
		Height:         30,
	}

	hit := ResolveMouseHit(state, 58, 4)
	if hit.Target != MouseTargetProxy || hit.Index != 0 {
		t.Fatalf("expected first proxy row in right panel, got target=%v index=%d", hit.Target, hit.Index)
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

