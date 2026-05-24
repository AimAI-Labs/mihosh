package nodes

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const (
	nodesFixedLines     = 12
	nodesMinHeight      = 10
	nodesDefaultNameLen = 8
)

// PageState 节点页面状态（由 Model 传入）
type PageState struct {
	Mode              string
	Groups            map[string]model.Group
	Proxies           map[string]model.Proxy
	GroupNames        []string
	SelectedGroup     int
	SelectedProxy     int
	CurrentProxies    []string
	Testing           bool
	TestingTarget     string
	TestFailures      []string
	ShowFailureDetail bool     // 是否显示测速失败弹窗
	FailureScrollTop  int      // 测速失败弹窗滚动偏移
	SortOrderLabels   []string // 排序选项文本
	CurrentSortOrder  int      // 当前排序模式
	Width             int
	Height            int    // 终端高度
	GroupScrollTop    int    // 策略组列表滚动偏移
	ProxyScrollTop    int    // 节点列表滚动偏移
	FilterText        string // 节点搜索关键词
	FilterMode        bool   // 是否处于搜索输入模式
}

// displayWidth 计算字符串的显示宽度（使用 runewidth 库精确计算）
func displayWidth(s string) int {
	return runewidth.StringWidth(s)
}

// padString 将字符串填充到指定显示宽度
func padString(s string, targetWidth int) string {
	currentWidth := displayWidth(s)
	if currentWidth >= targetWidth {
		return s
	}
	return s + strings.Repeat(" ", targetWidth-currentWidth)
}

// RenderNodesPage 渲染节点管理页面
func RenderNodesPage(state PageState) string {
	metrics := calcNodesLayoutMetrics(state.Width, state.Height)
	modeSwitch := RenderModeSwitchComponent(state.Mode)

	sortLabel := ""
	if len(state.SortOrderLabels) > 0 && state.CurrentSortOrder < len(state.SortOrderLabels) {
		sortLabel = state.SortOrderLabels[state.CurrentSortOrder]
	}

	// 搜索状态提示行
	var searchLine string
	if state.FilterMode {
		searchLine = common.TableHeaderStyle.Render(i18n.Tf("nodes.search_active", state.FilterText))
	} else if state.FilterText != "" {
		searchLine = common.MutedStyle.Render(i18n.Tf("nodes.search_inactive", state.FilterText))
	}

	helpText := common.MutedStyle.Render(i18n.Tf("nodes.help", sortLabel))

	var failureBadge string
	if len(state.TestFailures) > 0 {
		failureBadge = common.ErrorStyle.Render(i18n.Tf("nodes.failure_badge", len(state.TestFailures))) +
			" " + common.MutedStyle.Render(i18n.T("nodes.view_failure"))
	}

	var mainContent string
	if metrics.Wide {
		groupPanel := renderTokyoPanel(
			i18n.Tf("nodes.group_header", state.SelectedGroup+1, len(state.GroupNames)),
			RenderGroupListComponentWidth(state, metrics.GroupMaxLines, metrics.GroupPanelWidth-4),
			metrics.GroupPanelWidth,
		)
		proxyPanel := renderTokyoPanel(
			i18n.Tf("nodes.list_header", state.SelectedProxy+1, len(state.CurrentProxies)),
			RenderProxyListComponentWidth(state, metrics.ProxyMaxLines, metrics.ProxyPanelWidth-4),
			metrics.ProxyPanelWidth,
		)
		panels := lipgloss.JoinHorizontal(lipgloss.Top, groupPanel, strings.Repeat(" ", nodesPanelGap), proxyPanel)
		mainContent = lipgloss.JoinVertical(
			lipgloss.Left,
			modeSwitch,
			panels,
			searchLine,
			failureBadge,
		)
	} else {
		groupList := RenderGroupListComponentWidth(state, metrics.GroupMaxLines, state.Width-6)
		proxyList := RenderProxyListComponentWidth(state, metrics.ProxyMaxLines, state.Width-6)
		mainContent = lipgloss.JoinVertical(
			lipgloss.Left,
			modeSwitch,
			renderTokyoPanel(
				i18n.Tf("nodes.group_header", state.SelectedGroup+1, len(state.GroupNames)),
				groupList,
				state.Width-2,
			),
			"",
			renderTokyoPanel(
				i18n.Tf("nodes.list_header", state.SelectedProxy+1, len(state.CurrentProxies)),
				proxyList,
				state.Width-2,
			),
			searchLine,
			failureBadge,
		)
	}

	contentLines := strings.Count(mainContent, "\n") + 1
	footer := common.RenderFooter(state.Width, state.Height, contentLines, helpText)
	fullPage := mainContent + footer

	if state.ShowFailureDetail {
		modal := buildFailureModal(state)
		return overlayCenter(fullPage, modal, state.Width, state.Height)
	}
	return fullPage
}



