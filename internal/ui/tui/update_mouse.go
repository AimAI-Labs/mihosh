package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleGlobalMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// 帮助弹窗打开时吞掉所有鼠标事件，防止穿透到底层
	if m.showHelp {
		// 左键点击任意位置关闭弹窗
		if isMouseLeftPress(msg) {
			m.showHelp = false
		}
		return m, nil
	}
	// 报错弹窗打开时吞掉所有鼠标事件
	if m.showErrorPopup {
		if isMouseLeftPress(msg) {
			m.showErrorPopup = false
		}
		return m, nil
	}
	// 更新弹窗打开时吞掉所有鼠标事件
	if m.showUpdateDialog {
		if isMouseLeftPress(msg) {
			m.showUpdateDialog = false
			m.updateError = nil
		}
		return m, nil
	}
	switch {
	case isMouseLeftPress(msg):
		statusBarHeight := common.StatusBarHeight
		contentHeight := m.height - statusBarHeight
		if contentHeight < common.MinContentHeight {
			contentHeight = common.MinContentHeight
		}

		// 检查是否点击了底栏 (Status Bar)
		if msg.Y >= contentHeight {
			if m.err != nil {
				// 粗略判断点击区域：假设状态栏左侧显示错误信息
				if msg.X >= 0 && msg.X < m.width/2 {
					m.showErrorPopup = true
					return m, nil
				}
			}

			if m.hasUpdate && msg.X >= m.width-5 {
				m.showUpdateDialog = true
				return m, nil
			}

			activeProxy, _, exists := m.nodesState.GetActiveProxyAndDelay()
			if exists && activeProxy != "" && !m.nodesState.Testing {
				// 粗略判断点击区域：假设状态栏左侧宽约 50 个字符 (包含运行状态和节点名称及延时)
				if msg.X > 8 && msg.X < 50 {
					m.nodesState = m.nodesState.StartSingleTest(activeProxy)
					return m, nodes.TestProxy(m.client, activeProxy, m.testURL, m.timeout)
				}
			}
			// 消费掉底栏的点击事件，防止误触发主页面内容
			return m, nil
		}

		if msg.Y >= 0 && msg.Y < layout.TopNavHeight {
			clickedPage := layout.GetClickedTopNavPage(msg.X, msg.Y, m.width)
			if clickedPage >= 0 && clickedPage < layout.PageCount {
				m.currentPage = clickedPage
				return m, m.onPageChange()
			}
		}

		pageX, pageY, pageWidth, pageHeight, ok := m.resolveMainPageMouseHit(msg.X, msg.Y)
		if !ok {
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

	// ── 全局：键盘 ──
	return m, nil
}

// handleMouseScroll 处理鼠标滚轮滚动
func (m Model) handleMouseScroll(up bool, x, y int) (tea.Model, tea.Cmd) {
	if y >= 0 && y < layout.TopNavHeight {
		if up {
			m.currentPage = (m.currentPage + layout.PageCount - 1) % layout.PageCount
		} else {
			m.currentPage = (m.currentPage + 1) % layout.PageCount
		}
		return m, m.onPageChange()
	}

	pageX, pageY, pageWidth, pageHeight, ok := m.resolveMainPageMouseHit(x, y)
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



func (m Model) resolveMainPageMouseHit(x, y int) (pageX, pageY, pageWidth, pageHeight int, ok bool) {
	statusBarHeight := common.StatusBarHeight
	contentHeight := m.height - statusBarHeight - layout.TopNavHeight
	if contentHeight < common.MinContentHeight {
		contentHeight = common.MinContentHeight
	}
	contentYAbs := y - layout.TopNavHeight
	if contentYAbs < 0 || contentYAbs >= contentHeight {
		return 0, 0, 0, 0, false
	}

	mainWidth := m.width
	if mainWidth < common.MinMainWidth {
		mainWidth = common.MinMainWidth
	}

	// 移除外边框检查，内容直接从顶部开始
	pageY = contentYAbs
	pageHeight = m.height - layout.TopNavHeight - common.StatusBarHeight
	if pageHeight < common.MinContentHeight {
		pageHeight = common.MinContentHeight
	}
	pageWidth = mainWidth
	if pageWidth < common.MinMainWidth {
		pageWidth = common.MinMainWidth
	}
	pageX = x
	if pageX < 0 || pageX >= pageWidth {
		return 0, 0, 0, 0, false
	}

	return pageX, pageY, pageWidth, pageHeight, true
}

func isMouseLeftPress(msg tea.MouseMsg) bool {
	if msg.Button == tea.MouseButtonLeft {
		return msg.Action == tea.MouseActionPress
	}
	return msg.Type == tea.MouseLeft
}

func isMouseWheelUp(msg tea.MouseMsg) bool {
	if msg.Button == tea.MouseButtonWheelUp {
		return msg.Action == tea.MouseActionPress
	}
	return msg.Type == tea.MouseWheelUp
}

func isMouseWheelDown(msg tea.MouseMsg) bool {
	if msg.Button == tea.MouseButtonWheelDown {
		return msg.Action == tea.MouseActionPress
	}
	return msg.Type == tea.MouseWheelDown
}
