package tui

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
)

func TestResolveMainPageMouseHitUsesTopNavOffset(t *testing.T) {
	m := Model{
		width:  100,
		height: 30,
	}

	pageX, pageY, pageWidth, _, ok := m.resolveMainPageMouseHit(2, layout.TopNavHeight+3)
	if !ok {
		t.Fatal("expected point inside main page content to resolve")
	}
	if pageX != 1 {
		t.Fatalf("expected pageX 1 without left sidebar offset, got %d", pageX)
	}
	if pageY != 0 {
		t.Fatalf("expected pageY 0 after top nav and title offsets, got %d", pageY)
	}
	if pageWidth != 98 {
		t.Fatalf("expected page width 98, got %d", pageWidth)
	}
}

func TestResolveMainPageMouseHitIgnoresTopNavRows(t *testing.T) {
	m := Model{
		width:  100,
		height: 30,
	}

	if _, _, _, _, ok := m.resolveMainPageMouseHit(2, 1); ok {
		t.Fatal("expected top nav row to be outside main page content")
	}
}

func TestMouseClickTopNavChangesPage(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")

	m := Model{
		currentPage: layout.PageNodes,
		width:       100,
		height:      30,
	}
	nav := layout.RenderTopNav(layout.PageNodes, m.width)
	lines := strings.Split(stripANSITUI(nav), "\n")
	if len(lines) != layout.TopNavHeight {
		t.Fatalf("expected top nav height %d, got %d in %q", layout.TopNavHeight, len(lines), nav)
	}
	x := -1
	for candidate := 0; candidate < m.width; candidate++ {
		if layout.GetClickedTopNavPage(candidate, 1, m.width) == layout.PageLogs {
			x = candidate
			break
		}
	}
	if x < 0 || !strings.Contains(lines[1], i18n.T("menu.logs")) {
		t.Fatalf("expected logs label and clickable coordinate in top nav, got %q", nav)
	}

	next, _ := m.Update(tea.MouseMsg{
		X:      x,
		Y:      1,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	got := next.(Model).currentPage
	if got != layout.PageLogs {
		t.Fatalf("expected click on top nav logs item to switch to PageLogs, got %v", got)
	}
}

func TestViewShowsAutoRefreshHintInTopNav(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")

	m := Model{
		currentPage:          layout.PageNodes,
		width:                104,
		height:               30,
		config:               &config.Config{AutoRefreshInterval: 5},
		autoRefreshRemaining: 5,
	}

	view := stripANSITUI(m.View())

	if !strings.Contains(view, "自动刷新") || !strings.Contains(view, "5s") {
		t.Fatalf("expected top nav to show auto refresh hint and countdown, got %q", view)
	}
}

func stripANSITUI(s string) string {
	var out strings.Builder
	inEscape := false
	for _, r := range s {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
