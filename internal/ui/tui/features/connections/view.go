package connections

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

// PageState 连接页面状态
type PageState struct {
	Connections        *model.ConnectionsResponse
	Width              int
	Height             int
	SelectedIndex      int
	ScrollTop          int
	FilterText         string
	FilterInput        string // textinput.View() 渲染结果
	FilterMode         bool
	DetailMode         bool              // 是否显示详情
	SelectedConnection *model.Connection // 选中的连接
	IPInfo             *model.IPInfo     // 目标IP地理信息
	DetailLeftScroll   int               // 详情左侧页面滚动偏移
	DetailRightScroll  int               // 详情右侧页面滚动偏移
	DetailFocusPanel   int               // 详情当前焦点面板
	// 图表数据
	ChartData *model.ChartData
	// 视图模式
	ViewMode          int                // 0=流量监控, 1=活跃连接, 2=历史连接
	ClosedConnections []model.Connection // 已关闭的连接历史
	// 网站测速
	SiteTests        []model.SiteTest // 网站测试数据
	SelectedSiteTest int              // 选中的网站索引
	// Top N 排行榜
	TopNItems       []components.TopNItem
	TopNModalMode   bool
	TopNModalItems  []components.TopNItem
	TopNModalScroll int
	InlineMode      bool // 是否开启内联详情模式
	InlineFocused   bool // 内联详情是否获取焦点
	InlineScroll    int  // 内联详情滚动偏移
}

// RenderConnectionsPage 渲染连接监控页面
func RenderConnectionsPage(state PageState) string {
	// 详情模式优先级最高：渲染沉浸式连接详情（位于模式切换栏下方）
	if state.DetailMode && state.SelectedConnection != nil {
		modeSwitch := RenderConnModeSwitchComponent(state.ViewMode, state.Width, state.InlineMode)
		modeSwitchHeight := lipgloss.Height(modeSwitch)
		// 再减 1 行，用于在详情面板下方预留底栏提示行，
		// 避免 OverlayHelpAtBottomRight 将提示叠加到详情面板底边框（╰──╯）上。
		remainingHeight := state.Height - modeSwitchHeight - 2
		if remainingHeight < 5 {
			remainingHeight = 5
		}

		detailView := components.RenderConnectionDetailImmersive(
			state.SelectedConnection,
			state.IPInfo,
			state.Width,
			remainingHeight,
			state.DetailLeftScroll,
			state.DetailRightScroll,
			state.DetailFocusPanel,
		)

		// 详情面板下方留 1 行空白，供右下角内联帮助提示使用。
		mainContent := lipgloss.JoinVertical(lipgloss.Left, modeSwitch, "", detailView, "")
		return renderConnectionsInlineHelp(mainContent, state)
	}

	if state.TopNModalMode {
		mainContent := components.RenderTopNModal(state.TopNModalItems, state.Width, state.Height, state.TopNModalScroll)
		return renderConnectionsInlineHelp(mainContent, state)
	}

	// 渲染模式切换组件（带边框）
	modeSwitch := RenderConnModeSwitchComponent(state.ViewMode, state.Width, state.InlineMode)

	// 组装页面
	var content []string
	content = append(content, modeSwitch)
	content = append(content, "")

	// 流量监控 tab：图表 + TopN + 站点卡片（无连接表格）
	if state.ViewMode == ConnViewTraffic {
		mainContent := renderTrafficTab(state, content)
		return renderConnectionsInlineHelp(mainContent, state)
	}

	// 活跃/历史连接 tab：表格独占
	mainContent := renderConnectionListTab(state, content)
	return renderConnectionsInlineHelp(mainContent, state)
}

// renderTrafficTab 渲染流量监控 tab
func renderTrafficTab(state PageState, content []string) string {
	maxChartHeight := calcMaxChartHeight(state)

	// 渲染监控图表区域
	chartsSection := components.RenderChartsSection(state.ChartData, state.Width, maxChartHeight)
	if chartsSection != "" {
		content = append(content, chartsSection)
		content = append(content, "")
	}

	// 渲染 Top N 大盘
	if len(state.TopNItems) > 0 {
		topNSection := components.RenderTopNSection(state.TopNItems, state.Width)
		if topNSection != "" {
			content = append(content, topNSection)
			content = append(content, "")
		}
	}

	// 渲染网站测速区域
	if len(state.SiteTests) > 0 {
		siteTestSection := components.RenderSiteTestSection(state.SiteTests, state.SelectedSiteTest, state.Width)
		content = append(content, siteTestSection)
	}

	return strings.Join(content, "\n")
}

