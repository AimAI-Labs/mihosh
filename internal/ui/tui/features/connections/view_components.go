package connections

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

const (
	connectionsBaseUsedLines    = 9 // 模式切换(3) + 间距(1) + 表头(1) + 分隔线(1) + 底部(3)
	connectionsMinDisplayRows   = 5
	connectionsSiteCardsTopLine = 2
	connectionsSiteCardHeight   = 5
	connectionsSiteCardMinWidth = 12
	connectionsSiteCardMaxWidth = 20
	connectionsSiteCardOuterPad = 3
	connectionsModeSwitchHeight = 3 // 模式切换栏边框高度
)

// MouseTarget 表示 connections 页面鼠标命中的组件
type MouseTarget int

const (
	ConnectionsMouseTargetNone MouseTarget = iota
	MouseTargetConnection
	MouseTargetSiteTest
	MouseTargetViewTraffic
	MouseTargetViewActive
	MouseTargetViewHistory
	MouseTargetChart
	MouseTargetTopN
	MouseTargetTopNModalItem
)

// MouseHit 是 connections 页面鼠标命中结果
type MouseHit struct {
	Target MouseTarget
	Index  int
}

type connectionsListWindow struct {
	ScrollTop   int
	VisibleRows int
	ShowTopHint bool
}

// ResolveMouseHit 根据 pageContent 内的坐标定位命中的连接行/网站卡片。
func ResolveMouseHit(state PageState, pageX, pageY int) MouseHit {
	if pageX < 0 || pageY < 0 {
		return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}
	}

	// 详情模式优先级最高：只有模式切换栏和详情区域
	if state.DetailMode && state.SelectedConnection != nil {
		if hit, ok := resolveViewModeHit(state, pageX, pageY); ok {
			return hit
		}
		return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}
	}

	if state.TopNModalMode {
		left, top, right, bottom := components.ResolveTopNModalBounds(state.TopNModalItems, state.Width, state.Height, state.TopNModalScroll)
		if pageX >= left && pageX < right && pageY >= top && pageY < bottom {
			// 点击在弹窗内。
			// 1(border with title) + 1(padding \n) = 2
			localY := pageY - top - 2
			if localY < 0 {
				return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}
			}

			// 处理向上滚动提示行
			if state.TopNModalScroll > 0 {
				if localY == 0 {
					return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1} // 点击了提示行
				}
				localY--
			}

			if localY >= 0 && localY < len(state.TopNModalItems)-state.TopNModalScroll {
				return MouseHit{
					Target: MouseTargetTopNModalItem,
					Index:  state.TopNModalScroll + localY,
				}
			}
		}
		return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}
	}

	if hit, ok := resolveViewModeHit(state, pageX, pageY); ok {
		return hit
	}

	line := connectionsModeSwitchHeight + 1 // 模式切换(3) + 空行(1)

	if state.ViewMode == ConnViewTraffic {
		if state.ChartData != nil {
			chartsSection := components.RenderChartsSection(state.ChartData, state.Width, calcMaxChartHeight(state))
			if chartsSection != "" {
				h := lipgloss.Height(chartsSection)
				if pageY >= line && pageY < line+h {
					return MouseHit{Target: MouseTargetChart}
				}
				line += h + 1
			}
		}
		if len(state.TopNItems) > 0 {
			topNSection := components.RenderTopNSection(state.TopNItems, state.Width)
			if topNSection != "" {
				h := lipgloss.Height(topNSection)
				if pageY >= line && pageY < line+h {
					return MouseHit{Target: MouseTargetTopN}
				}
				line += h + 1
			}
		}
		if len(state.SiteTests) > 0 {
			siteStart := line
			if idx := resolveSiteTestMouseHit(state, pageX, pageY-siteStart); idx >= 0 {
				return MouseHit{
					Target: MouseTargetSiteTest,
					Index:  idx,
				}
			}
		}
		return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}
	}

	// 活跃/历史连接 tab：仅检测连接列表
	if state.ViewMode == ConnViewActive && state.Connections == nil {
		return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}
	}

	line++ // 表头行
	line++ // 分隔线
	if state.FilterMode || state.FilterText != "" {
		line++ // 过滤行
	}

	filteredConns := filterConnections(connectionsByViewMode(state), state.FilterText)
	if len(filteredConns) == 0 {
		return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}
	}

	window := resolveConnectionsListWindow(state, len(filteredConns))
	dataStart := line
	if window.ShowTopHint {
		dataStart++
	}

	if pageY >= dataStart && pageY < dataStart+window.VisibleRows {
		return MouseHit{
			Target: MouseTargetConnection,
			Index:  window.ScrollTop + (pageY - dataStart),
		}
	}

	return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}
}

