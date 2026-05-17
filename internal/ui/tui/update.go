package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// connRefreshInterval 连接刷新间隔
const connRefreshInterval = 1 * time.Second

// connTick 创建连接页面定时器
func connTick() tea.Cmd {
	return tea.Tick(connRefreshInterval, func(t time.Time) tea.Msg {
		return messages.ConnTickMsg(t)
	})
}

// logsTick 创建日志页面定时器
func logsTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return messages.LogsTickMsg(t)
	})
}

// Init 初始化
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		nodes.FetchGroups(m.client),
		nodes.FetchProxies(m.client),
		nodes.FetchConfigMode(m.client),
		startWSStreams(m.wsClient, m.wsMsgChan),
		listenWSMessages(m.wsCtx, m.wsMsgChan),
	)
}

// Update 消息路由器：全局消息自处理，页面消息分发到子状态
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── 全局：窗口大小 ──
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.logsState = m.logsState.UpdateMaxHScrollOffset(m.width, m.height)
		return m, tea.ClearScreen

	// ── 全局：鼠标事件 ──
	case tea.MouseMsg:
		switch {
		case isMouseLeftPress(msg):
			statusBarHeight := common.StatusBarHeight
			contentHeight := m.height - statusBarHeight
			if contentHeight < common.MinContentHeight {
				contentHeight = common.MinContentHeight
			}
			if msg.X >= 0 && msg.X < layout.SidebarWidth && msg.Y >= 0 && msg.Y < contentHeight {
				clickedPage := layout.GetClickedPage(msg.X, msg.Y, contentHeight)
				if clickedPage >= 0 && clickedPage < layout.PageCount {
					m.currentPage = clickedPage
					return m, m.onPageChange()
				}
			}

			if m.currentPage == layout.PageNodes {
				return m.handleNodesMouseLeft(msg.X, msg.Y)
			}
			if m.currentPage == layout.PageConnections {
				return m.handleConnectionsMouseLeft(msg.X, msg.Y)
			}
			if m.currentPage == layout.PageSettings {
				return m.handleSettingsMouseLeft(msg.X, msg.Y)
			}
			if m.currentPage == layout.PageLogs {
				return m.handleLogsMouseLeft(msg.X, msg.Y)
			}
		case isMouseWheelUp(msg):
			return m.handleMouseScroll(true, msg.X, msg.Y)
		case isMouseWheelDown(msg):
			return m.handleMouseScroll(false, msg.X, msg.Y)
		}
		return m, nil

	// ── 全局：键盘 ──
	case tea.KeyMsg:
		// 帮助弹窗拦截
		if m.showHelp {
			switch msg.String() {
			case "esc", "q", "?":
				m.showHelp = false
				return m, nil
			}
			if key.Matches(msg, common.Keys.Quit) {
				return m, tea.Quit
			}
			return m, nil
		}

		// 全局帮助
		if msg.String() == "?" {
			m.showHelp = true
			return m, nil
		}

		// 全局快捷键
		switch {
		case key.Matches(msg, common.Keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, common.Keys.NextPage):
			m.currentPage = (m.currentPage + 1) % layout.PageCount
			return m, m.onPageChange()

		case key.Matches(msg, common.Keys.PrevPage):
			m.currentPage = (m.currentPage + layout.PageCount - 1) % layout.PageCount
			return m, m.onPageChange()

		case key.Matches(msg, common.Keys.Page1):
			m.currentPage = layout.PageNodes
			return m, m.onPageChange()

		case key.Matches(msg, common.Keys.Page2):
			m.currentPage = layout.PageConnections
			return m, m.onPageChange()

		case key.Matches(msg, common.Keys.Page3):
			m.currentPage = layout.PageLogs
			return m, m.onPageChange()

		case key.Matches(msg, common.Keys.Page4):
			m.currentPage = layout.PageRules
			return m, m.onPageChange()

		case key.Matches(msg, common.Keys.Page5):
			m.currentPage = layout.PageSettings
			return m, nil

		case key.Matches(msg, common.Keys.Refresh):
			return m, m.refreshCurrentPage()
		}

		// 分发到页面子状态
		return m.dispatchKeyToPage(msg)

	// ── 数据消息：分发到子状态 ──

	case messages.GroupsMsg:
		m.nodesState = m.nodesState.ApplyGroups(msg.Groups, msg.OrderedNames)

	case messages.ProxiesMsg:
		m.nodesState = m.nodesState.ApplyProxies(msg)

	case messages.ConfigModeMsg:
		m.nodesState = m.nodesState.ApplyConfigMode(msg.Mode)

	case messages.ConnectionsMsg:
		m.connsState = m.connsState.ApplyConnections(msg.Resp)
		if msg.Resp != nil && m.chartData != nil {
			m.chartData.AddConnCountData(len(msg.Resp.Connections))
		}

	case messages.MemoryWSMsg:
		if m.chartData != nil {
			m.chartData.AddMemoryData(msg.Memory)
		}
		if m.wsMsgChan != nil {
			return m, listenWSMessages(m.wsCtx, m.wsMsgChan)
		}

	case messages.TrafficWSMsg:
		if m.chartData != nil {
			m.chartData.AddSpeedData(msg.Up, msg.Down)
		}
		if m.wsMsgChan != nil {
			return m, listenWSMessages(m.wsCtx, m.wsMsgChan)
		}

	case messages.ConnectionsWSMsg:
		m.connsState = m.connsState.ApplyWSConnections(msg.Data)
		if m.chartData != nil {
			m.chartData.AddConnCountData(len(msg.Data.Connections))
		}
		if m.wsMsgChan != nil {
			return m, listenWSMessages(m.wsCtx, m.wsMsgChan)
		}

	case messages.LogsWSMsg:
		m.logsState = m.logsState.AppendLog(msg.LogType, msg.Payload)
		if m.wsMsgChan != nil {
			return m, listenWSMessages(m.wsCtx, m.wsMsgChan)
		}

	case messages.LogIPResolvedMsg:
		m.logsState = m.logsState.ApplyIPResolved(msg.IP, msg.Resolved)

	case messages.RulesMsg:
		m.rulesState = m.rulesState.ApplyRules(msg)

	case messages.SiteTestMsg:
		m.connsState = m.connsState.ApplySiteTestResult(msg.Name, msg.Delay, msg.Err)

	case messages.TestDoneMsg:
		m.nodesState = m.nodesState.ApplyTestDone(msg.Name, msg.Delay, msg.Err)
		// 如果是批量测速，需要补位
		if m.nodesState.TestAllActive {
			var batchCmd tea.Cmd
			m.nodesState, batchCmd = m.nodesState.LaunchBatchTests(m.client, m.testURL, m.timeout)
			return m, batchCmd
		}
		return m, nodes.FetchProxies(m.client)

	case messages.TestAllDoneMsg:
		m.nodesState = m.nodesState.ApplyTestAllDone(msg.Results)
		return m, nodes.FetchProxies(m.client)

	case messages.IPInfoMsg:
		if msg.Info != nil {
			m.connsState = m.connsState.ApplyIPInfo(msg.Info)
		}

	case messages.ConnectionClosedMsg:
		m.connsState = m.connsState.ApplyConnectionClosed()

	case messages.AllConnectionsClosedMsg:
		m.connsState = m.connsState.ApplyAllConnectionsClosed()

	case messages.ConnTickMsg:
		if m.currentPage == layout.PageConnections {
			return m, connTick()
		}

	case messages.LogsTickMsg:
		if m.currentPage == layout.PageLogs {
			return m, logsTick()
		}

	case messages.ErrMsg:
		m.err = msg
		m.nodesState.Testing = false
		m.nodesState.TestingTarget = ""
		m.nodesState.TestAllActive = false
		m.nodesState.TestAllPending = nil
		m.nodesState.TestAllRunning = nil
		m.nodesState.TestAllTotal = 0
		m.nodesState.TestAllDone = 0
		m.nodesState.TestPending = 0
	}

	return m, nil
}

