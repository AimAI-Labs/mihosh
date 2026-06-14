package nodes

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

const (
	nodesFixedLines     = 11 // 模式切换(1) + 面板标题(2*2) + 搜索行(1) + 失败行(1) + 间距(4)
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
	TestResults       []TestResultEntry
	ShowTestDetail    bool     // 是否显示测速结果弹窗
	DetailScrollTop   int      // 测速结果弹窗滚动偏移
	SortOrderLabels   []string // 排序选项文本
	CurrentSortOrder  int      // 当前排序模式
	Width             int
	Height            int    // 终端高度
	GroupScrollTop    int    // 策略组列表滚动偏移
	ProxyScrollTop    int    // 节点列表滚动偏移
	FilterText        string // 节点搜索关键词
	FilterMode        bool   // 是否处于搜索输入模式
	FilterEngine      FilterEngine
}

// displayWidth 委托给 common.DisplayWidth
func displayWidth(s string) int {
	return common.DisplayWidth(s)
}

// padString 委托给 common.PadString
func padString(s string, targetWidth int) string {
	return common.PadString(s, targetWidth)
}

// RenderNodesPage 渲染节点管理页面
func RenderNodesPage(state PageState) string {
	metrics := calcNodesLayoutMetrics(state.Width, state.Height)
	modeSwitch := RenderModeSwitchComponent(state.Mode, state.Width)

	// ── 底部固定行：搜索提示 + 失败徽标 ──────────────────────────────
	// 搜索行始终占一行（空行或内容行），确保其位置固定在底栏上方，
	// 不随策略组/节点面板内容多少而上下漂移。
	var searchLine string
	var engineIndicator string
	switch state.FilterEngine {
	case FilterEngineRegex:
		engineIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).Render(" [RE]")
	case FilterEngineFuzzy:
		engineIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render(" [Fuzzy]")
	}

	if state.FilterMode {
		searchLine = common.TableHeaderStyle.Render(i18n.Tf("nodes.search_active", state.FilterText)) + engineIndicator
	} else if state.FilterText != "" {
		searchLine = common.MutedStyle.Render(i18n.Tf("nodes.search_inactive", state.FilterText)) + engineIndicator
	} else {
		searchLine = "" // 占位空行由 clampPanelArea 负责填充
	}

	var resultBadge string
	if len(state.TestResults) > 0 {
		failCount := 0
		for _, r := range state.TestResults {
			if r.Error != "" {
				failCount++
			}
		}
		badgeText := i18n.Tf("nodes.result_badge", len(state.TestResults))
		if failCount > 0 {
			badgeText += " " + common.ErrorStyle.Render(i18n.Tf("nodes.result_fail_count", failCount))
		}
		resultBadge = common.MutedStyle.Render(badgeText) +
			" " + common.MutedStyle.Render(i18n.T("nodes.view_detail"))
	}

	// 底部固定区域占用的行数：搜索行(1) + 结果徽标(0或1)
	bottomLines := 1 // 搜索行始终占 1 行
	if resultBadge != "" {
		bottomLines++
	}

	// panel 区域可用高度 = 总高度 - 底部固定行数
	panelAreaHeight := state.Height - bottomLines
	if panelAreaHeight < nodesMinHeight {
		panelAreaHeight = nodesMinHeight
	}

	// ── 面板区域渲染 ─────────────────────────────────────────────────
	var panelArea string
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
		panelArea = lipgloss.JoinVertical(lipgloss.Left, modeSwitch, "", panels)
	} else {
		groupList := RenderGroupListComponentWidth(state, metrics.GroupMaxLines, state.Width-6)
		proxyList := RenderProxyListComponentWidth(state, metrics.ProxyMaxLines, state.Width-6)
		panelArea = lipgloss.JoinVertical(
			lipgloss.Left,
			modeSwitch,
			"",
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
		)
	}

	// 将面板区域精确约束到 panelAreaHeight 行，确保底部固定行不随内容漂移
	panelArea = clampToLines(panelArea, panelAreaHeight)

	// ── 拼接：面板区 + 搜索行 + 结果徽标 ────────────────────
	bottomParts := []string{searchLine}
	if resultBadge != "" {
		bottomParts = append(bottomParts, resultBadge)
	}
	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		append([]string{panelArea}, bottomParts...)...,
	)

	if state.ShowTestDetail {
		modal := buildTestResultModal(state)
		return overlayCenter(mainContent, modal, state.Width, state.Height)
	}
	return mainContent
}

