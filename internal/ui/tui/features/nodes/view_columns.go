package nodes

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

const (
	nodesGroupMinLines  = 3
	nodesWideBreakpoint = 100
	nodesPanelGap       = 2
	nodesDefaultNameLen = 8
)

type nodesListWindow struct {
	ScrollTop int
	End       int
}

type nodesLayoutMetrics struct {
	Wide            bool
	GroupPanelWidth int
	ProxyPanelWidth int
	ProxyPanelX     int
	GroupMaxLines   int
	ProxyMaxLines   int
	AreaStartY      int
	DataStartY      int
}

func calcColumnsLayoutMetrics(width, areaH int) nodesLayoutMetrics {
	if width <= 0 {
		width = 80
	}
	if areaH < cardsMinAreaHeight {
		areaH = cardsMinAreaHeight
	}

	wide := width >= nodesWideBreakpoint
	metrics := nodesLayoutMetrics{
		Wide:       wide,
		AreaStartY: cardsModeRows,     // 模式切换(3) + 间距(1)
		DataStartY: cardsModeRows + 2, // 上边框(1) + 列表头(1)
	}
	if !wide {
		// 窄屏垂直堆叠：策略组面板(边框2+表头1) + 间距(1) + 节点面板(边框2+表头1) = 7 行固定开销
		avail := max(2, areaH-7)
		groupMaxLines := min(max(nodesGroupMinLines, avail/3), avail-1)
		proxyMaxLines := avail - groupMaxLines
		metrics.GroupPanelWidth = maxInt(24, width)
		metrics.ProxyPanelWidth = maxInt(24, width)
		metrics.GroupMaxLines = groupMaxLines
		metrics.ProxyMaxLines = proxyMaxLines
		return metrics
	}

	contentWidth := maxInt(40, width)
	groupPanelWidth := contentWidth * 43 / 100
	if groupPanelWidth < 38 {
		groupPanelWidth = 38
	}
	proxyPanelWidth := contentWidth - groupPanelWidth - nodesPanelGap
	if proxyPanelWidth < 42 {
		proxyPanelWidth = 42
		groupPanelWidth = contentWidth - proxyPanelWidth - nodesPanelGap
		if groupPanelWidth < 32 {
			groupPanelWidth = 32
		}
	}

	// 宽屏左右并排：面板上下边框(2) + 列表头(1) = 3 行固定开销
	panelRows := max(1, areaH-3)
	metrics.GroupPanelWidth = groupPanelWidth
	metrics.ProxyPanelWidth = proxyPanelWidth
	metrics.ProxyPanelX = groupPanelWidth + nodesPanelGap
	metrics.GroupMaxLines = panelRows
	metrics.ProxyMaxLines = panelRows
	return metrics
}

func renderColumnsArea(state PageState, areaH int) string {
	metrics := calcColumnsLayoutMetrics(state.Width, areaH)
	groupIndex := 0
	if len(state.GroupNames) > 0 {
		groupIndex = state.SelectedGroup + 1
	}
	proxyIndex := 0
	if len(state.CurrentProxies) > 0 {
		proxyIndex = state.SelectedProxy + 1
	}
	groupTitle := i18n.Tf("nodes.group_header", groupIndex, len(state.GroupNames))
	proxyTitle := i18n.Tf("nodes.list_header", proxyIndex, len(state.CurrentProxies))

	if metrics.Wide {
		groupPanel := common.RenderTokyoPanel(
			groupTitle,
			RenderGroupListComponentWidth(state, metrics.GroupMaxLines, metrics.GroupPanelWidth-4),
			metrics.GroupPanelWidth,
		)
		proxyPanel := common.RenderTokyoPanel(
			proxyTitle,
			RenderProxyListComponentWidth(state, metrics.ProxyMaxLines, metrics.ProxyPanelWidth-4),
			metrics.ProxyPanelWidth,
		)
		return lipgloss.JoinHorizontal(lipgloss.Top, groupPanel, strings.Repeat(" ", nodesPanelGap), proxyPanel)
	}

	groupPanel := common.RenderTokyoPanel(
		groupTitle,
		RenderGroupListComponentWidth(state, metrics.GroupMaxLines, metrics.GroupPanelWidth-4),
		metrics.GroupPanelWidth,
	)
	proxyPanel := common.RenderTokyoPanel(
		proxyTitle,
		RenderProxyListComponentWidth(state, metrics.ProxyMaxLines, metrics.ProxyPanelWidth-4),
		metrics.ProxyPanelWidth,
	)
	return lipgloss.JoinVertical(lipgloss.Left, groupPanel, "", proxyPanel)
}

