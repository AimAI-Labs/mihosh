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
	MouseTargetInlineToggle
	MouseTargetInlineDetail
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

	maxDisplay, detailHeight := calcConnectionsMaxDisplay(state)
	listEndLine := line + maxDisplay
	if state.InlineMode && state.ViewMode != ConnViewTraffic && detailHeight > 0 {
		if pageY >= listEndLine && pageY < listEndLine+detailHeight {
			return MouseHit{Target: MouseTargetInlineDetail, Index: -1}
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

			if state.ViewMode == ConnViewActive || state.ViewMode == ConnViewHistory {
				toggleLabel := " Inline ◫ "
				toggleWidth := lipgloss.Width(toggleLabel)
				innerWidth := state.Width - 2
				if innerWidth < 1 {
					innerWidth = 1
				}
				toggleStartX := 1 + innerWidth - toggleWidth
				if pageX >= toggleStartX && pageX < toggleStartX+toggleWidth {
					return MouseHit{Target: MouseTargetInlineToggle, Index: -1}, true
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
func RenderConnModeSwitchComponent(viewMode int, width int, inlineMode bool) string {
	modes := []struct {
		Label string
		Value int
	}{
		{i18n.T("conns.tab_traffic"), ConnViewTraffic},
		{i18n.T("conns.tab_active"), ConnViewActive},
		{i18n.T("conns.tab_history"), ConnViewHistory},
	}

	activeStyle := lipgloss.NewStyle().
		Background(common.TokyoSelected()).
		Foreground(common.TokyoCyan()).
		Bold(true)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(common.TokyoBlue())
	separatorStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted())

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

	var toggleStr string
	if viewMode == ConnViewActive || viewMode == ConnViewHistory {
		label := " Inline ◫ "
		if inlineMode {
			toggleStr = activeStyle.Render(label)
		} else {
			toggleStr = inactiveStyle.Render(label)
		}
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	// 计算内边框宽度
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	// 填充内容到指定宽度，并将 toggleStr 放在最右侧
	contentWidth := lipgloss.Width(content)
	toggleWidth := lipgloss.Width(toggleStr)
	if contentWidth+toggleWidth <= innerWidth {
		content += strings.Repeat(" ", innerWidth-contentWidth-toggleWidth) + toggleStr
	}

	// 渲染带边框的模式切换栏
	borderStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue())
	topLine := borderStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")
	middleLine := borderStyle.Render("│") + content + borderStyle.Render("│")
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")

	return topLine + "\n" + middleLine + "\n" + bottomLine
}

func resolveSiteTestMouseHit(state PageState, pageX int, siteSectionY int) int {
	if siteSectionY < connectionsSiteCardsTopLine {
		return -1
	}

	numCards := len(state.SiteTests)
	if numCards == 0 {
		return -1
	}

	cols, cardWidth := components.GetSiteTestLayout(state.Width, numCards)
	cardOuterWidth := cardWidth + connectionsSiteCardOuterPad
	if cardOuterWidth <= 0 {
		return -1
	}

	relativeY := siteSectionY - connectionsSiteCardsTopLine
	rowIdx := relativeY / connectionsSiteCardHeight
	
	rows := (numCards + cols - 1) / cols
	if rowIdx >= rows {
		return -1
	}

	colIdx := pageX / cardOuterWidth
	if colIdx >= cols {
		return -1
	}

	idx := rowIdx*cols + colIdx
	if idx < 0 || idx >= numCards {
		return -1
	}
	return idx
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

	maxDisplay, _ := calcConnectionsMaxDisplay(state)
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
			layoutCols, _ := components.GetSiteTestLayout(state.Width, len(state.SiteTests))
			cardRows := (len(state.SiteTests) + layoutCols - 1) / layoutCols
			otherUsed += 2 + cardRows*connectionsSiteCardHeight + 1
		}
		if len(state.TopNItems) > 0 {
			otherUsed += len(state.TopNItems) + 2
		}
		return state.Height - otherUsed
	}
	// 活跃/历史 tab：不渲染图表
	return 0
}

func calcConnectionsMaxDisplay(state PageState) (int, int) {
	// 活跃/历史 tab：表格独占，不计算图表/TopN/站点卡片占用
	usedLines := connectionsBaseUsedLines
	if state.FilterMode || state.FilterText != "" {
		usedLines++
	}

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

	detailHeight := 0
	// 这里的 len(filteredConns) 判断在实际应用中如果难以获取可以简化，
	// 但是 view.go 中的逻辑是必须 len(filteredConns) > 0 才渲染 inline detail。
	// 为了确保点击区域准确，这里假定只要处于 InlineMode 且不是流量模式，就预留。
	// 因为如果没有连接，整个列表都是空的，也不存在点击问题。
	if state.InlineMode && state.ViewMode != ConnViewTraffic {
		detailHeight = maxDisplay / 2
		if detailHeight < 5 {
			detailHeight = 5
		}
		
		maxDisplay -= detailHeight
		if maxDisplay < 3 {
			maxDisplay = 3
		}
	}

	return maxDisplay, detailHeight
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

// ============================================================
//  内联帮助提示面板（右下角浮层）
// ============================================================
//
// 渲染逻辑（InlineHelpHint / FormatInlineHintRow / OverlayHelpAtBottomRight）
// 共享自 components/common。

// buildConnectionsInlineHelpHints 根据连接页上下文构建内联帮助条目
func buildConnectionsInlineHelpHints(state PageState) []common.InlineHelpHint {
	// 详情模式：左右面板切换 + 滚动 + 关闭
	if state.DetailMode {
		return []common.InlineHelpHint{
			{Key: "↑/↓", Desc: i18n.T("help.conns_detail.scroll")},
			{Key: "←/→", Desc: i18n.T("help.conns_detail.switch_panel")},
			{Key: "Esc/q", Desc: i18n.T("help.conns_detail.close")},
		}
	}

	// TopN 弹窗：滚动 + 关闭
	if state.TopNModalMode {
		return []common.InlineHelpHint{
			{Key: "↑/↓", Desc: i18n.T("help.conns_topn.scroll")},
			{Key: "Esc/q", Desc: i18n.T("help.conns_topn.close")},
		}
	}

	// 过滤输入模式：确认 + 取消
	if state.FilterMode {
		return []common.InlineHelpHint{
			{Key: "Enter", Desc: i18n.T("help.conns_search.confirm")},
			{Key: "Esc", Desc: i18n.T("help.conns_search.cancel")},
		}
	}

	// 按 tab 分流
	switch state.ViewMode {
	case ConnViewTraffic:
		// 流量监控 tab：测速 + 选站 + 切换
		return []common.InlineHelpHint{
			{Key: "h", Desc: i18n.T("help.conns.hint_switch")},
			{Key: "s/S", Desc: i18n.T("help.conns.hint_test")},
			{Key: "←→", Desc: i18n.T("help.conns.hint_site")},
		}

	case ConnViewActive:
		// 活跃连接 tab：选择 + 详情 + 关闭 + 搜索 + 切换
		return []common.InlineHelpHint{
			{Key: "↑↓", Desc: i18n.T("help.conns.hint_select")},
			{Key: "Enter", Desc: i18n.T("help.conns.hint_detail")},
			{Key: "x/X", Desc: i18n.T("help.conns.hint_close")},
			{Key: "/", Desc: i18n.T("help.conns.hint_search")},
			{Key: "h", Desc: i18n.T("help.conns.hint_switch")},
		}

	case ConnViewHistory:
		// 历史连接 tab：选择 + 详情 + 搜索 + 切换
		return []common.InlineHelpHint{
			{Key: "↑↓", Desc: i18n.T("help.conns.hint_select")},
			{Key: "Enter", Desc: i18n.T("help.conns.hint_detail")},
			{Key: "/", Desc: i18n.T("help.conns.hint_search")},
			{Key: "h", Desc: i18n.T("help.conns.hint_switch")},
		}
	}

	return nil
}

// renderConnectionsInlineHelp 渲染右下角内联帮助面板并叠加到页面上
func renderConnectionsInlineHelp(page string, state PageState) string {
	hints := buildConnectionsInlineHelpHints(state)
	if len(hints) == 0 {
		return page
	}

	body := common.FormatInlineHintRow(hints)
	return common.OverlayHelpAtBottomRight(page, body, state.Width, state.Height)
}
