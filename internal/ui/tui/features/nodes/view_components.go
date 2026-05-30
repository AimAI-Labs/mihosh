package nodes

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

var hexColorRegex = regexp.MustCompile(`#([0-9a-fA-F]{6})`)

func parseHexColor(s string) (lipgloss.Color, bool) {
	match := hexColorRegex.FindString(s)
	if match != "" {
		return lipgloss.Color(match), true
	}
	return lipgloss.Color(""), false
}


const (
	nodesGroupMinLines      = 3
	nodesProxyMinLines      = 5
	nodesSectionHeaderLines = 2 // PageHeaderStyle 文本 + 下边框
	nodesWideBreakpoint     = 100
	nodesPanelGap           = 2
	nodesPanelPadding       = 1
	nodesPanelFrameWidth    = 2
	nodesPanelChromeWidth   = nodesPanelFrameWidth + nodesPanelPadding*2
)

// Tokyo 颜色引用共享常量
var (
	tokyoForeground = common.TokyoForeground
	tokyoMuted      = common.TokyoMuted
	tokyoBlue       = common.TokyoBlue
	tokyoCyan       = common.TokyoCyan
	tokyoGreen      = common.TokyoGreen
	tokyoRed        = common.TokyoRed
	tokyoPanel      = common.TokyoPanel
	tokyoSelected   = common.TokyoSelected
)

// MouseTarget 表示 nodes 页面鼠标命中的列表组件
type MouseTarget int

const (
	MouseTargetNone MouseTarget = iota
	MouseTargetGroup
	MouseTargetProxy
	MouseTargetMode
)

// MouseHit 是 nodes 页面鼠标命中结果
type MouseHit struct {
	Target MouseTarget
	Index  int
}

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
	ListStartY      int
	DataStartY      int
}

// ResolveMouseHit 根据 pageContent 内的 Y 坐标定位命中的策略组/节点行。
func ResolveMouseHit(state PageState, pageX, pageY int) MouseHit {
	metrics := calcNodesLayoutMetrics(state.Width, state.Height)

	// 模式切换菜单位于最顶部，占据 [0, 1] 行。
	// 格式： 规则 │ 全局 │ 直连
	//       ──────────────────────
	if pageY >= 0 && pageY <= 1 {
		// 样式中有 Padding(0, 0, 0, 1)，所以从 X=1 开始
		// 每个选项: 1(padding) + 1(space) + 4(label) + 1(space) + 1(padding) = 8
		// 选项之间有 1 个分隔符 │
		// 所以 index 0: X[1-8], index 1: X[10-17], index 2: X[19-26]
		if pageX >= 1 && pageX < 28 {
			index := (pageX - 1) / 9
			// 简单校验是否点在分隔符上
			if (pageX-1)%9 < 8 {
				if index >= 0 && index < 3 {
					return MouseHit{
						Target: MouseTargetMode,
						Index:  index,
					}
				}
			}
		}
	}

	if metrics.Wide {
		panelDataY := metrics.DataStartY
		if pageY < panelDataY {
			return MouseHit{Target: MouseTargetNone, Index: -1}
		}

		if pageX < metrics.ProxyPanelX {
			return resolvePanelMouseHit(MouseTargetGroup, state.SelectedGroup, state.GroupScrollTop, metrics.GroupMaxLines, len(state.GroupNames), pageY-panelDataY)
		}
		return resolvePanelMouseHit(MouseTargetProxy, state.SelectedProxy, state.ProxyScrollTop, metrics.ProxyMaxLines, len(state.CurrentProxies), pageY-panelDataY)
	}

	// Mode switch uses 2 lines (no empty line after it)
	groupListLines := 1
	groupMaxLines := metrics.GroupMaxLines
	proxyMaxLines := metrics.ProxyMaxLines
	groupListStart := metrics.ListStartY + nodesSectionHeaderLines
	if len(state.GroupNames) > 0 {
		groupWindow := resolveListWindow(state.SelectedGroup, state.GroupScrollTop, groupMaxLines, len(state.GroupNames))
		groupRows := groupWindow.End - groupWindow.ScrollTop
		groupListLines = 1 + groupRows

		groupDataStart := groupListStart + 1 // 跳过表头
		groupDataEnd := groupDataStart + groupRows
		if pageY >= groupDataStart && pageY < groupDataEnd {
			return MouseHit{
				Target: MouseTargetGroup,
				Index:  groupWindow.ScrollTop + (pageY - groupDataStart),
			}
		}
	}

	proxyHeaderStart := groupListStart + groupListLines + 1
	proxyListStart := proxyHeaderStart + nodesSectionHeaderLines
	if len(state.CurrentProxies) > 0 {
		proxyWindow := resolveListWindow(state.SelectedProxy, state.ProxyScrollTop, proxyMaxLines, len(state.CurrentProxies))
		proxyRows := proxyWindow.End - proxyWindow.ScrollTop
		proxyDataStart := proxyListStart + 1
		proxyDataEnd := proxyDataStart + proxyRows
		if pageY >= proxyDataStart && pageY < proxyDataEnd {
			return MouseHit{
				Target: MouseTargetProxy,
				Index:  proxyWindow.ScrollTop + (pageY - proxyDataStart),
			}
		}
	}

	return MouseHit{Target: MouseTargetNone, Index: -1}
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

func CalcNodesListMaxLines(height int) (int, int) {
	availableHeight := height - nodesFixedLines
	if availableHeight < nodesMinHeight {
		availableHeight = nodesMinHeight
	}

	groupMaxLines := availableHeight / 3
	if groupMaxLines < nodesGroupMinLines {
		groupMaxLines = nodesGroupMinLines
	}

	proxyMaxLines := availableHeight - groupMaxLines
	if proxyMaxLines < nodesProxyMinLines {
		proxyMaxLines = nodesProxyMinLines
	}

	return groupMaxLines, proxyMaxLines
}

func calcNodesLayoutMetrics(width, height int) nodesLayoutMetrics {
	if width <= 0 {
		width = 80
	}

	wide := width >= nodesWideBreakpoint
	groupMaxLines, proxyMaxLines := CalcNodesListMaxLines(height)
	metrics := nodesLayoutMetrics{
		Wide:          wide,
		GroupMaxLines: groupMaxLines,
		ProxyMaxLines: proxyMaxLines,
		ListStartY:    2,
		DataStartY:    4,
	}
	if !wide {
		return metrics
	}

	contentWidth := width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}
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

	panelRows := height - 11
	if panelRows < nodesProxyMinLines {
		panelRows = nodesProxyMinLines
	}
	metrics.GroupPanelWidth = groupPanelWidth
	metrics.ProxyPanelWidth = proxyPanelWidth
	metrics.ProxyPanelX = groupPanelWidth + nodesPanelGap
	metrics.GroupMaxLines = panelRows
	metrics.ProxyMaxLines = panelRows
	return metrics
}