func resolveViewModeHit(state PageState, pageX, pageY int) (MouseHit, bool) {
	// 模式切换菜单位于最顶部，占据第 0-2 行（带边框）。
	if pageY >= 0 && pageY < connectionsModeSwitchHeight && pageX >= 0 {
		// 只检测中间行（Y=1）的按钮点击
		if pageY == 1 {
			buttonWidths := []int{
				lipgloss.Width(" " + i18n.T("conns.tab_traffic") + " "),
				lipgloss.Width(" " + i18n.T("conns.tab_active") + " "),
				lipgloss.Width(" " + i18n.T("conns.tab_history") + " "),
			}
			targets := []MouseTarget{
				MouseTargetViewTraffic,
				MouseTargetViewActive,
				MouseTargetViewHistory,
			}
			separatorWidth := 1

			x := 1 // 跳过左边框
			for i, bw := range buttonWidths {
				if pageX >= x && pageX < x+bw {
					return MouseHit{Target: targets[i], Index: -1}, true
				}
				x += bw
				if i < len(buttonWidths)-1 {
					x += separatorWidth
				}
			}
		}
		return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}, true
	}

	return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}, false
}

func connectionTabLabels(viewMode int) (trafficLabel, activeLabel, historyLabel string) {
	trafficLabel = i18n.T("conns.tab_traffic")
	activeLabel = i18n.T("conns.tab_active")
	historyLabel = i18n.T("conns.tab_history")
	switch viewMode {
	case ConnViewTraffic:
		trafficLabel = "● " + trafficLabel
	case ConnViewActive:
		activeLabel = "● " + activeLabel
	case ConnViewHistory:
		historyLabel = "● " + historyLabel
	}
	return trafficLabel, activeLabel, historyLabel
}

// RenderConnModeSwitchComponent 渲染连接页面模式切换按钮（带边框，与节点模式切换风格一致）
func RenderConnModeSwitchComponent(viewMode int, width int) string {
	modes := []struct {
		Label string
		Value int
	}{
		{i18n.T("conns.tab_traffic"), ConnViewTraffic},
		{i18n.T("conns.tab_active"), ConnViewActive},
		{i18n.T("conns.tab_history"), ConnViewHistory},
	}

	activeStyle := lipgloss.NewStyle().
		Background(common.TokyoSelected).
		Foreground(common.TokyoCyan).
		Bold(true)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(common.TokyoBlue)
	separatorStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted)

	var parts []string
	for i, m := range modes {
		label := " " + m.Label + " "
		if viewMode == m.Value {
			parts = append(parts, activeStyle.Render(label))
		} else {
			parts = append(parts, inactiveStyle.Render(label))
		}
		if i < len(modes)-1 {
			parts = append(parts, separatorStyle.Render("│"))
		}
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	// 计算内边框宽度
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	// 填充内容到指定宽度
	contentWidth := lipgloss.Width(content)
	if contentWidth < innerWidth {
		content += strings.Repeat(" ", innerWidth-contentWidth)
	}

	// 渲染带边框的模式切换栏
	borderStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue)
	topLine := borderStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")
	middleLine := borderStyle.Render("│") + content + borderStyle.Render("│")
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")

	return topLine + "\n" + middleLine + "\n" + bottomLine
}

