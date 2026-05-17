package connections

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
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
	ViewMode          int                // 0=活跃连接, 1=历史连接
	ClosedConnections []model.Connection // 已关闭的连接历史
	// 网站测速
	SiteTests        []model.SiteTest // 网站测试数据
	SelectedSiteTest int              // 选中的网站索引
	// Top N 排行榜
	TopNItems       []components.TopNItem
	TopNModalMode   bool
	TopNModalItems  []components.TopNItem
	TopNModalScroll int
}

// RenderConnectionsPage 渲染连接监控页面
func RenderConnectionsPage(state PageState) string {
	// 详情模式：渲染连接详情
	if state.DetailMode && state.SelectedConnection != nil {
		return components.RenderConnectionDetailModal(
			state.SelectedConnection,
			state.IPInfo,
			state.Width,
			state.Height,
			state.DetailLeftScroll,
			state.DetailRightScroll,
			state.DetailFocusPanel,
		)
	}
	if state.TopNModalMode {
		return components.RenderTopNModal(state.TopNModalItems, state.Width, state.Height, state.TopNModalScroll)
	}

	// 样式定义
	headerStyle := common.BoldStyle.Foreground(common.CSecondary)
	selectedStyle := common.SelectedStyle
	normalStyle := lipgloss.NewStyle().Foreground(common.CWhite)
	dimStyle := common.MutedStyle

	// 根据视图模式选择数据源
	var connList []model.Connection
	var viewModeLabel string
	if state.ViewMode == 0 {
		// 活跃连接
		if state.Connections == nil {
			return i18n.T("conns.loading")
		}
		connList = state.Connections.Connections
		viewModeLabel = headerStyle.Render(i18n.T("conns.active_active")) + dimStyle.Render(i18n.T("conns.history_inactive"))
	} else {
		// 历史连接
		connList = state.ClosedConnections
		viewModeLabel = dimStyle.Render(i18n.T("conns.active_inactive")) + headerStyle.Render(i18n.T("conns.history_active"))
	}

	// 过滤连接
	filteredConns := filterConnections(connList, state.FilterText)

	// 统计信息
	var stats string
	if state.ViewMode == 0 && state.Connections != nil {
		stats = i18n.Tf("conns.stats_active",
			headerStyle.Render(fmt.Sprintf("%d", len(filteredConns))),
			headerStyle.Render(utils.FormatBytes(state.Connections.UploadTotal)),
			headerStyle.Render(utils.FormatBytes(state.Connections.DownloadTotal)),
		)
	} else {
		stats = i18n.Tf("conns.stats_history",
			headerStyle.Render(fmt.Sprintf("%d", len(filteredConns))),
		)
	}

	// 过滤输入框
	filterLine := ""
	if state.FilterMode {
		filterLine = i18n.Tf("conns.filter_active", state.FilterText)
	} else if state.FilterText != "" {
		filterLine = dimStyle.Render(i18n.Tf("conns.filter_inactive", state.FilterText))
	}

	// 表头
	tableHeader := components.RenderTableHeader(headerStyle, state.Width)

	// 计算使用的行数 (Header + Stats + Spacers + TableHeader + Divider + Footer)
	usedLines := connectionsBaseUsedLines

	// 加上图表和测试区域的行数
	if state.ViewMode == 0 {
		if state.ChartData != nil {
			if state.Width < 90 {
				usedLines += 14 // 窄屏堆叠：3个图表×(3行+1间距)+2间距 ≈ 14
			} else {
				usedLines += 8 // 宽屏并排：7行+1间距
			}
		}
		if len(state.SiteTests) > 0 {
			layoutCols := 4
			if state.Width < 60 {
				layoutCols = 2
			} else if state.Width < 90 {
				layoutCols = 3
			}
			cardRows := (len(state.SiteTests) + layoutCols - 1) / layoutCols
			usedLines += 2 + cardRows*5 + 1 // 标题+间距+卡片行+间距
		}
		if len(state.TopNItems) > 0 {
			usedLines += len(state.TopNItems) + 2
		}
	}

	// 加上过滤器行数
	if filterLine != "" {
		usedLines++
	}

	// 计算列表可显示的行数
	maxDisplay := state.Height - usedLines
	if maxDisplay < 3 {
		maxDisplay = 3 // 至少显示 3 行，如果高度实在太小，可能会挤出底部
	}

	// 如果 height 特别小，确保不要溢出
	if usedLines+maxDisplay > state.Height {
		maxDisplay = state.Height - usedLines
		if maxDisplay < 1 {
			maxDisplay = 1
		}
	}

	// 连接列表
	var rows []string
	if len(filteredConns) == 0 {
		rows = append(rows, dimStyle.Render(i18n.T("conns.empty_active")))
	} else {
		// 确保选中索引在有效范围内
		selectedIdx := state.SelectedIndex
		if selectedIdx >= len(filteredConns) {
			selectedIdx = len(filteredConns) - 1
		}
		if selectedIdx < 0 {
			selectedIdx = 0
		}

		// 计算滚动范围
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

		// 显示滚动提示
		if scrollTop > 0 {
			rows = append([]string{dimStyle.Render(i18n.Tf("conns.scroll_up", scrollTop))}, rows...)
		}
		if endIdx < len(filteredConns) {
			rows = append(rows, dimStyle.Render(i18n.Tf("conns.scroll_down", len(filteredConns)-endIdx)))
		}
	}

	// 帮助提示
	var helpText string
	if state.ViewMode == 0 {
		helpText = dimStyle.Render(i18n.T("conns.help_active"))
	} else {
		helpText = dimStyle.Render(i18n.T("conns.help_history"))
	}

	// 组装页面
	var content []string
	content = append(content, headerStyle.Render(i18n.T("title.connections"))+"  "+viewModeLabel)
	content = append(content, "")

	// 渲染监控图表区域（仅在活跃连接视图显示）
	if state.ViewMode == 0 {
		chartsSection := components.RenderChartsSection(state.ChartData, state.Width)
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
			content = append(content, "")
		}
	}

	content = append(content, stats)
	if filterLine != "" {
		content = append(content, filterLine)
	}
	content = append(content, tableHeader)
	content = append(content, common.TableBorderStyle.Render(strings.Repeat("─", max(state.Width-2, 1))))
	content = append(content, strings.Join(rows, "\n"))

	// 统一底部的提示信息，固定到底部
	mainContent := strings.Join(content, "\n")
	contentLines := strings.Count(mainContent, "\n") + 1

	footer := common.RenderFooter(state.Width, state.Height, contentLines, helpText)
	return mainContent + footer
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