func resolveListWindow(selected, scrollTop, maxLines, total int) nodesListWindow {
	if total <= 0 {
		return nodesListWindow{}
	}

	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}

	if scrollTop < 0 {
		scrollTop = 0
	}
	if scrollTop >= total {
		scrollTop = total - 1
	}

	if selected < scrollTop {
		scrollTop = selected
	}
	if selected >= scrollTop+maxLines {
		scrollTop = selected - maxLines + 1
	}
	if scrollTop < 0 {
		scrollTop = 0
	}

	endIdx := scrollTop + maxLines
	if endIdx > total {
		endIdx = total
	}

	return nodesListWindow{
		ScrollTop: scrollTop,
		End:       endIdx,
	}
}

// renderScrollbar 渲染垂直滚动条
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

func RenderGroupListComponent(state PageState, groupMaxLines int) string {
	return RenderGroupListComponentWidth(state, groupMaxLines, 0)
}

func RenderGroupListComponentWidth(state PageState, groupMaxLines, width int) string {
	if len(state.GroupNames) == 0 {
		return tokyoMutedStyle().Render(i18n.T("nodes.loading"))
	}

	nameLen, typeLen, nowLen := calcGroupColumnWidths(state, width)
	if width > 0 && width < 28 {
		width = 28
	}

	// 注意：宽度要扣减左侧指示器占用的 2 列 (┃ ) 和右侧滚动条占用的 2 列 ( │)
	contentWidth := width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	window := resolveListWindow(state.SelectedGroup, state.GroupScrollTop, groupMaxLines, len(state.GroupNames))

	// 头部列名称 (不加 ┃ 指示线前缀，但在最左侧留 2 空格)
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

		// 1. 前导指示竖线 (仅选中行高亮)
		prefix := "  "
		if i == state.SelectedGroup {
			prefix = tokyoCyanStyle().Render("┃ ")
		}

		// 2. 自定义名称着色 (含有十六进制颜色码优先)
		namePart := fitCell(name, nameLen)
		if color, ok := parseHexColor(name); ok {
			namePart = lipgloss.NewStyle().Foreground(color).Render(namePart)
		} else if i == state.SelectedGroup {
			namePart = tokyoCyanStyle().Render(namePart)
		} else {
			namePart = tokyoTextStyle().Render(namePart)
		}

		// 3. 策略组类型列着色
		typePart := fitCell(group.Type, typeLen)
		if i == state.SelectedGroup {
			typePart = tokyoCyanStyle().Render(typePart)
		} else {
			typePart = tokyoTextStyle().Render(typePart)
		}

		// 4. 当前节点列着色 (含有颜色码优先，特殊节点特判)
		nowPart := fitCell(group.Now, nowLen)
		if color, ok := parseHexColor(group.Now); ok {
			nowPart = lipgloss.NewStyle().Foreground(color).Render(nowPart)
		} else if group.Now == "REJECT" {
			nowPart = tokyoRedStyle().Render(nowPart)
		} else if group.Now == "DIRECT" {
			nowPart = tokyoMutedStyle().Render(nowPart)
		} else {
			nowPart = tokyoTextStyle().Render(nowPart)
		}

		// 5. 智能右侧状态灯 ● (优先使用节点颜色码，其次是延迟色)
		dotColor := tokyoMuted
		if color, ok := parseHexColor(group.Now); ok {
			dotColor = color
		} else if group.Now == "REJECT" {
			dotColor = tokyoRed
		} else if group.Now == "DIRECT" {
			dotColor = tokyoMuted
		} else if proxy, exists := state.Proxies[group.Now]; exists && len(proxy.History) > 0 {
			lastDelay := proxy.History[len(proxy.History)-1].Delay
			dotColor = utils.GetDelayColor(lastDelay)
		}
		dotStr := lipgloss.NewStyle().Foreground(dotColor).Render("●")

		// 拼接行内容
		lineContent := namePart + "  " + typePart + "  " + nowPart + " " + dotStr

		// 如果是选中行，整行应用 tokyoSelected 背景色 (前缀 ┃ 不含背景)
		if i == state.SelectedGroup {
			lineContent = tokyoSelectedStyle(contentWidth).Render(lineContent)
		} else {
			// 采用 lipgloss.Width 精准过滤 ANSI 颜色码，补齐空格以使右侧圆点对齐
			pad := contentWidth - lipgloss.Width(lineContent)
			if pad > 0 {
				lineContent += strings.Repeat(" ", pad)
			}
		}

		bar := renderScrollbar(groupMaxLines, len(state.GroupNames), window.ScrollTop, i-window.ScrollTop)
		lines = append(lines, prefix+lineContent+" "+tokyoMutedStyle().Render(bar))
	}

	return tokyoHeaderStyle().Render(header) + "\n" + strings.Join(lines, "\n")
}