// dispatchKeyToPage 将按键分发到当前页面子状态
func (m Model) dispatchKeyToPage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.currentPage {
	case layout.PageNodes:
		m.nodesState, cmd = m.nodesState.Update(msg, m.client, m.proxySvc, m.testURL, m.timeout)

	case layout.PageConnections:
		m.connsState, cmd = m.connsState.Update(msg, m.client, m.timeout)

	case layout.PageLogs:
		m.logsState, cmd = m.logsState.Update(msg, m.ipResolver)

	case layout.PageRules:
		m.rulesState, cmd = m.rulesState.Update(msg, m.client)

	case layout.PageSettings:
		var newCfg, proxyAddr = m.config, ""
		oldLanguage := ""
		if m.config != nil {
			oldLanguage = m.config.Language
		}
		m.settingsState, newCfg, proxyAddr, cmd = m.settingsState.Update(msg, m.config, m.configSvc)
		m.config = newCfg
		if newCfg != nil && newCfg.Language != oldLanguage {
			i18n.SetLanguageOverride(newCfg.Language)
			common.InitKeyBindings()
			cmd = tea.Batch(cmd, tea.ClearScreen)
		}
		if proxyAddr != "" {
			m.connsState = m.connsState.UpdateProxyAddr(proxyAddr)
		}
	}
	return m, cmd
}

// onPageChange 页面切换处理
func (m *Model) onPageChange() tea.Cmd {
	m.err = nil
	switch m.currentPage {
	case layout.PageConnections:
		m.connsState = m.connsState.ResetPrevConnIDs()
		return tea.Batch(connections.FetchConnections(m.client), connTick())
	case layout.PageLogs:
		return logsTick()
	case layout.PageRules:
		return rules.FetchRules(m.client)
	}
	return nil
}

// refreshCurrentPage 刷新当前页面
func (m *Model) refreshCurrentPage() tea.Cmd {
	switch m.currentPage {
	case layout.PageNodes:
		return tea.Batch(nodes.FetchGroups(m.client), nodes.FetchProxies(m.client))
	case layout.PageRules:
		return rules.FetchRules(m.client)
	case layout.PageSettings:
		cfg, _ := m.configSvc.LoadConfig()
		m.config = cfg
	}
	return nil
}

