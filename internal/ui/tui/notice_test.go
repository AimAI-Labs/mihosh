package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestGlobalNoticeRendersAbovePageTabsAndHandlesDetails(t *testing.T) {
	i18n.Init()
	for _, width := range []int{40, 120} {
		m := Model{width: width, height: 24, currentPage: layout.PageCore}
		m.nodesState.Testing = true
		m.nodesState.TestingTarget = "HK-01"
		assertNoticeRow(t, m, i18n.T("status.testing")+": HK-01")
		m.nodesState.Testing = false

		next, cmd := m.Update(messages.NoticeMsg{Text: "configuration saved", Level: messages.NoticeSuccess})
		if cmd == nil {
			t.Fatal("success notification did not request expiration")
		}
		m = next.(Model)
		assertNoticeRow(t, m, "configuration saved")
		next, _ = m.Update(messages.ErrMsg{Err: errors.New("connection refused from the core")})
		m = next.(Model)
		assertNoticeRow(t, m, i18n.T("status.err.refused"))
		if width == 120 {
			x := 0
			for x < width && layout.GetClickedTopNavPage(x, 1, width) != layout.PageLogs {
				x++
			}
			next, _ = m.Update(tea.MouseMsg{X: x, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			m = next.(Model)
			if m.currentPage != layout.PageLogs {
				t.Fatal("navigation did not reach logs")
			}
			assertNoticeRow(t, m, i18n.T("status.err.refused"))
		}

		next, _ = m.Update(tea.MouseMsg{X: 1, Y: layout.TopNavHeight, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
		m = next.(Model)
		if !m.notice.DetailsOpen || !strings.Contains(ansi.Strip(m.View()), "connection refused from the core") {
			t.Fatal("clicking the notification did not open its error details")
		}
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = next.(Model)
		if m.notice.DetailsOpen || m.notice.Message.Text != "" {
			t.Fatal("dismissing details did not clear the error")
		}
	}
}

func assertNoticeRow(t *testing.T, m Model, text string) {
	t.Helper()
	view := ansi.Strip(m.View())
	rows := strings.Split(view, "\n")
	visible := ansi.Truncate(text, m.width-1, "…")
	if len(rows) != m.height || strings.TrimSpace(rows[layout.TopNavHeight]) != visible || strings.Count(view, visible) != 1 {
		t.Fatalf("expected %q once above the page content, got %q", visible, view)
	}
	_, pageY, _, _, ok := m.resolveMainPageMouseHit(1, pageTop)
	if !ok || pageY != 0 {
		t.Fatal("page mouse coordinates were not shifted with the notification row")
	}
}