func resolveSiteTestMouseHit(state PageState, pageX int, siteSectionY int) int {
	if siteSectionY < connectionsSiteCardsTopLine || siteSectionY >= connectionsSiteCardsTopLine+connectionsSiteCardHeight {
		return -1
	}

	cardOuterWidth := calcSiteCardOuterWidth(state.Width)
	if cardOuterWidth <= 0 {
		return -1
	}

	idx := pageX / cardOuterWidth
	if idx < 0 || idx >= len(state.SiteTests) {
		return -1
	}
	return idx
}

func calcSiteCardOuterWidth(pageWidth int) int {
	layoutCols := 4
	if pageWidth < 60 {
		layoutCols = 2
	} else if pageWidth < 90 {
		layoutCols = 3
	}
	cardWidth := (pageWidth - 10) / layoutCols
	if cardWidth < connectionsSiteCardMinWidth {
		cardWidth = connectionsSiteCardMinWidth
	}
	if cardWidth > connectionsSiteCardMaxWidth {
		cardWidth = connectionsSiteCardMaxWidth
	}
	return cardWidth + connectionsSiteCardOuterPad
}

func resolveConnectionsListWindow(state PageState, total int) connectionsListWindow {
	if total <= 0 {
		return connectionsListWindow{}
	}

	selected := state.SelectedIndex
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}

	maxDisplay := calcConnectionsMaxDisplay(state)
	scrollTop := state.ScrollTop
	if scrollTop < 0 {
		scrollTop = 0
	}
	if scrollTop >= total {
		scrollTop = total - 1
	}

	if selected >= scrollTop+maxDisplay {
		scrollTop = selected - maxDisplay + 1
	}
	if selected < scrollTop {
		scrollTop = selected
	}

	endIdx := scrollTop + maxDisplay
	if endIdx > total {
		endIdx = total
	}

	return connectionsListWindow{
		ScrollTop:   scrollTop,
		VisibleRows: endIdx - scrollTop,
		ShowTopHint: scrollTop > 0,
	}
}

// calcMaxChartHeight 计算图表区域可用的最大高度。
// 扣除基础布局、TopN、网站测速、过滤器和连接列表最小行数后，剩余空间给图表。
func calcMaxChartHeight(state PageState) int {
	if state.ChartData == nil {
		return 0
	}
	// 流量监控 tab：图表可使用更多空间（不需要为连接列表预留行数）
	if state.ViewMode == ConnViewTraffic {
		otherUsed := connectionsModeSwitchHeight + 2 // 模式切换(3) + 空行(1) + 底部留白(1)
		if len(state.SiteTests) > 0 {
			layoutCols := 4
			if state.Width < 60 {
				layoutCols = 2
			} else if state.Width < 90 {
				layoutCols = 3
			}
			cardRows := (len(state.SiteTests) + layoutCols - 1) / layoutCols
			otherUsed += 2 + cardRows*5 + 1
		}
		if len(state.TopNItems) > 0 {
			otherUsed += len(state.TopNItems) + 2
		}
		return state.Height - otherUsed
	}
	// 活跃/历史 tab：不渲染图表
	return 0
}

func calcConnectionsMaxDisplay(state PageState) int {
	// 活跃/历史 tab：表格独占，不计算图表/TopN/站点卡片占用
	usedLines := connectionsBaseUsedLines
	if state.FilterMode || state.FilterText != "" {
		usedLines++
	}

	maxDisplay := state.Height - usedLines
	if maxDisplay < connectionsMinDisplayRows {
		maxDisplay = connectionsMinDisplayRows
	}
	return maxDisplay
}

func connectionsByViewMode(state PageState) []model.Connection {
	switch state.ViewMode {
	case ConnViewActive:
		if state.Connections == nil {
			return nil
		}
		return state.Connections.Connections
	case ConnViewHistory:
		return state.ClosedConnections
	default:
		return nil
	}
}