// handleMouseScroll 处理鼠标滚轮滚动
func (m Model) handleMouseScroll(up bool, x, y int) (tea.Model, tea.Cmd) {
	if x >= 0 && x < layout.SidebarWidth {
		if up {
			m.currentPage = (m.currentPage + layout.PageCount - 1) % layout.PageCount
		} else {
			m.currentPage = (m.currentPage + 1) % layout.PageCount
		}
		return m, m.onPageChange()
	}

	sidebarRenderedWidth := layout.SidebarWidth + 1
	mainWidth := m.width - sidebarRenderedWidth
	if mainWidth < common.MinMainWidth {
		mainWidth = common.MinMainWidth
	}
	mainX := x - sidebarRenderedWidth
	mainY := y
	mainHeight := m.height

	switch m.currentPage {
	case layout.PageNodes:
		pageX, pageY, pageWidth, pageHeight, ok := m.resolveMainPageMouseHit(x, y)
		if ok {
			m.nodesState = m.nodesState.HandleMouseScroll(up, pageX, pageY, pageWidth, pageHeight)
		} else {
			// 如果无法解析，则回退到原始调用（不传坐标，虽然 nodesState 也会变）
			m.nodesState = m.nodesState.HandleMouseScroll(up, -1, -1, m.width, m.height)
		}
	case layout.PageConnections:
		m.connsState = m.connsState.HandleMouseScroll(up, mainX, mainY, mainWidth, mainHeight)
	case layout.PageLogs:
		m.logsState = m.logsState.HandleMouseScroll(up)
	case layout.PageRules:
		m.rulesState = m.rulesState.HandleMouseScroll(up)
	case layout.PageSettings:
		m.settingsState = m.settingsState.HandleMouseScroll(up)
	}

	return m, nil
}

func (m Model) handleNodesMouseLeft(x, y int) (tea.Model, tea.Cmd) {
	pageX, pageY, pageWidth, pageHeight, ok := m.resolveMainPageMouseHit(x, y)
	if !ok {
		return m, nil
	}

	var cmd tea.Cmd
	m.nodesState, cmd = m.nodesState.HandleMouseLeft(pageX, pageY, pageWidth, pageHeight, m.client)
	return m, cmd
}

func (m Model) handleConnectionsMouseLeft(x, y int) (tea.Model, tea.Cmd) {
	pageX, pageY, pageWidth, pageHeight, ok := m.resolveMainPageMouseHit(x, y)
	if !ok {
		return m, nil
	}

	var cmd tea.Cmd
	m.connsState, cmd = m.connsState.HandleMouseLeft(pageX, pageY, pageWidth, pageHeight, m.chartData, m.timeout)
	return m, cmd
}

func (m Model) handleSettingsMouseLeft(x, y int) (tea.Model, tea.Cmd) {
	_, pageY, _, _, ok := m.resolveMainPageMouseHit(x, y)
	if !ok {
		return m, nil
	}

	m.settingsState = m.settingsState.HandleMouseLeft(pageY, m.config)
	return m, nil
}

func (m Model) handleLogsMouseLeft(x, y int) (tea.Model, tea.Cmd) {
	pageX, pageY, pageWidth, _, ok := m.resolveMainPageMouseHit(x, y)
	if !ok {
		return m, nil
	}

	var cmd tea.Cmd
	m.logsState, cmd = m.logsState.HandleMouseLeft(pageY, pageX, pageWidth, m.ipResolver)
	return m, cmd
}

func (m Model) resolveMainPageMouseHit(x, y int) (pageX, pageY, pageWidth, pageHeight int, ok bool) {
	statusBarHeight := common.StatusBarHeight
	contentHeight := m.height - statusBarHeight
	if contentHeight < common.MinContentHeight {
		contentHeight = common.MinContentHeight
	}
	if y < 0 || y >= contentHeight {
		return 0, 0, 0, 0, false
	}

	sidebarRenderedWidth := layout.SidebarWidth + 1
	mainWidth := m.width - sidebarRenderedWidth
	if mainWidth < common.MinMainWidth {
		mainWidth = common.MinMainWidth
	}

	mainX := x - sidebarRenderedWidth
	if mainX <= 0 || mainX >= mainWidth-1 {
		return 0, 0, 0, 0, false
	}
	if y <= 0 || y >= contentHeight-1 {
		return 0, 0, 0, 0, false
	}

	contentY := y - 1
	const pageContentOffsetY = 2 // 标题 + 空行
	if contentY < pageContentOffsetY {
		return 0, 0, 0, 0, false
	}

	pageY = contentY - pageContentOffsetY
	pageHeight = m.height - 8
	pageWidth = mainWidth - 2
	if pageWidth < common.MinMainWidth {
		pageWidth = common.MinMainWidth
	}
	pageX = mainX - 1
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