func resolveColumnsMouseHit(state PageState, pageX, pageY int) MouseHit {
	none := MouseHit{Target: MouseTargetNone, Index: -1}
	areaH := cardAreaHeight(state)
	metrics := calcColumnsLayoutMetrics(state.Width, areaH)
	if pageX < 0 || pageX >= state.Width || pageY < metrics.AreaStartY || pageY >= metrics.AreaStartY+areaH {
		return none
	}

	if metrics.Wide {
		if pageY < metrics.DataStartY {
			return none
		}
		row := pageY - metrics.DataStartY
		if pageX < metrics.GroupPanelWidth {
			return resolvePanelMouseHit(MouseTargetGroup, state.SelectedGroup, state.GroupScrollTop, metrics.GroupMaxLines, len(state.GroupNames), row)
		}
		if pageX >= metrics.ProxyPanelX && pageX < metrics.ProxyPanelX+metrics.ProxyPanelWidth {
			return resolvePanelMouseHit(MouseTargetProxy, state.SelectedProxy, state.ProxyScrollTop, metrics.ProxyMaxLines, len(state.CurrentProxies), row)
		}
		return none
	}

	if len(state.GroupNames) > 0 {
		groupWindow := resolveListWindow(state.SelectedGroup, state.GroupScrollTop, metrics.GroupMaxLines, len(state.GroupNames))
		groupRows := groupWindow.End - groupWindow.ScrollTop
		groupDataStart := metrics.AreaStartY + 2
		if pageY >= groupDataStart && pageY < groupDataStart+groupRows {
			return MouseHit{
				Target: MouseTargetGroup,
				Index:  groupWindow.ScrollTop + (pageY - groupDataStart),
			}
		}
	}

	proxyPanelY := metrics.stackedProxyY(len(state.GroupNames))
	if len(state.CurrentProxies) > 0 {
		proxyWindow := resolveListWindow(state.SelectedProxy, state.ProxyScrollTop, metrics.ProxyMaxLines, len(state.CurrentProxies))
		proxyRows := proxyWindow.End - proxyWindow.ScrollTop
		proxyDataStart := proxyPanelY + 2
		if pageY >= proxyDataStart && pageY < proxyDataStart+proxyRows {
			return MouseHit{
				Target: MouseTargetProxy,
				Index:  proxyWindow.ScrollTop + (pageY - proxyDataStart),
			}
		}
	}

	return none
}

func (m nodesLayoutMetrics) stackedProxyY(groupCount int) int {
	// The group panel has a header (or empty state), two borders and a gap.
	return m.AreaStartY + min(groupCount, m.GroupMaxLines) + 4
}

func (s State) scrollColumns(state PageState, focus nodesMouseFocus, delta, x, y int) State {
	areaH := cardAreaHeight(state)
	metrics := calcColumnsLayoutMetrics(state.Width, areaH)
	if x >= 0 && x < state.Width && y >= metrics.AreaStartY && y < metrics.AreaStartY+areaH {
		focus = nodesMouseFocusProxy
		if metrics.Wide && x < metrics.ProxyPanelX ||
			!metrics.Wide && y < metrics.stackedProxyY(len(state.GroupNames)) {
			focus = nodesMouseFocusGroup
		}
	}
	s.MouseFocus = focus
	if focus == nodesMouseFocusGroup {
		count := len(state.GroupNames)
		s.GroupScrollTop = resolveListWindow(s.SelectedGroup, s.GroupScrollTop, metrics.GroupMaxLines, count).ScrollTop
		s.applyGroupSelection(s.SelectedGroup + delta)
		s.GroupScrollTop = resolveListWindow(s.SelectedGroup, s.GroupScrollTop, metrics.GroupMaxLines, count).ScrollTop
	} else if count := len(state.CurrentProxies); count > 0 {
		top := resolveListWindow(s.SelectedProxy, s.ProxyScrollTop, metrics.ProxyMaxLines, count).ScrollTop
		s.SelectedProxy = clampInt(s.SelectedProxy+delta, 0, count-1)
		s.ProxyScrollTop = resolveListWindow(s.SelectedProxy, top, metrics.ProxyMaxLines, count).ScrollTop
	}
	return s
}