// clampToLines 将字符串精确约束为 h 行：不足时末尾补空行，超出时截断。
func clampToLines(content string, h int) string {
	if h <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) < h {
		for len(lines) < h {
			lines = append(lines, "")
		}
	} else if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}



// buildTestResultModal 构建测速结果详情弹窗字符串
func buildTestResultModal(state PageState) string {
	results := state.TestResults

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

	allLines := buildTestResultDetailLines(results, innerWidth)
	if len(allLines) == 0 {
		allLines = []string{i18n.T("nodes.empty_results")}
	}
	totalLines := len(allLines)

	// 限制滚动范围
	scrollTop := state.DetailScrollTop
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
		bodyLines = append(bodyLines, common.DimStyle.Render(i18n.Tf("nodes.result_scroll_up", scrollTop)))
	}
	for _, line := range allLines[scrollTop:endIdx] {
		bodyLines = append(bodyLines, line)
	}
	if endIdx < totalLines {
		bodyLines = append(bodyLines, common.DimStyle.Render(i18n.Tf("nodes.result_scroll_down", totalLines-endIdx)))
	}
	bodyLines = append(bodyLines, "")
	bodyLines = append(bodyLines, common.MutedStyle.Render(i18n.T("nodes.result_modal_help")))

	body := strings.Join(bodyLines, "\n")

	// 标题：显示总数和失败数
	failCount := 0
	for _, r := range results {
		if r.Error != "" {
			failCount++
		}
	}
	title := common.TableHeaderStyle.Render(i18n.Tf("nodes.result_modal_title", len(results)))
	if failCount > 0 {
		title += " " + common.ErrorStyle.Render(i18n.Tf("nodes.result_fail_count", failCount))
	}
	subtitle := common.DimStyle.Render(i18n.T("nodes.result_modal_subtitle"))
	separator := common.DimStyle.Render(strings.Repeat("─", innerWidth))
	content := lipgloss.JoinVertical(lipgloss.Left, title, subtitle, separator, body)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7aa2f7")).
		Padding(0, 1).
		Width(modalWidth).
		Render(content)
}

func buildTestResultDetailLines(results []TestResultEntry, width int) []string {
	if width < 20 {
		width = 20
	}

	lines := make([]string, 0, len(results)*4)
	for i, entry := range results {
		if entry.Error != "" {
			// 失败条目
			lines = append(lines, common.ErrorStyle.Render(fmt.Sprintf("[%02d] %s", i+1, entry.Name)))
			summary := summarizeFailure(entry.Error)
			lines = append(lines, wrapWithPrefix(i18n.T("nodes.reason_prefix"), summary, width)...)
			rawMsg := entry.Error
			if entry.TestURL != "" {
				rawMsg = fmt.Sprintf("[%s] %s", entry.TestURL, entry.Error)
			}
			lines = append(lines, wrapWithPrefix(i18n.T("nodes.raw_prefix"), rawMsg, width)...)
		} else {
			// 成功条目
			lines = append(lines, fmt.Sprintf("[%02d] %s", i+1, entry.Name))
			delayText := fmt.Sprintf("%dms", entry.Delay)
			lines = append(lines, wrapWithPrefix(i18n.T("nodes.delay_prefix"), delayText, width)...)
			if entry.TestURL != "" {
				lines = append(lines, wrapWithPrefix(i18n.T("nodes.raw_prefix"), entry.TestURL, width)...)
			}
		}
		if i < len(results)-1 {
			lines = append(lines, "")
		}
	}
	return lines
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

		olWidth := displayWidth(ol)
		blWidth := displayWidth(bl)

		// 截取底层行左侧
		leftPart := ansi.Cut(bl, 0, startCol)
		lw := displayWidth(leftPart)
		if lw < startCol {
			leftPart += strings.Repeat(" ", startCol-lw)
		}

		// 截取底层行右侧
		rightPart := ""
		if blWidth > startCol+olWidth {
			rightPart = ansi.Cut(bl, startCol+olWidth, blWidth)
		}

		result[row] = leftPart + ol + rightPart
	}

	return strings.Join(result, "\n")
}