// renderConnectionListTab 渲染活跃/历史连接 tab（表格独占）
func renderConnectionListTab(state PageState, content []string) string {
	// 样式定义 — Tokyo Night
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(common.TokyoBlue())
	isHistory := state.ViewMode == ConnViewHistory
	var normalStyle lipgloss.Style
	if isHistory {
		normalStyle = lipgloss.NewStyle().Foreground(common.TokyoMuted())
	} else {
		normalStyle = lipgloss.NewStyle().Foreground(common.TokyoForeground())
	}
	selectedStyle := lipgloss.NewStyle().Background(common.TokyoSelected()).Foreground(common.TokyoCyan()).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted())

	// 根据视图模式选择数据源
	var connList []model.Connection
	if state.ViewMode == ConnViewActive {
		if state.Connections == nil {
			return i18n.T("conns.loading")
		}
		connList = state.Connections.Connections
	} else {
		connList = state.ClosedConnections
	}

	// 过滤连接
	filteredConns := filterConnections(connList, state.FilterText)

	// 过滤输入框
	filterLine := ""
	if state.FilterMode {
		filterLine = state.FilterInput
	} else if state.FilterText != "" {
		filterLine = dimStyle.Render(i18n.Tf("conns.filter_inactive", state.FilterText))
	}

	// 表头
	tableHeader := components.RenderTableHeader(headerStyle, state.Width)

	// 计算使用的行数（表格独占，不含图表/TopN/站点）
	usedLines := connectionsBaseUsedLines
	if filterLine != "" {
		usedLines++
	}

	// 计算列表可显示的行数
	maxDisplay := state.Height - usedLines
	if maxDisplay < 3 {
		maxDisplay = 3
	}
	if usedLines+maxDisplay > state.Height {
		maxDisplay = state.Height - usedLines
		if maxDisplay < 1 {
			maxDisplay = 1
		}
	}

	var detailContent string
	if state.InlineMode && state.ViewMode != ConnViewTraffic && len(filteredConns) > 0 {
		selectedIdx := state.SelectedIndex
		if selectedIdx >= len(filteredConns) {
			selectedIdx = len(filteredConns) - 1
		}
		if selectedIdx < 0 {
			selectedIdx = 0
		}
		conn := &filteredConns[selectedIdx]

		detailHeight := maxDisplay / 2
		if detailHeight < 5 {
			detailHeight = 5
		}

		maxDisplay -= detailHeight
		if maxDisplay < 3 {
			maxDisplay = 3
		}

		detailContent = components.RenderConnectionDetailInline(
			conn, state.Width, detailHeight, state.InlineFocused, state.InlineScroll,
		)
	}

	// 连接列表
	var rows []string
	if len(filteredConns) == 0 {
		rows = append(rows, dimStyle.Render(i18n.T("conns.empty_active")))
	} else {
		selectedIdx := state.SelectedIndex
		if selectedIdx >= len(filteredConns) {
			selectedIdx = len(filteredConns) - 1
		}
		if selectedIdx < 0 {
			selectedIdx = 0
		}

		scrollTop := state.ScrollTop
		if selectedIdx >= scrollTop+maxDisplay {
			scrollTop = selectedIdx - maxDisplay + 1
		}
		if selectedIdx < scrollTop {
			scrollTop = selectedIdx
		}

		endIdx := scrollTop + maxDisplay
		if endIdx > len(filteredConns) {
			endIdx = len(filteredConns)
		}

		for i := scrollTop; i < endIdx; i++ {
			conn := filteredConns[i]
			isSelected := i == selectedIdx

			rowStyle := normalStyle
			prefix := common.SymbolSelectInactive
			if isSelected {
				rowStyle = selectedStyle
				prefix = common.SymbolSelectActive
			}

			row := components.RenderConnectionRow(conn, rowStyle, prefix, state.Width)
			rows = append(rows, row)
		}

		if scrollTop > 0 {
			rows = append([]string{dimStyle.Render(i18n.Tf("conns.scroll_up", scrollTop))}, rows...)
		}
		if endIdx < len(filteredConns) {
			rows = append(rows, dimStyle.Render(i18n.Tf("conns.scroll_down", len(filteredConns)-endIdx)))
		}
	}

	// 补齐空行以固定列表高度，防止底部详情上移（崩塌）
	for len(rows) < maxDisplay {
		rows = append(rows, "")
	}

	if filterLine != "" {
		content = append(content, filterLine)
	}
	content = append(content, tableHeader)
	content = append(content, common.TableBorderStyle().Render(strings.Repeat("─", max(state.Width-2, 1))))
	content = append(content, strings.Join(rows, "\n"))

	if detailContent != "" {
		content = append(content, detailContent)
	}

	return strings.Join(content, "\n")
}

// filterConnections 过滤连接
func filterConnections(connections []model.Connection, filter string) []model.Connection {
	if filter == "" {
		return connections
	}

	filter = strings.ToLower(filter)
	var filtered []model.Connection
	for _, conn := range connections {
		// 搜索主机、规则、代理链
		if strings.Contains(strings.ToLower(conn.Metadata.Host), filter) ||
			strings.Contains(strings.ToLower(conn.Rule), filter) ||
			containsAnyLower(conn.Chains, filter) ||
			strings.Contains(strings.ToLower(conn.Metadata.DestinationIP), filter) {
			filtered = append(filtered, conn)
		}
	}
	return filtered
}

// containsAnyLower 检查字符串切片中是否有包含子串的元素
func containsAnyLower(slice []string, sub string) bool {
	for _, s := range slice {
		if strings.Contains(strings.ToLower(s), sub) {
			return true
		}
	}
	return false
}
