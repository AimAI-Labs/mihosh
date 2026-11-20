package connections

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

const (
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

	if state.ViewMode == ConnViewActive && state.Connections == nil {
		return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}
	}
	return layoutConnectionList(state).hit(pageX, pageY, state.Width)
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
	topLine := borderStyle.Render(theme.Frame().TopLeft + strings.Repeat("─", innerWidth) + theme.Frame().TopRight)
	middleLine := borderStyle.Render("│") + content + borderStyle.Render("│")
	bottomLine := borderStyle.Render(theme.Frame().BottomLeft + strings.Repeat("─", innerWidth) + theme.Frame().BottomRight)

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
	if state.Unlock.open {
		return []common.InlineHelpHint{
			{Key: "S", Desc: i18n.T("conns.unlock.run_all"), Tone: common.InlineHelpToneSuccess},
			{Key: "↵", Desc: i18n.T("conns.unlock.run_one")},
			{Key: "Esc", Desc: i18n.T("conns.unlock.close")},
		}
	}
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
			{Key: "↵", Desc: i18n.T("help.conns_search.confirm")},
			{Key: "Esc", Desc: i18n.T("help.conns_search.cancel")},
		}
	}

	unlock := common.InlineHelpHint{Key: "U", Desc: i18n.T("conns.unlock.title")}
	// 按 tab 分流
	switch state.ViewMode {
	case ConnViewTraffic:
		var hints []common.InlineHelpHint
		if len(state.SiteTests) > 0 {
			hints = append(hints,
				common.InlineHelpHint{Key: "S", Desc: i18n.T("help.conns.hint_test_all"), Tone: common.InlineHelpToneSuccess},
				common.InlineHelpHint{Key: "s", Desc: i18n.T("help.conns.hint_test"), Tone: common.InlineHelpToneSuccess},
				common.InlineHelpHint{Key: "←→", Desc: i18n.T("help.conns.hint_site")},
			)
		}
		return append(hints,
			common.InlineHelpHint{Key: "h", Desc: i18n.T("help.conns.hint_switch")},
			common.InlineHelpHint{Key: "?", Desc: i18n.T("core.inline.help")},
			unlock,
		)

	case ConnViewActive:
		hints := []common.InlineHelpHint{
			{Key: "↑↓", Desc: i18n.T("help.conns.hint_select")},
			{Key: "↵", Desc: i18n.T("help.conns.hint_detail")},
			{Key: "i", Desc: i18n.T("help.conns.hint_inline")},
			{Key: "x/X", Desc: i18n.T("help.conns.hint_close")},
			{Key: "/", Desc: i18n.T("help.conns.hint_search")},
			{Key: "h", Desc: i18n.T("help.conns.hint_switch")},
		}
		if state.InlineFocused {
			hints[0] = common.InlineHelpHint{Key: "↑↓", Desc: i18n.T("help.conns_detail.scroll")}
			hints = append(hints, common.InlineHelpHint{Key: "Esc", Desc: i18n.T("help.conns_search.cancel")})
		} else {
			hints = append(hints, common.InlineHelpHint{Key: "?", Desc: i18n.T("core.inline.help")})
		}
		return append(hints, unlock)

	case ConnViewHistory:
		hints := []common.InlineHelpHint{
			{Key: "↑↓", Desc: i18n.T("help.conns.hint_select")},
			{Key: "↵", Desc: i18n.T("help.conns.hint_detail")},
			{Key: "i", Desc: i18n.T("help.conns.hint_inline")},
			{Key: "/", Desc: i18n.T("help.conns.hint_search")},
			{Key: "h", Desc: i18n.T("help.conns.hint_switch")},
		}
		if state.InlineFocused {
			hints[0] = common.InlineHelpHint{Key: "↑↓", Desc: i18n.T("help.conns_detail.scroll")}
			hints = append(hints, common.InlineHelpHint{Key: "Esc", Desc: i18n.T("help.conns_search.cancel")})
		} else {
			hints = append(hints, common.InlineHelpHint{Key: "?", Desc: i18n.T("core.inline.help")})
		}
		return append(hints, unlock)
	}

	return nil
}

// InlineHelpHints 返回连接页当前可见的右下角交互菜单条目。
func (s State) InlineHelpHints(chartData *model.ChartData, width, height int) []common.InlineHelpHint {
	return common.FitInlineHints(buildConnectionsInlineHelpHints(s.ToPageState(chartData, width, height)), max(1, width-1))
}

// renderConnectionsInlineHelp 渲染右下角内联帮助面板并叠加到页面上
func renderConnectionsInlineHelp(page string, state PageState) string {
	hints := common.FitInlineHints(buildConnectionsInlineHelpHints(state), max(1, state.Width-1))
	if len(hints) == 0 {
		return page
	}

	body := common.FormatInlineHintRow(hints)
	if state.Height > 0 {
		rows := strings.Split(page, "\n")
		page = strings.Join(rows[:min(len(rows), state.Height)], "\n")
	}
	return common.OverlayHelpAtBottomRight(page, body, state.Width, state.Height)
}
