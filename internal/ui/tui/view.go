package tui

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/help"
	"github.com/charmbracelet/lipgloss"
)

// View 渲染视图
func (m Model) View() string {
	if m.width == 0 {
		return "正在初始化..."
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
		Interval:         m.autoRefreshInterval(),
		Synced:           m.autoRefreshSynced,
	})

	// ── 渲染当前页面内容（固定高度，确保状态栏不崩塌）──
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
	// 将页面内容约束在精确的 contentHeight 行内：
	// 内容不足时补空行，内容溢出时截断，确保状态栏始终固定在底部
	pageContent = clampToHeight(pageContent, contentHeight)

	// ── 底部状态栏 ──
	var uploadTotal, downloadTotal int64
	if m.connsState.Connections != nil {
		uploadTotal = m.connsState.Connections.UploadTotal
		downloadTotal = m.connsState.Connections.DownloadTotal
	}

	// 提取当前活动节点信息（复用 GetActiveProxyAndDelay 统一逻辑）
	var groupName, nodeName string
	var delay int
	nodeName, delay, _ = m.nodesState.GetActiveProxyAndDelay()
	if nodeName != "" {
		// 根据模式反推所属策略组名
		groupName = m.nodesState.ResolveActiveGroupName()
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

	fullPage := lipgloss.JoinVertical(lipgloss.Left, topNav, pageContent, statusBar)

	// ── 帮助弹窗叠加（lazygit 风格，叠加在完整页面之上）──
	// 节点页始终显示右下角内联帮助提示；按 ? 可叠加完整帮助弹窗
	if m.showHelp {
		return help.OverlayHelpPopup(fullPage, m.width, m.height, m.buildHelpContext())
	}

	return fullPage
}

// buildHelpContext 根据当前模型状态构建帮助弹窗上下文
func (m Model) buildHelpContext() help.HelpContext {
	return help.HelpContext{
		CurrentPage:        int(m.currentPage),
		ConnViewMode:       m.connsState.ViewMode(),
		ConnDetail:         m.connsState.DetailMode(),
		ConnTopN:           m.connsState.TopNModalMode(),
		ConnFilter:         m.connsState.FilterMode(),
		NodesTestDetail:    m.nodesState.ShowTestDetail,
		NodesFilterMode:    m.nodesState.NodeFilterMode,
		LogsDetail:         m.logsState.DetailMode(),
		LogsFilter:         m.logsState.FilterMode(),
		RulesTypeFilter:     m.rulesState.ShowTypeFilter(),
		RulesFilter:         m.rulesState.FilterMode(),
		RulesDeleteConfirm:  m.rulesState.ShowDeleteConfirm(),
		SettingsEdit:       m.settingsState.IsEditing(),
		SettingsLanguage:   m.settingsState.IsLanguageSelected(),
	}
}

// clampToHeight 将内容字符串精确约束为 h 行：
// 不足时在末尾补空行，超出时截断多余行。
// 这保证了 lipgloss.JoinVertical 后状态栏始终位于终端最底部。
func clampToHeight(content string, h int) string {
	if h <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) < h {
		// 补空行
		for len(lines) < h {
			lines = append(lines, "")
		}
	} else if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}
