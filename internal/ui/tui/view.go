package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/charmbracelet/lipgloss"
)

// View 渲染视图
func (m Model) View() string {
	if m.width == 0 {
		return "正在初始化..."
	}

	// 帮助弹窗处理
	if m.showHelp {
		helpView := m.renderHelpPage()
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpView)
	}

	// ── 布局参数 ──
	statusBarHeight := common.StatusBarHeight // 分隔线 + 信息行
	contentHeight := m.height - statusBarHeight - layout.TopNavHeight
	if contentHeight < common.MinContentHeight {
		contentHeight = common.MinContentHeight
	}
	mainWidth := m.width
	if mainWidth < common.MinMainWidth {
		mainWidth = common.MinMainWidth
	}

	// ── 顶部导航 ──
	topNav := layout.RenderTopNav(m.currentPage, m.width, layout.TopNavRefreshStatus{
		Enabled:          m.autoRefreshInterval() > 0,
		SecondsRemaining: m.autoRefreshRemaining,
		Synced:           m.autoRefreshSynced,
	})

	// ── 渲染当前页面内容 ──
	var pageContent string
	switch m.currentPage {
	case layout.PageNodes:
		pageContent = m.renderNodesPage()
	case layout.PageConnections:
		pageContent = m.renderConnectionsPage()
	case layout.PageSettings:
		pageContent = m.renderSettingsPage()
	case layout.PageLogs:
		pageContent = m.renderLogsPage()
	case layout.PageRules:
		pageContent = m.renderRulesPage()
	}

	// ── 底部状态栏 ──
	var uploadTotal, downloadTotal int64
	if m.connsState.Connections != nil {
		uploadTotal = m.connsState.Connections.UploadTotal
		downloadTotal = m.connsState.Connections.DownloadTotal
	}

	// 提取当前活动节点信息（优先 GLOBAL 组，即实际代理出口）
	var groupName, nodeName string
	var delay int
	// 优先查找 GLOBAL 组
	if group, ok := m.nodesState.Groups["GLOBAL"]; ok {
		groupName = "GLOBAL"
		nodeName = group.Now
	} else if len(m.nodesState.GroupNames) > 0 {
		// 回退到第一个策略组
		groupName = m.nodesState.GroupNames[0]
		if group, ok := m.nodesState.Groups[groupName]; ok {
			nodeName = group.Now
		}
	}
	if nodeName != "" {
		if proxy, ok := m.nodesState.Proxies[nodeName]; ok && len(proxy.History) > 0 {
			delay = proxy.History[len(proxy.History)-1].Delay
		}
	}

	statusBar := layout.RenderStatusBar(
		m.width,
		m.err,
		m.nodesState.Testing,
		m.nodesState.TestingTarget,
		m.notice,
		m.chartData,
		uploadTotal,
		downloadTotal,
		m.nodesState.Mode,
		groupName,
		nodeName,
		delay,
	)

	return lipgloss.JoinVertical(lipgloss.Left, topNav, pageContent, statusBar)
}