func RenderProxyListComponent(state PageState, proxyMaxLines int) string {
	return RenderProxyListComponentWidth(state, proxyMaxLines, 0)
}

func RenderProxyListComponentWidth(state PageState, proxyMaxLines, width int) string {
	if len(state.CurrentProxies) == 0 {
		if state.FilterText != "" {
			return tokyoMutedStyle().Render(i18n.T("nodes.empty_search"))
		}
		return tokyoMutedStyle().Render(i18n.T("nodes.empty_proxies"))
	}

	var currentNode string
	if len(state.GroupNames) > 0 && state.SelectedGroup < len(state.GroupNames) {
		groupName := state.GroupNames[state.SelectedGroup]
		if group, ok := state.Groups[groupName]; ok {
			currentNode = group.Now
		}
	}

	nameLen, delayColWidth, statusColWidth := calcProxyColumnWidths(state, width)
	if width > 0 && width < 28 {
		width = 28
	}

	contentWidth := width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

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

		// 1. 前导指示竖线
		prefix := "  "
		if i == state.SelectedProxy {
			prefix = tokyoCyanStyle().Render("┃ ")
		}

		// 计算级联延迟/颜色码前景色
		var nodeColor lipgloss.Color
		hasCustomColor := false
		if color, ok := parseHexColor(name); ok {
			nodeColor = color
			hasCustomColor = true
		} else if exists && len(proxy.History) > 0 {
			lastEntry := proxy.History[len(proxy.History)-1]
			nodeColor = utils.GetDelayColor(lastEntry.Delay)
		} else if name == currentNode {
			nodeColor = tokyoGreen
		} else {
			nodeColor = tokyoForeground
		}

		// 2. 节点名称着色
		namePart := fitCell(name, nameLen)
		namePart = lipgloss.NewStyle().Foreground(nodeColor).Render(namePart)

		// 3. 延迟列着色
		delayStr := strings.Repeat(" ", delayColWidth)
		if exists && len(proxy.History) > 0 {
			lastEntry := proxy.History[len(proxy.History)-1]
			lastDelay := lastEntry.Delay
			if lastEntry.Error != "" || lastDelay < 0 {
				delayStr = fitCell("Error", delayColWidth)
				delayStr = tokyoRedStyle().Render(delayStr)
			} else if lastDelay >= 0 {
				delayStr = fitCell(fmt.Sprintf("%dms", lastDelay), delayColWidth)
				if hasCustomColor {
					delayStr = lipgloss.NewStyle().Foreground(nodeColor).Render(delayStr)
				} else {
					delayColor := utils.GetDelayColor(lastDelay)
					delayStr = lipgloss.NewStyle().Foreground(delayColor).Render(delayStr)
				}
			}
		}
		if !exists {
			delayStr = tokyoMutedStyle().Render(delayStr)
		}

		// 4. 状态勾号列着色
		status := "  "
		if name == currentNode {
			status = common.SymbolCheck
		}
		if name == currentNode {
			status = tokyoGreenStyle().Render(status)
		} else {
			status = tokyoMutedStyle().Render(status)
		}

		line := namePart + "  " + delayStr + "  " + status
		
		if i == state.SelectedProxy {
			line = tokyoSelectedStyle(contentWidth).Render(line)
		} else {
			// 采用 lipgloss.Width 精准补齐空格，防止 ANSI 字符影响对齐
			pad := contentWidth - lipgloss.Width(line)
			if pad > 0 {
				line += strings.Repeat(" ", pad)
			}
		}

		bar := renderScrollbar(proxyMaxLines, len(state.CurrentProxies), window.ScrollTop, i-window.ScrollTop)
		lines = append(lines, prefix+line+" "+tokyoMutedStyle().Render(bar))
	}

	return tokyoHeaderStyle().Render(header) + "\n" + strings.Join(lines, "\n")
}

