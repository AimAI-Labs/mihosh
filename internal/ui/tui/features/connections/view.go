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
	Paused          bool // 是否暂停实时刷新
	PausedUpdates   int  // 暂停期间收到的后台更新次数
	Query           ConnQuery
	Unlock          unlockView
	DNS             dnsQueryView
	CurrentIP       components.CurrentIPBarState
}

// RenderConnectionsPage 渲染连接监控页面
func RenderConnectionsPage(state PageState) string {
	// 详情模式优先级最高：渲染沉浸式连接详情（位于模式切换栏下方）
	if state.DetailMode && state.SelectedConnection != nil {
		return layoutConnectionDetail(state).content
	}

	if state.TopNModalMode {
		return components.RenderTopNModal(state.TopNModalItems, state.Width, state.Height, state.TopNModalScroll)
	}

	var page string
	if state.ViewMode == ConnViewTraffic {
		modeSwitch := RenderConnModeSwitchComponent(state.ViewMode, state.Width, state.InlineMode, state.Paused, state.PausedUpdates)
		ipBar := components.RenderCurrentIPBar(state.CurrentIP, state.Width)
		page = renderTrafficTab(state, []string{modeSwitch, ipBar})
	} else {
		page = renderConnectionListTab(state)
	}
	if state.DNS.open {
		popup, _ := state.DNS.popup(state.Width, state.Height)
		page = popup.Overlay(page, true)
	} else if state.Unlock.open {
		popup, _ := state.Unlock.popup(state.Width, state.Height)
		page = popup.Overlay(page, true)
	}
	return page
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
func renderConnectionListTab(state PageState) string {
	if state.ViewMode == ConnViewActive && state.Connections == nil {
		return i18n.T("conns.loading")
	}
	layout := layoutConnectionList(state)
	normalStyle := lipgloss.NewStyle().Foreground(common.TokyoForeground())
	if state.ViewMode == ConnViewHistory {
		normalStyle = normalStyle.Foreground(common.TokyoMuted())
	}
	selectedStyle := lipgloss.NewStyle().Background(common.TokyoSelected()).Foreground(common.TokyoCyan()).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted())

	var rows []string
	if len(layout.connections) == 0 {
		rows = append(rows, dimStyle.Render(i18n.T("conns.empty_active")))
	} else {
		if layout.scrollTop > 0 {
			rows = append(rows, dimStyle.Render(i18n.Tf("conns.scroll_up", layout.scrollTop)))
		}
		for i := layout.scrollTop; i < layout.end; i++ {
			rowStyle, prefix := normalStyle, common.SymbolSelectInactive
			if i == layout.selected {
				rowStyle, prefix = selectedStyle, common.SymbolSelectActive
			}
			rows = append(rows, components.RenderConnectionRow(layout.connections[i], rowStyle, prefix, state.Width))
		}
		if layout.end < len(layout.connections) {
			rows = append(rows, dimStyle.Render(i18n.Tf("conns.scroll_down", len(layout.connections)-layout.end)))
		}
	}
	for len(rows) < layout.listHeight {
		rows = append(rows, "")
	}
	content := layout.header + "\n" + strings.Join(rows, "\n")
	if layout.detailHeight > 0 {
		content += "\n" + components.RenderConnectionDetailInline(
			&layout.connections[layout.selected], state.Width, layout.detailHeight, state.InlineFocused, state.InlineScroll,
		)
	}
	return content
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