// buildFailureModal 构建测速失败详情弹窗字符串
func buildFailureModal(state PageState) string {
	failures := state.TestFailures

	// 弹窗内容区宽度（去掉左右边框各1 + 内边距各1 = 4）
	modalWidth := state.Width - 10
	if modalWidth < 50 {
		modalWidth = 50
	}
	if modalWidth > 100 {
		modalWidth = 100
	}
	innerWidth := modalWidth - 4

	// 可显示的最大行数（去掉标题、分隔线、空行、帮助行 = 4行）
	modalHeight := state.Height - 8
	if modalHeight < 6 {
		modalHeight = 6
	}
	maxDisplay := modalHeight - 4
	if maxDisplay < 1 {
		maxDisplay = 1
	}

	allLines := buildFailureDetailLines(failures, innerWidth)
	if len(allLines) == 0 {
		allLines = []string{i18n.T("nodes.empty_failure")}
	}
	totalLines := len(allLines)

	// 限制滚动范围
	scrollTop := state.FailureScrollTop
	if scrollTop > totalLines-maxDisplay {
		scrollTop = totalLines - maxDisplay
	}
	if scrollTop < 0 {
		scrollTop = 0
	}
	endIdx := scrollTop + maxDisplay
	if endIdx > totalLines {
		endIdx = totalLines
	}

	// 构建内容行
	var bodyLines []string
	if scrollTop > 0 {
		bodyLines = append(bodyLines, common.DimStyle.Render(i18n.Tf("nodes.failure_scroll_up", scrollTop)))
	}
	for _, line := range allLines[scrollTop:endIdx] {
		bodyLines = append(bodyLines, line)
	}
	if endIdx < totalLines {
		bodyLines = append(bodyLines, common.DimStyle.Render(i18n.Tf("nodes.failure_scroll_down", totalLines-endIdx)))
	}
	bodyLines = append(bodyLines, "")
	bodyLines = append(bodyLines, common.MutedStyle.Render(i18n.T("nodes.failure_modal_help")))

	body := strings.Join(bodyLines, "\n")

	title := common.ErrorStyle.Render(i18n.Tf("nodes.failure_modal_title", len(failures)))
	subtitle := common.DimStyle.Render(i18n.T("nodes.failure_modal_subtitle"))
	separator := common.DimStyle.Render(strings.Repeat("─", innerWidth))
	content := lipgloss.JoinVertical(lipgloss.Left, title, subtitle, separator, body)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#E74C3C")).
		Padding(0, 1).
		Width(modalWidth).
		Render(content)
}

func buildFailureDetailLines(failures []string, width int) []string {
	if width < 20 {
		width = 20
	}

	lines := make([]string, 0, len(failures)*6)
	for i, entry := range failures {
		node, raw := splitFailureEntry(entry)
		summary := summarizeFailure(raw)

		lines = append(lines, fmt.Sprintf("[%02d] %s", i+1, node))
		lines = append(lines, wrapWithPrefix(i18n.T("nodes.reason_prefix"), summary, width)...)
		lines = append(lines, wrapWithPrefix(i18n.T("nodes.raw_prefix"), raw, width)...)
		if i < len(failures)-1 {
			lines = append(lines, "")
		}
	}
	return lines
}

