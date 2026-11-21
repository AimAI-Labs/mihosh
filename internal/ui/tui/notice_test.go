package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestGlobalNoticeRendersAboveInlineHelpAndHandlesDetails(t *testing.T) {
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

		next, _ = m.Update(tea.MouseMsg{X: 1, Y: m.mainPageLayout().noticeRow(), Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
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
	helpY := m.mainPageLayout().noticeRow() + 1
	if len(rows) != m.height || strings.TrimSpace(rows[helpY-1]) != visible || strings.Count(view, visible) != 1 {
		t.Fatalf("expected %q once above the inline help, got %q", visible, view)
	}
	_, pageY, _, _, ok := m.resolveMainPageMouseHit(1, layout.TopNavHeight)
	if !ok || pageY != 0 {
		t.Fatal("the first page row should start directly after the navigation")
	}
	_, pageY, _, pageHeight, ok := m.resolveMainPageMouseHit(1, helpY)
	if !ok || pageY != pageHeight-1 {
		t.Fatal("the first inline help row should use the final page coordinate")
	}
	if _, _, _, _, ok := m.resolveMainPageMouseHit(1, helpY-1); ok {
		t.Fatal("the notice row should not dispatch page clicks")
	}
}

func TestNodeResultsUseNoticeRowAndOpenDetails(t *testing.T) {
	i18n.Init()
	m := Model{width: 100, height: 25, currentPage: layout.PageNodes}
	state, cmd := m.nodesState.StartSingleTest("fixture-node")
	m.nodesState = state
	next, _ := m.Update(cmd())
	m = next.(Model)

	summary := i18n.Tf("nodes.result_badge", 1) + " " + i18n.Tf("nodes.result_fail_count", 1) + " " + i18n.T("nodes.view_detail")
	assertNoticeRow(t, m, summary)
	if m.visibleNotice().Level != messages.NoticeWarning {
		t.Fatal("a failed test must use the theme warning color")
	}
	next, _ = m.Update(tea.MouseMsg{X: 2, Y: m.mainPageLayout().noticeRow(),
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = next.(Model)
	if !m.nodesState.ShowTestDetail {
		t.Fatal("clicking the test result notice did not open the node test details")
	}
}

func TestEveryPageWrapsInlineHelpBelowNotice(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"en-US", "zh-CN"} {
		i18n.SetLanguageOverride(language)
		for page := layout.PageType(0); page < layout.PageCount; page++ {
			m := Model{width: 32, height: 26, currentPage: page}
			m.notice.Message = messages.NoticeMsg{Text: "footer fixture", Level: messages.NoticeInfo}
			arrangement := m.mainPageLayout()
			if len(arrangement.help) < 2 {
				t.Fatalf("%s page %d did not wrap its footer: %+v", language, page, arrangement.help)
			}
			assertNoticeRow(t, m, "footer fixture")
			view := strings.Split(ansi.Strip(m.View()), "\n")
			for i, hints := range arrangement.help {
				y := arrangement.noticeRow() + 1 + i
				want := ansi.Strip(common.FormatInlineHintRow(hints))
				if !strings.HasSuffix(strings.TrimRight(view[y], " "), strings.TrimRight(want, " ")) {
					t.Fatalf("%s page %d row %d missing shortcuts %q: %q", language, page, y, want, view[y])
				}
			}
			if arrangement.statusRow() != m.height-common.StatusBarHeight {
				t.Fatalf("%s page %d status bar moved", language, page)
			}
		}
	}
}

func TestMouseCanClickEarlierWrappedHelpRow(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	m := Model{width: 36, height: 24, currentPage: layout.PageRules}
	page := m.mainPageLayout()
	label := "/ " + i18n.T("help.rules.hint_search")
	view := strings.Split(ansi.Strip(m.View()), "\n")
	for y := page.noticeRow() + 1; y < page.statusRow()-1; y++ {
		if at := strings.Index(view[y], label); at >= 0 {
			next, _ := m.Update(tea.MouseMsg{X: lipgloss.Width(view[y][:at]), Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			if !next.(Model).rulesState.FilterMode() {
				t.Fatal("clicking the wrapped search hint did not enter filter mode")
			}
			return
		}
	}
	t.Fatal("search hint did not appear above the last footer row")
}
