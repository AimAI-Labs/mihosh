package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/help"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/notice"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleGlobalMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.guideState.Active {
		return m, nil
	}
	// 帮助弹窗打开时吞掉所有鼠标事件，防止穿透到底层
	if m.showHelp {
		// 左键点击任意位置关闭弹窗
		if isMouseLeftPress(msg) {
			m.showHelp = false
			m.helpState = help.State{}
		} else if isMouseWheelUp(msg) || isMouseWheelDown(msg) {
			delta := 1
			if isMouseWheelUp(msg) {
				delta = -1
			}
			m.helpState = m.helpState.Scroll(delta, m.width, m.height, m.buildHelpContext())
		}
		return m, nil
	}
	// 报错弹窗打开时吞掉所有鼠标事件
	if m.notice.DetailsOpen {
		if isMouseLeftPress(msg) {
			m.notice.Clear()
		}
		return m, nil
	}
	// 更新弹窗打开时吞掉所有鼠标事件
	if m.showUpdateDialog {
		if isMouseLeftPress(msg) && !m.isUpdating {
			m.showUpdateDialog = false
			m.updateError = nil
		}
		return m, nil
	}
	page := m.mainPageLayout()
	if msg.Y == page.noticeRow() {
		if isMouseLeftPress(msg) {
			if m.notice.Message.Text == "" && m.currentPage == layout.PageNodes && !m.nodesState.Testing &&
				!m.pageInputCapture().Active && notice.HitText(m.visibleNotice().Text, msg.X, m.width) {
				return m.dispatchToPage(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
			}
			m.notice.OpenDetails(msg.X, m.width)
		}
		return m, nil
	}
	if m.pageInputCapture().Active {
		pageX, pageY, pageWidth, pageHeight, ok := page.resolveMainPageMouseHit(msg.X, msg.Y)
		if !ok {
			if m.currentPage == layout.PageCore && isMouseLeftPress(msg) {
				return m.dispatchToPage(messages.PageMouseClickMsg{X: -1, Y: -1, Width: page.width, Height: page.height})
			}
			return m, nil
		}
		switch {
		case isMouseLeftPress(msg):
			if keyMsg, hit := page.resolveInlineHelpClick(pageX, pageY); hit {
				return m.handleGlobalKeys(keyMsg)
			}
			if pageY >= pageHeight || (m.currentPage == layout.PageCore && len(page.help) > 0 && pageY == pageHeight-1) {
				return m, nil
			}
			return m.dispatchToPage(messages.PageMouseClickMsg{X: pageX, Y: pageY, Width: pageWidth, Height: pageHeight})
		case isMouseWheelUp(msg), isMouseWheelDown(msg):
			if pageY < pageHeight {
				return m.dispatchToPage(messages.PageMouseScrollMsg{Up: isMouseWheelUp(msg), X: pageX, Y: pageY, Width: pageWidth, Height: pageHeight})
			}
		}
		return m, nil
	}
	switch {
	case isMouseLeftPress(msg):
		// 检查是否点击了底栏 (Status Bar)
		if msg.Y >= page.statusRow() {
			if m.hasUpdate && msg.X >= m.width-5 {
				m.showUpdateDialog = true
				return m, nil
			}

			activeProxy, _, exists := m.nodesState.GetActiveProxyAndDelay()
			if exists && activeProxy != "" && !m.nodesState.Testing {
				// 粗略判断点击区域：假设状态栏左侧宽约 50 个字符 (包含运行状态和节点名称及延时)
				if msg.X > 8 && msg.X < 50 {
					var cmd tea.Cmd
					m.nodesState, cmd = m.nodesState.StartSingleTest(activeProxy)
					return m, cmd
				}
			}
			// 消费掉底栏的点击事件，防止误触发主页面内容
			return m, nil
		}

		if msg.Y >= 0 && msg.Y < layout.TopNavHeight {
			clickedPage := layout.GetClickedTopNavPageForState(msg.X, msg.Y, m.width, m.currentPage, layout.TopNavRefreshStatus{Enabled: m.autoRefreshInterval() > 0, SecondsRemaining: m.autoRefreshRemaining, Interval: m.autoRefreshInterval(), Synced: m.autoRefreshSynced})
			if clickedPage >= 0 && clickedPage < layout.PageCount {
				if clickedPage == m.currentPage {
					return m, m.refreshMenu()
				}
				m.onPageLeave()
				m.currentPage = clickedPage
				return m, m.onPageChange()
			}
		}

		pageX, pageY, pageWidth, pageHeight, ok := page.resolveMainPageMouseHit(msg.X, msg.Y)
		if !ok {
			return m, nil
		}
		if keyMsg, hit := page.resolveInlineHelpClick(pageX, pageY); hit {
			return m.handleGlobalKeys(keyMsg)
		}
		if pageY >= pageHeight || (m.currentPage == layout.PageCore && len(page.help) > 0 && pageY == pageHeight-1) {
			return m, nil
		}
		clickMsg := messages.PageMouseClickMsg{
			X:      pageX,
			Y:      pageY,
			Width:  pageWidth,
			Height: pageHeight,
		}
		return m.dispatchToPage(clickMsg)
	case isMouseWheelUp(msg):
		return m.handleMouseScroll(true, msg.X, msg.Y)
	case isMouseWheelDown(msg):
		return m.handleMouseScroll(false, msg.X, msg.Y)
	}
	return m, nil
}

