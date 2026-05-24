package tui

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
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

	// ── 主面板 ──
	pageTitle := layout.GetPageTitle(m.currentPage)

	titleStyle := lipgloss.NewStyle().
		Foreground(styles.ColorPrimary).
		Bold(true).
		Padding(0, 1)

	mainPaneStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Width(mainWidth - 2).
		Height(contentHeight - 2)

	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(pageTitle),
		"",
		pageContent,
	)

	mainPane := mainPaneStyle.Render(mainContent)

	// ── 底部状态栏 ──
	var uploadTotal, downloadTotal int64
	if m.connsState.Connections != nil {
		uploadTotal = m.connsState.Connections.UploadTotal
		downloadTotal = m.connsState.Connections.DownloadTotal
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
	)

	return lipgloss.JoinVertical(lipgloss.Left, topNav, mainPane, statusBar)
}
