package connections

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	connectionsSiteCardsTopLine = 1
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
	MouseTargetPauseToggle
	MouseTargetIPMaskToggle
	MouseTargetIPRefresh
	MouseTargetIPDetail
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
		if pageY == connectionsModeSwitchHeight {
			if pageX >= 0 && pageX < state.Width {
				ipLayout := components.LayoutCurrentIPBar(state.CurrentIP, state.Width)
				if ipLayout.IPStartX >= 0 && pageX >= ipLayout.IPStartX && pageX < ipLayout.IPEndX {
					return MouseHit{Target: MouseTargetIPMaskToggle, Index: -1}
				}
				if ipLayout.RefreshStart >= 0 && pageX >= ipLayout.RefreshStart && pageX < ipLayout.RefreshEnd {
					return MouseHit{Target: MouseTargetIPRefresh, Index: -1}
				}
				return MouseHit{Target: MouseTargetIPDetail, Index: -1}
			}
			return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}
		}
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
				leftWidth := x - 1
				toggleStartX := 1 + innerWidth - toggleWidth
				if leftWidth+toggleWidth <= innerWidth {
					if pageX >= toggleStartX && pageX < toggleStartX+toggleWidth {
						return MouseHit{Target: MouseTargetInlineToggle, Index: -1}, true
					}
					pauseWidth := lipgloss.Width(connPauseLabel(state.Paused, state.PausedUpdates))
					if leftWidth+pauseWidth+1+toggleWidth <= innerWidth {
						pauseStartX := toggleStartX - 1 - pauseWidth
						if pageX >= pauseStartX && pageX < pauseStartX+pauseWidth {
							return MouseHit{Target: MouseTargetPauseToggle, Index: -1}, true
						}
					}
				}
			}
		}
		return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}, true
	}

	return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}, false
}

func connPauseLabel(paused bool, pausedUpdates int) string {
	if paused {
		if pausedUpdates > 0 {
			return " " + common.SymbolPause + " " + i18n.Tf("conns.paused_badge_new", pausedUpdates) + " "
		}
		return " " + common.SymbolPause + " " + i18n.T("conns.paused_badge") + " "
	}
	return " " + common.SymbolPause + " " + i18n.T("conns.pause_btn") + " "
}