func resolvePanelMouseHit(target MouseTarget, selected, scrollTop, maxLines, total, row int) MouseHit {
	if total <= 0 || row < 0 {
		return MouseHit{Target: MouseTargetNone, Index: -1}
	}
	window := resolveListWindow(selected, scrollTop, maxLines, total)
	rows := window.End - window.ScrollTop
	if row >= rows {
		return MouseHit{Target: MouseTargetNone, Index: -1}
	}
	return MouseHit{
		Target: target,
		Index:  window.ScrollTop + row,
	}
}

func resolveListWindow(selected, scrollTop, maxLines, total int) nodesListWindow {
	if total <= 0 {
		return nodesListWindow{}
	}
	maxLines = max(1, maxLines)
	selected = clampInt(selected, 0, total-1)
	scrollTop = clampInt(scrollTop, 0, max(0, total-maxLines))
	if selected < scrollTop {
		scrollTop = selected
	}
	if selected >= scrollTop+maxLines {
		scrollTop = selected - maxLines + 1
	}
	if scrollTop < 0 {
		scrollTop = 0
	}
	endIdx := minInt(total, scrollTop+maxLines)
	return nodesListWindow{
		ScrollTop: scrollTop,
		End:       endIdx,
	}
}

func renderScrollbar(height, total, scrollTop, currentIdx int) string {
	if total <= height {
		return " "
	}
	barHeight := float64(height) * float64(height) / float64(total)
	if barHeight < 1 {
		barHeight = 1
	}
	barStart := float64(scrollTop) * float64(height) / float64(total)
	if float64(currentIdx) >= barStart && float64(currentIdx) < barStart+barHeight {
		return common.SymbolScrollbarThumb
	}
	return common.SymbolScrollbarTrack
}