func splitFailureEntry(entry string) (node string, raw string) {
	parts := strings.SplitN(strings.TrimSpace(entry), ": ", 2)
	if len(parts) == 2 {
		node = strings.TrimSpace(parts[0])
		raw = strings.TrimSpace(parts[1])
	}
	if node == "" {
		node = i18n.T("nodes.unknown_node")
	}
	if raw == "" {
		raw = strings.TrimSpace(entry)
	}
	if raw == "" {
		raw = i18n.T("nodes.unknown_error")
	}
	return node, raw
}

func summarizeFailure(raw string) string {
	msg := strings.TrimSpace(raw)
	if msg == "" {
		return i18n.T("nodes.unknown_error")
	}

	if detail := extractRequestFailureDetail(msg); detail != "" {
		return detail
	}

	if strings.Contains(msg, "context deadline exceeded") {
		return i18n.T("nodes.timeout_context")
	}
	if strings.Contains(strings.ToLower(msg), "timeout") {
		return i18n.T("nodes.timeout")
	}

	return msg
}

func extractRequestFailureDetail(msg string) string {
	idx := strings.LastIndex(msg, `": `)
	if idx == -1 {
		return ""
	}
	quotedPart := msg[:idx]
	if !strings.Contains(quotedPart, `"http://`) &&
		!strings.Contains(quotedPart, `"https://`) &&
		!strings.Contains(quotedPart, `"socks5://`) {
		return ""
	}
	return strings.TrimSpace(msg[idx+3:])
}

func wrapWithPrefix(prefix, text string, width int) []string {
	prefixWidth := displayWidth(prefix)
	if width <= prefixWidth {
		width = prefixWidth + 1
	}

	parts := wrapByDisplayWidth(text, width-prefixWidth)
	if len(parts) == 0 {
		return []string{prefix}
	}

	indent := strings.Repeat(" ", prefixWidth)
	lines := make([]string, 0, len(parts))
	for i, line := range parts {
		if i == 0 {
			lines = append(lines, prefix+line)
			continue
		}
		lines = append(lines, indent+line)
	}
	return lines
}

func wrapByDisplayWidth(text string, width int) []string {
	if width < 1 {
		width = 1
	}

	var (
		lines []string
		sb    strings.Builder
		w     int
	)

	for _, r := range text {
		if r == '\n' {
			lines = append(lines, sb.String())
			sb.Reset()
			w = 0
			continue
		}

		rw := runewidth.RuneWidth(r)
		if rw < 0 {
			rw = 0
		}
		if w > 0 && w+rw > width {
			lines = append(lines, sb.String())
			sb.Reset()
			w = 0
		}

		sb.WriteRune(r)
		w += rw
	}

	if sb.Len() > 0 || len(lines) == 0 {
		lines = append(lines, sb.String())
	}

	return lines
}

// overlayCenter 将弹窗字符串居中叠加在底层页面上
func overlayCenter(base, overlay string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// 补齐底层行数到 height
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	overlayH := len(overlayLines)
	overlayW := 0
	for _, l := range overlayLines {
		if w := displayWidth(l); w > overlayW {
			overlayW = w
		}
	}

	// 计算叠加起始位置（居中）
	startRow := (height - overlayH) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (width - overlayW) / 2
	if startCol < 0 {
		startCol = 0
	}

	result := make([]string, len(baseLines))
	copy(result, baseLines)

	for i, ol := range overlayLines {
		row := startRow + i
		if row >= len(result) {
			break
		}
		bl := result[row]
		// 将底层行按显示宽度截断到 startCol，然后拼上弹窗行
		blRunes := []rune(bl)
		w, col := 0, 0
		for col < len(blRunes) {
			cw := runewidth.RuneWidth(blRunes[col])
			if w+cw > startCol {
				break
			}
			w += cw
			col++
		}
		prefix := string(blRunes[:col])
		// 补齐空格到 startCol
		if w < startCol {
			prefix += strings.Repeat(" ", startCol-w)
		}
		result[row] = prefix + ol
	}

	return strings.Join(result, "\n")
}