// RenderConnModeSwitchComponent 渲染连接页面模式切换按钮（带边框，与节点模式切换风格一致）
func RenderConnModeSwitchComponent(viewMode int, width int, inlineMode bool, pauseArgs ...any) string {
	paused := false
	pausedUpdates := 0
	if len(pauseArgs) > 0 {
		if v, ok := pauseArgs[0].(bool); ok {
			paused = v
		}
	}
	if len(pauseArgs) > 1 {
		if v, ok := pauseArgs[1].(int); ok {
			pausedUpdates = v
		}
	}
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
	var pauseStr string
	if viewMode == ConnViewActive || viewMode == ConnViewHistory {
		label := " Inline ◫ "
		if inlineMode {
			toggleStr = activeStyle.Render(label)
		} else {
			toggleStr = inactiveStyle.Render(label)
		}
		pauseLabel := connPauseLabel(paused, pausedUpdates)
		if paused {
			pauseStr = lipgloss.NewStyle().
				Background(theme.Resolve(theme.Current().Warning)).
				Foreground(theme.Resolve(theme.Current().Background)).
				Bold(true).
				Render(pauseLabel)
		} else {
			pauseStr = lipgloss.NewStyle().
				Foreground(common.TokyoMuted()).
				Render(pauseLabel)
		}
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	// 计算内边框宽度
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	// 填充内容到指定宽度，并将 pauseStr + toggleStr 放在最右侧
	contentWidth := lipgloss.Width(content)
	toggleWidth := lipgloss.Width(toggleStr)
	pauseWidth := lipgloss.Width(pauseStr)
	if pauseWidth > 0 && contentWidth+pauseWidth+1+toggleWidth <= innerWidth {
		content += strings.Repeat(" ", innerWidth-contentWidth-pauseWidth-1-toggleWidth) + pauseStr + " " + toggleStr
	} else if contentWidth+toggleWidth <= innerWidth {
		content += strings.Repeat(" ", innerWidth-contentWidth-toggleWidth) + toggleStr
	} else {
		content = common.PadString(ansi.Truncate(content, innerWidth, ""), innerWidth)
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

func (s State) InlineHelpHints(width int) []common.InlineHelpHint {
	if s.dns.open {
		return []common.InlineHelpHint{
			{Key: "↵", Desc: i18n.T("conns.dns.hint_query")},
			{Key: "Tab", Desc: i18n.T("conns.dns.hint_type")},
			{Key: "↑↓", Desc: i18n.T("conns.dns.hint_scroll")},
			{Key: "Esc", Desc: i18n.T("conns.dns.hint_close")},
		}
	}
	if s.unlock.open {
		arrows := "↑↓"
		if unlockColumns(width) > 1 {
			arrows = "↑↓←→"
		}
		return []common.InlineHelpHint{
			{Key: arrows, Desc: i18n.T("help.conns.hint_select")},
			{Key: "↵", Desc: i18n.T("conns.unlock.run_one")},
			{Key: "Esc", Desc: i18n.T("conns.unlock.close")},
		}
	}
	if s.currentIP.modalOpen {
		return []common.InlineHelpHint{
			{Key: "i", Desc: i18n.T("help.conns.hint_ip_mask")},
			{Key: "r", Desc: i18n.T("help.conns.hint_refresh")},
			{Key: "Esc/q", Desc: i18n.T("help.conns_detail.close")},
		}
	}
	// 详情模式：左右面板切换 + 滚动 + 关闭
	if s.connDetailMode {
		return []common.InlineHelpHint{
			{Key: "↑/↓", Desc: i18n.T("help.conns_detail.scroll")},
			{Key: "←/→", Desc: i18n.T("help.conns_detail.switch_panel")},
			{Key: "Esc/q", Desc: i18n.T("help.conns_detail.close")},
		}
	}

	// TopN 弹窗：滚动 + 关闭
	if s.topNModalMode {
		return []common.InlineHelpHint{
			{Key: "↑/↓", Desc: i18n.T("help.conns_topn.scroll")},
			{Key: "Esc/q", Desc: i18n.T("help.conns_topn.close")},
		}
	}

	// 过滤输入模式：确认 + 取消
	if s.filterList.FilterMode() {
		return []common.InlineHelpHint{
			{Key: "↵", Desc: i18n.T("help.conns_search.confirm")},
			{Key: "Esc", Desc: i18n.T("help.conns_search.cancel")},
		}
	}

	unlock := common.InlineHelpHint{Key: "U", Desc: i18n.T("conns.unlock.title")}
	dnsHint := common.InlineHelpHint{Key: "D", Desc: i18n.T("conns.dns.title"), Tone: common.InlineHelpToneInfo}
	// 按 tab 分流
	switch s.connViewMode {
	case ConnViewTraffic:
		var hints []common.InlineHelpHint
		if len(s.siteTests) > 0 {
			hints = append(hints,
				common.InlineHelpHint{Key: "S", Desc: i18n.T("help.conns.hint_test_all"), Tone: common.InlineHelpToneSuccess},
				common.InlineHelpHint{Key: "s", Desc: i18n.T("help.conns.hint_test"), Tone: common.InlineHelpToneSuccess},
				common.InlineHelpHint{Key: "←→", Desc: i18n.T("help.conns.hint_site")},
			)
		}
		return append(hints,
			common.InlineHelpHint{Key: "i", Desc: i18n.T("help.conns.hint_ip_mask")},
			common.InlineHelpHint{Key: "I", Desc: i18n.T("help.conns.hint_ip_detail"), Tone: common.InlineHelpToneInfo},
			common.InlineHelpHint{Key: "h", Desc: i18n.T("help.conns.hint_switch")},
			common.InlineHelpHint{Key: "?", Desc: i18n.T("core.inline.help")},
			dnsHint,
			unlock,
		)

	case ConnViewActive:
		pauseHint := common.InlineHelpHint{Key: "p", Desc: i18n.T("help.conns.hint_pause")}
		if s.paused {
			pauseHint = common.InlineHelpHint{Key: "p", Desc: i18n.T("help.conns.hint_resume"), Tone: common.InlineHelpToneWarning}
		}
		closeHint := common.InlineHelpHint{Key: "x/X", Desc: i18n.T("help.conns.hint_close")}
		if s.currentQuery().HasEffectiveFilter() {
			closeHint = common.InlineHelpHint{Key: "X", Desc: i18n.T("help.conns.hint_close_matched"), Tone: common.InlineHelpToneDanger}
		}
		hints := []common.InlineHelpHint{
			{Key: "↑↓", Desc: i18n.T("help.conns.hint_select")},
			{Key: "↵", Desc: i18n.T("help.conns.hint_detail")},
			pauseHint,
			{Key: "s", Desc: i18n.T("help.conns.hint_sort"), Tone: common.InlineHelpToneInfo},
			{Key: "t", Desc: i18n.T("help.conns.hint_proto"), Tone: common.InlineHelpToneInfo},
			{Key: "f", Desc: i18n.T("help.conns.hint_field"), Tone: common.InlineHelpToneInfo},
			{Key: "i", Desc: i18n.T("help.conns.hint_inline")},
			closeHint,
			{Key: "/", Desc: i18n.T("help.conns.hint_search")},
			{Key: "h", Desc: i18n.T("help.conns.hint_switch")},
		}
		if s.inlineDetailFocused {
			hints[0] = common.InlineHelpHint{Key: "↑↓", Desc: i18n.T("help.conns_detail.scroll")}
			hints = append(hints, common.InlineHelpHint{Key: "Esc", Desc: i18n.T("help.conns_search.cancel")})
		} else {
			hints = append(hints, common.InlineHelpHint{Key: "?", Desc: i18n.T("core.inline.help")})
		}
		return append(hints, dnsHint, unlock)

	case ConnViewHistory:
		pauseHint := common.InlineHelpHint{Key: "p", Desc: i18n.T("help.conns.hint_pause")}
		if s.paused {
			pauseHint = common.InlineHelpHint{Key: "p", Desc: i18n.T("help.conns.hint_resume"), Tone: common.InlineHelpToneWarning}
		}
		hints := []common.InlineHelpHint{
			{Key: "↑↓", Desc: i18n.T("help.conns.hint_select")},
			{Key: "↵", Desc: i18n.T("help.conns.hint_detail")},
			pauseHint,
			{Key: "s", Desc: i18n.T("help.conns.hint_sort"), Tone: common.InlineHelpToneInfo},
			{Key: "t", Desc: i18n.T("help.conns.hint_proto"), Tone: common.InlineHelpToneInfo},
			{Key: "f", Desc: i18n.T("help.conns.hint_field"), Tone: common.InlineHelpToneInfo},
			{Key: "i", Desc: i18n.T("help.conns.hint_inline")},
			{Key: "/", Desc: i18n.T("help.conns.hint_search")},
			{Key: "h", Desc: i18n.T("help.conns.hint_switch")},
		}
		if s.inlineDetailFocused {
			hints[0] = common.InlineHelpHint{Key: "↑↓", Desc: i18n.T("help.conns_detail.scroll")}
			hints = append(hints, common.InlineHelpHint{Key: "Esc", Desc: i18n.T("help.conns_search.cancel")})
		} else {
			hints = append(hints, common.InlineHelpHint{Key: "?", Desc: i18n.T("core.inline.help")})
		}
		return append(hints, dnsHint, unlock)
	}

	return nil
}