// RenderGroupListComponentWidth 渲染双列布局下的策略组列表。
func RenderGroupListComponentWidth(state PageState, groupMaxLines, width int) string {
	if len(state.GroupNames) == 0 {
		return common.TokyoMutedStyle().Render(i18n.T("nodes.loading"))
	}

	nameLen, typeLen, nowLen := calcGroupColumnWidths(state, width)
	if width > 0 && width < 28 {
		width = 28
	}

	// 扣减左侧指示器 2 列 (┃ ) 和右侧滚动条 2 列 ( │)
	contentWidth := maxInt(10, width-4)
	window := resolveListWindow(state.SelectedGroup, state.GroupScrollTop, groupMaxLines, len(state.GroupNames))

	header := fmt.Sprintf("  %s  %s  %s",
		fitCell(i18n.T("nodes.col_name"), nameLen),
		fitCell(i18n.T("nodes.col_type"), typeLen),
		fitCell(i18n.T("nodes.col_now"), nowLen),
	)
	header = fitLine(header, contentWidth)

	lines := make([]string, 0, window.End-window.ScrollTop)
	for i := window.ScrollTop; i < window.End; i++ {
		name := state.GroupNames[i]
		group := state.Groups[name]

		prefix := "  "
		if i == state.SelectedGroup {
			prefix = common.TokyoCyanStyle().Render("┃ ")
		}

		namePart := fitCell(name, nameLen)
		if color, ok := parseHexColor(name); ok {
			namePart = lipgloss.NewStyle().Foreground(theme.Resolve(color)).Render(namePart)
		} else if i == state.SelectedGroup {
			namePart = common.TokyoCyanStyle().Render(namePart)
		} else {
			namePart = common.TokyoTextStyle().Render(namePart)
		}

		typePart := fitCell(group.Type, typeLen)
		if i == state.SelectedGroup {
			typePart = common.TokyoCyanStyle().Render(typePart)
		} else {
			typePart = common.TokyoTextStyle().Render(typePart)
		}

		nowPart := fitCell(group.Now, nowLen)
		if color, ok := parseHexColor(group.Now); ok {
			nowPart = lipgloss.NewStyle().Foreground(theme.Resolve(color)).Render(nowPart)
		} else if group.Now == "REJECT" {
			nowPart = common.TokyoRedStyle().Render(nowPart)
		} else if group.Now == "DIRECT" {
			nowPart = common.TokyoMutedStyle().Render(nowPart)
		} else {
			nowPart = common.TokyoTextStyle().Render(nowPart)
		}

		dotColor := common.TokyoMuted()
		if color, ok := parseHexColor(group.Now); ok {
			dotColor = color
		} else if group.Now == "REJECT" {
			dotColor = common.TokyoRed()
		} else if group.Now == "DIRECT" {
			dotColor = common.TokyoMuted()
		} else if d := resolveNowDelay(state, group); d > 0 {
			dotColor = utils.GetDelayColor(d)
		}
		dotStr := lipgloss.NewStyle().Foreground(theme.Resolve(dotColor)).Render("●")

		lineContent := namePart + "  " + typePart + "  " + nowPart + " " + dotStr
		if i == state.SelectedGroup {
			lineContent = tokyoSelectedStyle(contentWidth).Render(lineContent)
		} else if pad := contentWidth - lipgloss.Width(lineContent); pad > 0 {
			lineContent += strings.Repeat(" ", pad)
		}

		bar := renderScrollbar(groupMaxLines, len(state.GroupNames), window.ScrollTop, i-window.ScrollTop)
		lines = append(lines, prefix+lineContent+" "+common.TokyoMutedStyle().Render(bar))
	}

	return common.TokyoHeaderStyle().Render(header) + "\n" + strings.Join(lines, "\n")
}

// RenderProxyListComponentWidth 渲染双列布局下的节点列表。
func RenderProxyListComponentWidth(state PageState, proxyMaxLines, width int) string {
	if len(state.CurrentProxies) == 0 {
		if state.FilterText != "" {
			return common.TokyoMutedStyle().Render(i18n.T("nodes.empty_search"))
		}
		return common.TokyoMutedStyle().Render(i18n.T("nodes.empty_proxies"))
	}

	var currentNode string
	if len(state.GroupNames) > 0 && state.SelectedGroup >= 0 && state.SelectedGroup < len(state.GroupNames) {
		groupName := state.GroupNames[state.SelectedGroup]
		if group, ok := state.Groups[groupName]; ok {
			currentNode = group.Now
		}
	}

	nameLen, delayColWidth, statusColWidth := calcProxyColumnWidths(state, width)
	if width > 0 && width < 28 {
		width = 28
	}

	contentWidth := maxInt(10, width-4)
	window := resolveListWindow(state.SelectedProxy, state.ProxyScrollTop, proxyMaxLines, len(state.CurrentProxies))

	header := fmt.Sprintf("  %s  %s  %s",
		fitCell(i18n.T("nodes.col_name"), nameLen),
		fitCell(i18n.T("nodes.col_delay"), delayColWidth),
		fitCell(i18n.T("nodes.col_status"), statusColWidth),
	)
	header = fitLine(header, contentWidth)

	lines := make([]string, 0, window.End-window.ScrollTop)
	for i := window.ScrollTop; i < window.End; i++ {
		name := state.CurrentProxies[i]
		proxy, exists := state.Proxies[name]

		prefix := "  "
		if i == state.SelectedProxy {
			prefix = common.TokyoCyanStyle().Render("┃ ")
		}

		var nodeColor lipgloss.Color
		hasCustomColor := false
		if color, ok := parseHexColor(name); ok {
			nodeColor = color
			hasCustomColor = true
		} else if exists && len(proxy.History) > 0 {
			lastEntry := proxy.History[len(proxy.History)-1]
			nodeColor = utils.GetDelayColor(lastEntry.Delay)
		} else if name == currentNode {
			nodeColor = common.TokyoGreen()
		} else {
			nodeColor = common.TokyoForeground()
		}

		namePart := lipgloss.NewStyle().Foreground(theme.Resolve(nodeColor)).Render(fitCell(name, nameLen))

		delayStr := strings.Repeat(" ", delayColWidth)
		if exists && len(proxy.History) > 0 {
			lastEntry := proxy.History[len(proxy.History)-1]
			lastDelay := lastEntry.Delay
			if lastEntry.Error != "" || lastDelay < 0 {
				delayStr = common.TokyoRedStyle().Render(fitCell("Error", delayColWidth))
			} else if lastDelay >= 0 {
				delayText := fitCell(fmt.Sprintf("%dms", lastDelay), delayColWidth)
				if hasCustomColor {
					delayStr = lipgloss.NewStyle().Foreground(theme.Resolve(nodeColor)).Render(delayText)
				} else {
					delayStr = lipgloss.NewStyle().Foreground(theme.Resolve(utils.GetDelayColor(lastDelay))).Render(delayText)
				}
			}
		}
		if !exists {
			delayStr = common.TokyoMutedStyle().Render(delayStr)
		}

		status := "  "
		if name == currentNode {
			status = common.TokyoGreenStyle().Render(common.SymbolCheck)
		} else {
			status = common.TokyoMutedStyle().Render(status)
		}

		line := namePart + "  " + delayStr + "  " + status
		if i == state.SelectedProxy {
			line = tokyoSelectedStyle(contentWidth).Render(line)
		} else if pad := contentWidth - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}

		bar := renderScrollbar(proxyMaxLines, len(state.CurrentProxies), window.ScrollTop, i-window.ScrollTop)
		lines = append(lines, prefix+line+" "+common.TokyoMutedStyle().Render(bar))
	}

	return common.TokyoHeaderStyle().Render(header) + "\n" + strings.Join(lines, "\n")
}