// RenderModeSwitchComponent 渲染模式切换按钮
func RenderModeSwitchComponent(currentMode string) string {
	modes := []struct {
		Label string
		Value string
	}{
		{i18n.T("nodes.mode_rule"), "rule"},
		{i18n.T("nodes.mode_global"), "global"},
		{i18n.T("nodes.mode_direct"), "direct"},
	}

	activeStyle := lipgloss.NewStyle().
		Background(tokyoSelected).
		Foreground(tokyoCyan).
		Bold(true).
		Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(tokyoBlue).
		Padding(0, 1)
	borderStyle := lipgloss.NewStyle().Foreground(tokyoMuted)

	var parts []string
	for i, m := range modes {
		text := m.Label
		if strings.ToLower(currentMode) == m.Value {
			parts = append(parts, activeStyle.Render(" "+text+" "))
		} else {
			parts = append(parts, inactiveStyle.Render(" "+text+" "))
		}
		if i < len(modes)-1 {
			parts = append(parts, borderStyle.Render("│"))
		}
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false). // Only bottom border
		BorderForeground(tokyoBlue).
		Padding(0, 0, 0, 1).
		Render(content)
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

	available := width - 10
	if available < 18 {
		available = 18
	}
	typeLen := clampInt(maxTypeLen, 6, 12)
	nowLen := clampInt(maxNowLen, 8, available/2)
	nameLen := available - typeLen - nowLen
	if nameLen < 8 {
		nameLen = 8
		nowLen = available - typeLen - nameLen
		if nowLen < 8 {
			nowLen = 8
		}
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

	nameLen := width - delayColWidth - statusColWidth - 8
	if nameLen < 10 {
		nameLen = 10
	}
	if nameLen > maxNameLen {
		nameLen = maxNameLen
	}
	return nameLen, delayColWidth, statusColWidth
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func fitCell(s string, width int) string {
	return padString(truncateDisplay(s, width), width)
}

func fitLine(s string, width int) string {
	if width <= 0 {
		return s
	}
	return padString(truncateDisplay(s, width), width)
}

func truncateDisplay(s string, width int) string {
	return common.TruncateDisplay(s, width)
}

// Tokyo 样式函数委托给 common 包

func tokyoTextStyle() lipgloss.Style   { return common.TokyoTextStyle() }
func tokyoHeaderStyle() lipgloss.Style { return common.TokyoHeaderStyle() }
func tokyoMutedStyle() lipgloss.Style  { return common.TokyoMutedStyle() }
func tokyoCyanStyle() lipgloss.Style   { return common.TokyoCyanStyle() }
func tokyoGreenStyle() lipgloss.Style  { return common.TokyoGreenStyle() }
func tokyoRedStyle() lipgloss.Style    { return common.TokyoRedStyle() }
func tokyoBlueStyle() lipgloss.Style   { return common.TokyoBlueStyle() }

func tokyoSelectedStyle(width int) lipgloss.Style {
	style := lipgloss.NewStyle().
		Background(common.TokyoSelected).
		Foreground(common.TokyoCyan).
		Bold(true)
	if width > 0 {
		style = style.Width(width)
	}
	return style
}

func renderTokyoPanel(title, body string, width int) string {
	return common.RenderTokyoPanel(title, body, width)
}