// handleMouseScroll 处理鼠标滚轮滚动
func (m Model) handleMouseScroll(up bool, x, y int) (tea.Model, tea.Cmd) {
	if y >= 0 && y < layout.TopNavHeight {
		m.onPageLeave()
		if up {
			m.currentPage = (m.currentPage + layout.PageCount - 1) % layout.PageCount
		} else {
			m.currentPage = (m.currentPage + 1) % layout.PageCount
		}
		return m, m.onPageChange()
	}

	pageX, pageY, pageWidth, pageHeight, ok := m.resolveMainPageMouseHit(x, y)
	if ok && pageY >= pageHeight {
		return m, nil
	}
	if !ok {
		pageX, pageY = -1, -1
		pageWidth, pageHeight = m.getPageSize()
	}

	scrollMsg := messages.PageMouseScrollMsg{
		Up:     up,
		X:      pageX,
		Y:      pageY,
		Width:  pageWidth,
		Height: pageHeight,
	}
	return m.dispatchToPage(scrollMsg)
}

func (m *Model) resolveMainPageMouseHit(x, y int) (pageX, pageY, pageWidth, pageHeight int, ok bool) {
	return m.mainPageLayout().resolveMainPageMouseHit(x, y)
}

func (p mainPageLayout) resolveMainPageMouseHit(x, y int) (pageX, pageY, pageWidth, pageHeight int, ok bool) {
	pageWidth, pageHeight = p.width, p.height
	pageY = y - pageTop
	if y == p.noticeRow() {
		return 0, 0, 0, 0, false
	}
	if y > p.noticeRow() {
		pageY -= notice.Height
	}
	if pageY < 0 || y >= p.statusRow() || x < 0 || x >= pageWidth {
		return 0, 0, 0, 0, false
	}

	return x, pageY, pageWidth, pageHeight, true
}

func (p mainPageLayout) resolveInlineHelpClick(pageX, pageY int) (tea.KeyMsg, bool) {
	row := pageY - p.height + 1
	if row < 0 || row >= len(p.help) {
		return tea.KeyMsg{}, false
	}
	text, left := p.helpLine(row)
	hint, ok := common.InlineHintAt(p.help[row], pageX-left, common.DisplayWidth(text))
	if !ok {
		return tea.KeyMsg{}, false
	}
	return hint.KeyMsg()
}

func isMouseLeftPress(msg tea.MouseMsg) bool {
	if msg.Button == tea.MouseButtonLeft {
		return msg.Action == tea.MouseActionPress
	}
	return msg.Type == tea.MouseLeft //nolint:staticcheck // Preserve legacy Bubble Tea v1 mouse messages.
}

func isMouseWheelUp(msg tea.MouseMsg) bool {
	if msg.Button == tea.MouseButtonWheelUp {
		return msg.Action == tea.MouseActionPress
	}
	return msg.Type == tea.MouseWheelUp //nolint:staticcheck // Preserve legacy Bubble Tea v1 mouse messages.
}

func isMouseWheelDown(msg tea.MouseMsg) bool {
	if msg.Button == tea.MouseButtonWheelDown {
		return msg.Action == tea.MouseActionPress
	}
	return msg.Type == tea.MouseWheelDown //nolint:staticcheck // Preserve legacy Bubble Tea v1 mouse messages.
}
