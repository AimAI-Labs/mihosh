package connections

import (
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

const (
	connectionsBaseUsedLines    = 8
	connectionsMinDisplayRows   = 5
	connectionsSiteCardsTopLine = 2
	connectionsSiteCardHeight   = 5
	connectionsSiteCardMinWidth = 12
	connectionsSiteCardMaxWidth = 20
	connectionsSiteCardOuterPad = 3
	connectionsHeaderTitle      = "连接监控"
	connectionsHeaderGapWidth   = 2
	connectionsTabsGapWidth     = 2
)

// MouseTarget 表示 connections 页面鼠标命中的组件
type MouseTarget int

const (
	ConnectionsMouseTargetNone MouseTarget = iota
	MouseTargetConnection
	MouseTargetSiteTest
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

	if state.TopNModalMode {
		left, top, right, bottom := components.ResolveTopNModalBounds(state.TopNModalItems, state.Width, state.Height, state.TopNModalScroll)
		if pageX >= left && pageX < right && pageY >= top && pageY < bottom {
			// 点击在弹窗内。
			// 1(border) + 1(padding) + 2(title area) = 4
			localY := pageY - top - 4
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
	if state.ViewMode == 0 && state.Connections == nil {
		return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}
	}

	line := 2 // 页面标题 + 空行

	if state.ViewMode == 0 {
		if state.ChartData != nil {
			chartsSection := components.RenderChartsSection(state.ChartData, state.Width)
			if chartsSection != "" {
				h := lipgloss.Height(chartsSection)
				if pageY >= line && pageY < line+h {
					return MouseHit{Target: MouseTargetChart}
				}
				line += h + 1 // 图表区域 + 空行
			}
		}
		if len(state.TopNItems) > 0 {
			topNSection := components.RenderTopNSection(state.TopNItems, state.Width)
			if topNSection != "" {
				h := lipgloss.Height(topNSection)
				if pageY >= line && pageY < line+h {
					return MouseHit{Target: MouseTargetTopN}
				}
				line += h + 1 // TopN 区域 + 空行
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
			siteSection := components.RenderSiteTestSection(state.SiteTests, state.SelectedSiteTest, state.Width)
			line += lipgloss.Height(siteSection) + 1 // 网站测速区域 + 空行
		}
	}

	line++ // 统计行
	if state.FilterMode || state.FilterText != "" {
		line++ // 过滤行
	}
	line += 2 // 表头 + 分隔线

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
	if pageY != 0 {
		return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}, false
	}

	activeLabel, historyLabel := connectionTabLabels(state.ViewMode)
	activeStart := lipgloss.Width(connectionsHeaderTitle) + connectionsHeaderGapWidth
	activeEnd := activeStart + lipgloss.Width(activeLabel)
	historyStart := activeEnd + connectionsTabsGapWidth
	historyEnd := historyStart + lipgloss.Width(historyLabel)

	if pageX >= activeStart && pageX < activeEnd {
		return MouseHit{Target: MouseTargetViewActive, Index: -1}, true
	}
	if pageX >= historyStart && pageX < historyEnd {
		return MouseHit{Target: MouseTargetViewHistory, Index: -1}, true
	}

	return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}, false
}

func connectionTabLabels(viewMode int) (activeLabel, historyLabel string) {
	activeLabel = i18n.T("conns.tab_active")
	historyLabel = i18n.T("conns.tab_history")
	if viewMode == ConnViewActive {
		activeLabel = "● " + activeLabel
	} else {
		historyLabel = "● " + historyLabel
	}
	return activeLabel, historyLabel
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

func calcConnectionsMaxDisplay(state PageState) int {
	usedLines := connectionsBaseUsedLines
	if state.ViewMode == 0 {
		if state.ChartData != nil {
			if state.Width < 90 {
				usedLines += 14 // 窄屏堆叠布局
			} else {
				usedLines += 8 // 宽屏并排布局
			}
		}
		if len(state.TopNItems) > 0 {
			usedLines += len(state.TopNItems) + 2
		}
		if len(state.SiteTests) > 0 {
			layoutCols := 4
			if state.Width < 60 {
				layoutCols = 2
			} else if state.Width < 90 {
				layoutCols = 3
			}
			cardRows := (len(state.SiteTests) + layoutCols - 1) / layoutCols
			usedLines += 2 + cardRows*5 + 1
		}
	}
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
	if state.ViewMode == 0 {
		if state.Connections == nil {
			return nil
		}
		return state.Connections.Connections
	}
	return state.ClosedConnections
}