func calcGroupColumnWidths(state PageState, width int) (int, int, int) {
	maxNameLen := nodesDefaultNameLen
	maxTypeLen := nodesDefaultNameLen
	maxNowLen := nodesDefaultNameLen
	for _, name := range state.GroupNames {
		if w := displayWidth(name); w > maxNameLen {
			maxNameLen = w
		}
		group := state.Groups[name]
		if w := displayWidth(group.Type); w > maxTypeLen {
			maxTypeLen = w
		}
		if w := displayWidth(group.Now); w > maxNowLen {
			maxNowLen = w
		}
	}

	if width <= 0 {
		return maxNameLen, maxTypeLen, maxNowLen
	}

	available := maxInt(18, width-10)
	typeLen := clampInt(maxTypeLen, 6, 12)
	nowLen := clampInt(maxNowLen, 8, available/2)
	nameLen := available - typeLen - nowLen
	if nameLen < 8 {
		nameLen = 8
		nowLen = maxInt(8, available-typeLen-nameLen)
	}
	return nameLen, typeLen, nowLen
}

func calcProxyColumnWidths(state PageState, width int) (int, int, int) {
	maxNameLen := nodesDefaultNameLen
	for _, name := range state.CurrentProxies {
		if w := displayWidth(name); w > maxNameLen {
			maxNameLen = w
		}
	}

	delayColWidth := 7
	statusColWidth := 6
	if width <= 0 {
		return maxNameLen, delayColWidth, statusColWidth
	}

	nameLen := maxInt(10, width-delayColWidth-statusColWidth-8)
	if nameLen > maxNameLen {
		nameLen = maxNameLen
	}
	return nameLen, delayColWidth, statusColWidth
}

func fitCell(s string, width int) string {
	return common.PadString(common.TruncateDisplay(s, width), width)
}

func fitLine(s string, width int) string {
	if width <= 0 {
		return s
	}
	return common.PadString(common.TruncateDisplay(s, width), width)
}

func tokyoSelectedStyle(width int) lipgloss.Style {
	style := lipgloss.NewStyle().
		Background(theme.Resolve(common.TokyoSelected())).
		Foreground(theme.Resolve(common.TokyoCyan())).
		Bold(true)
	if width > 0 {
		style = style.Width(width)
	}
	return style
}
