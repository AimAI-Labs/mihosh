package help

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ── 页面类型常量（与 layout 包保持一致，避免循环依赖）──
const (
	PageNodes       = 0
	PageConnections = 1
	PageLogs        = 2
	PageRules       = 3
	PageSettings    = 4
)

// ── 连接页视图模式常量（与 connections 包保持一致）──
const (
	ConnViewTraffic = 0
	ConnViewActive  = 1
	ConnViewHistory = 2
)

// HelpContext 帮助弹窗所需的当前界面上下文
type HelpContext struct {
	CurrentPage int // 当前页面（PageNodes / PageConnections / ...）

	// 连接页子状态
	ConnViewMode int  // ConnViewTraffic / ConnViewActive / ConnViewHistory
	ConnDetail   bool // 连接详情弹窗
	ConnTopN     bool // TopN 弹窗
	ConnFilter   bool // 过滤输入

	// 节点页子状态
	NodesFailureDetail bool // 测速失败详情弹窗
	NodesFilterMode    bool // 搜索输入

	// 日志页子状态
	LogsDetail bool // 日志详情弹窗
	LogsFilter bool // 过滤输入

	// 规则页子状态
	RulesTypeFilter bool // 类型筛选弹窗
	RulesFilter     bool // 过滤输入

	// 设置页子状态
	SettingsEdit     bool // 编辑模式
	SettingsLanguage bool // 当前选中项是语言
}

// keybinding 单条快捷键条目
type keybinding struct {
	key  string
	desc string
}

// section 快捷键分组
type section struct {
	title    string
	bindings []keybinding
}

// buildHelpSections 根据当前上下文动态构建帮助分区
func buildHelpSections(ctx HelpContext) []section {
	var sections []section

	// 1. 全局快捷键（始终显示）
	sections = append(sections, section{
		title: i18n.T("help.section.global"),
		bindings: []keybinding{
			{"?", i18n.T("help.global.toggle")},
			{"Tab / Shift+Tab", i18n.T("help.global.switch")},
			{"1-5", i18n.T("help.global.go")},
			{"r", i18n.T("help.global.refresh")},
			{"q / Ctrl+C", i18n.T("help.global.quit")},
		},
	})

	// 2. 当前页面快捷键
	switch ctx.CurrentPage {
	case PageNodes:
		sections = append(sections, buildNodesSections(ctx)...)
		// 延迟颜色图例仅在节点页显示
		sections = append(sections, section{
			title: i18n.T("help.section.latency_colors"),
			bindings: []keybinding{
				{i18n.T("help.latency.green"), i18n.T("help.latency.green")},
				{i18n.T("help.latency.yellow"), i18n.T("help.latency.yellow")},
				{i18n.T("help.latency.red"), i18n.T("help.latency.red")},
			},
		})
	case PageConnections:
		sections = append(sections, buildConnectionsSections(ctx)...)
	case PageLogs:
		sections = append(sections, buildLogsSections(ctx)...)
	case PageRules:
		sections = append(sections, buildRulesSections(ctx)...)
	case PageSettings:
		sections = append(sections, buildSettingsSections(ctx)...)
	}

	return sections
}

// buildNodesSections 节点页帮助分区
func buildNodesSections(ctx HelpContext) []section {
	if ctx.NodesFilterMode {
		return []section{{
			title: i18n.T("help.section.nodes_search"),
			bindings: []keybinding{
				{"输入字符", i18n.T("help.nodes_search.append")},
				{"Backspace", i18n.T("help.nodes_search.backspace")},
				{"Ctrl+R", i18n.T("help.nodes_search.regex")},
				{"Ctrl+F", i18n.T("help.nodes_search.fuzzy")},
				{"Enter", i18n.T("help.nodes_search.confirm")},
				{"Esc", i18n.T("help.nodes_search.cancel")},
			},
		}}
	}

	if ctx.NodesFailureDetail {
		return []section{{
			title: i18n.T("help.section.nodes_failure"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.nodes_failure.scroll")},
				{"Home / End", i18n.T("help.nodes_failure.jump")},
				{"f / Esc", i18n.T("help.nodes_failure.close")},
			},
		}}
	}

	return []section{{
		title: i18n.T("help.section.nodes_mgr"),
		bindings: []keybinding{
			{"↑/↓  k/j", i18n.T("help.nodes.select")},
			{"←/→  h/l", i18n.T("help.nodes.switch_group")},
			{"Enter", i18n.T("help.nodes.switch_node")},
			{"t", i18n.T("help.nodes.test")},
			{"a", i18n.T("help.nodes.test_all")},
			{"m", i18n.T("help.nodes.mode")},
			{"s", i18n.T("help.nodes.sort")},
			{"/", i18n.T("help.nodes.search")},
			{"f", i18n.T("help.nodes.failure_detail")},
		},
	}}
}

// buildConnectionsSections 连接页帮助分区
func buildConnectionsSections(ctx HelpContext) []section {
	if ctx.ConnDetail {
		return []section{{
			title: i18n.T("help.section.conns_detail"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.conns_detail.scroll")},
				{"←/→  h/l", i18n.T("help.conns_detail.switch_panel")},
				{"Esc / q", i18n.T("help.conns_detail.close")},
			},
		}}
	}

	if ctx.ConnTopN {
		return []section{{
			title: i18n.T("help.section.conns_topn"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.conns_topn.scroll")},
				{"Enter", i18n.T("help.conns_topn.detail")},
				{"Esc / q", i18n.T("help.conns_topn.close")},
			},
		}}
	}

	if ctx.ConnFilter {
		return []section{{
			title: i18n.T("help.section.conns_search"),
			bindings: []keybinding{
				{"输入字符", i18n.T("help.conns_search.append")},
				{"Enter", i18n.T("help.conns_search.confirm")},
				{"Esc", i18n.T("help.conns_search.cancel")},
			},
		}}
	}

	switch ctx.ConnViewMode {
	case ConnViewTraffic:
		return []section{{
			title: i18n.T("help.section.conns_traffic"),
			bindings: []keybinding{
				{"h", i18n.T("help.conns_traffic.switch")},
				{"←/→", i18n.T("help.conns_traffic.select_site")},
				{"s", i18n.T("help.conns_traffic.test_site")},
				{"S", i18n.T("help.conns_traffic.test_all")},
			},
		}}

	case ConnViewActive:
		return []section{{
			title: i18n.T("help.section.conns_active"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.conns_active.select")},
				{"Enter", i18n.T("help.conns_active.detail")},
				{"x", i18n.T("help.conns_active.close_conn")},
				{"X", i18n.T("help.conns_active.close_all")},
				{"/", i18n.T("help.conns_active.search")},
				{"h", i18n.T("help.conns_active.switch")},
				{"Esc", i18n.T("help.conns_active.clear")},
			},
		}}

	case ConnViewHistory:
		return []section{{
			title: i18n.T("help.section.conns_history"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.conns_history.select")},
				{"Enter", i18n.T("help.conns_history.detail")},
				{"/", i18n.T("help.conns_history.search")},
				{"h", i18n.T("help.conns_history.switch")},
				{"Esc", i18n.T("help.conns_history.clear")},
			},
		}}
	}

	return nil
}

// buildLogsSections 日志页帮助分区
func buildLogsSections(ctx HelpContext) []section {
	if ctx.LogsDetail {
		return []section{{
			title: i18n.T("help.section.logs_detail"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.logs_detail.scroll")},
				{"Esc / q", i18n.T("help.logs_detail.close")},
			},
		}}
	}

	if ctx.LogsFilter {
		return []section{{
			title: i18n.T("help.section.logs_search"),
			bindings: []keybinding{
				{"输入字符", i18n.T("help.logs_search.append")},
				{"Backspace", i18n.T("help.logs_search.backspace")},
				{"Enter", i18n.T("help.logs_search.confirm")},
				{"Esc", i18n.T("help.logs_search.cancel")},
			},
		}}
	}

	return []section{{
		title: i18n.T("help.section.logs"),
		bindings: []keybinding{
			{"↑/↓  k/j", i18n.T("help.logs.select")},
			{"Enter", i18n.T("help.logs.detail")},
			{"[ / ]", i18n.T("help.logs.level_down")},
			{"←/→  h/l", i18n.T("help.logs.scroll")},
			{"/", i18n.T("help.logs.search")},
			{"c", i18n.T("help.logs.clear")},
			{"Esc", i18n.T("help.logs.clear_search")},
		},
	}}
}

// buildRulesSections 规则页帮助分区
func buildRulesSections(ctx HelpContext) []section {
	if ctx.RulesTypeFilter {
		return []section{{
			title: i18n.T("help.section.rules_type_filter"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.rules_filter.select")},
				{"Space", i18n.T("help.rules_filter.toggle")},
				{"输入字符", i18n.T("help.rules_filter.search")},
				{"Backspace", i18n.T("help.rules_filter.backspace")},
				{"Enter", i18n.T("help.rules_filter.confirm")},
				{"Esc", i18n.T("help.rules_filter.cancel")},
			},
		}}
	}

	if ctx.RulesFilter {
		return []section{{
			title: i18n.T("help.section.rules_search"),
			bindings: []keybinding{
				{"输入字符", i18n.T("help.rules_search.append")},
				{"Backspace", i18n.T("help.rules_search.backspace")},
				{"Enter", i18n.T("help.rules_search.confirm")},
				{"Esc", i18n.T("help.rules_search.cancel")},
			},
		}}
	}

	return []section{{
		title: i18n.T("help.section.rules"),
		bindings: []keybinding{
			{"↑/↓  k/j", i18n.T("help.rules.select")},
			{"/", i18n.T("help.rules.search")},
			{"t", i18n.T("help.rules.type")},
			{"Esc", i18n.T("help.rules.clear")},
		},
	}}
}

// buildSettingsSections 设置页帮助分区
func buildSettingsSections(ctx HelpContext) []section {
	if ctx.SettingsEdit {
		if ctx.SettingsLanguage {
			return []section{{
				title: i18n.T("help.section.settings_edit_lang"),
				bindings: []keybinding{
					{"←/→ / Tab", i18n.T("help.settings_edit_lang.switch")},
					{"Enter", i18n.T("help.settings_edit_lang.confirm")},
					{"Esc", i18n.T("help.settings_edit_lang.cancel")},
				},
			}}
		}
		return []section{{
			title: i18n.T("help.section.settings_edit_config"),
			bindings: []keybinding{
				{"←/→", i18n.T("help.settings_edit_config.move")},
				{"Home / End", i18n.T("help.settings_edit_config.jump")},
				{"Backspace", i18n.T("help.settings_edit_config.backspace")},
				{"Delete", i18n.T("help.settings_edit_config.delete")},
				{"Enter", i18n.T("help.settings_edit_config.confirm")},
				{"Esc", i18n.T("help.settings_edit_config.cancel")},
			},
		}}
	}

	return []section{{
		title: i18n.T("help.section.settings"),
		bindings: []keybinding{
			{"↑/↓", i18n.T("help.settings.select")},
			{"Enter / 双击", i18n.T("help.settings.edit")},
		},
	}}
}

// ── 颜色常量 — Tokyo Night ──
var (
	colorBorder  = lipgloss.Color("#7aa2f7") // blue
	colorTitle   = lipgloss.Color("#c0caf5") // foreground
	colorSection = lipgloss.Color("#7dcfff") // cyan
	colorKey     = lipgloss.Color("#e0af68") // yellow
	colorDesc    = lipgloss.Color("#a9b1d6") // comment
	colorDim     = lipgloss.Color("#565f89") // dark5
	colorGreen   = lipgloss.Color("#9ece6a")
	colorYellow  = lipgloss.Color("#e0af68")
	colorRed     = lipgloss.Color("#f7768e")
)

// OverlayHelpPopup 将帮助弹窗居中叠加在 base 页面之上。
//
// 步骤：
//  1. 对 base 每行整体套 Faint，让背景内容降亮但不消失
//  2. 渲染弹窗本体，并计算居中偏移量
//  3. 逐行用 ansi.Cut 截取底层的左侧和右侧，将弹窗内容嵌入中间
func OverlayHelpPopup(base string, width, height int, ctx HelpContext) string {
	// ── 1. 暗化底层 ──
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	if len(baseLines) > height {
		baseLines = baseLines[:height]
	}

	faint := lipgloss.NewStyle().Faint(true)
	dimmed := make([]string, height)
	for i, l := range baseLines {
		dimmed[i] = faint.Render(l)
	}

	// ── 2. 弹窗居中计算 ──
	popup := renderPopup(width, height, ctx)
	popupLines := strings.Split(popup, "\n")
	popupHeight := len(popupLines)
	if popupHeight == 0 {
		return strings.Join(dimmed, "\n")
	}

	popupWidth := lipgloss.Width(popupLines[0])
	leftOffset := (width - popupWidth) / 2
	if leftOffset < 0 {
		leftOffset = 0
	}
	topOffset := (height - popupHeight) / 2
	if topOffset < 0 {
		topOffset = 0
	}

	// ── 3. 弹窗行嵌入暗化底层 ──
	for i, pl := range popupLines {
		y := topOffset + i
		if y >= height {
			break
		}

		leftPart := ansi.Cut(dimmed[y], 0, leftOffset)
		leftW := lipgloss.Width(leftPart)
		if leftW < leftOffset {
			leftPart += strings.Repeat(" ", leftOffset-leftW)
		}

		rightPart := ansi.Cut(dimmed[y], leftOffset+popupWidth, width)
		dimmed[y] = leftPart + pl + rightPart
	}

	return strings.Join(dimmed, "\n")
}

// renderPopup 渲染帮助弹窗（无背景色，使用终端默认背景）
func renderPopup(termWidth, termHeight int, ctx HelpContext) string {
	// ── 动态构建帮助分区 ──
	helpSections := buildHelpSections(ctx)

	// ── 计算每列内容行数（用于智能布局）──
	sectionHeights := make([]int, len(helpSections))
	for i, sec := range helpSections {
		sectionHeights[i] = len(sec.bindings) + 2 // title + bindings + spacing
	}

	// ── 弹窗尺寸 ──
	popupWidth := termWidth * 88 / 100
	if popupWidth > 112 {
		popupWidth = 112
	}
	if popupWidth < 60 {
		popupWidth = 60
	}

	popupHeight := termHeight * 85 / 100
	if popupHeight > 42 {
		popupHeight = 42
	}
	if popupHeight < 20 {
		popupHeight = 20
	}

	// 内容区宽度 = 弹窗宽度 - 边框(2) - 内边距(左右各2=4)
	innerWidth := popupWidth - 6
	if innerWidth < 30 {
		innerWidth = 30
	}

	// ── 智能列数：根据 section 数量和可用宽度决定 ──
	maxCols := 3
	if innerWidth < 80 {
		maxCols = 2
	}
	if innerWidth < 48 {
		maxCols = 1
	}
	cols := maxCols
	if len(helpSections) < cols {
		cols = len(helpSections)
	}
	if cols < 1 {
		cols = 1
	}

	// ── 标题行 ──
	title := lipgloss.NewStyle().Bold(true).Foreground(colorTitle).Render(i18n.T("help.popup_title"))
	closeHint := lipgloss.NewStyle().Foreground(colorDim).Render(i18n.T("help.close_hint"))
	gap := innerWidth - lipgloss.Width(title) - lipgloss.Width(closeHint)
	if gap < 1 {
		gap = 1
	}
	titleLine := title + strings.Repeat(" ", gap) + closeHint

	divider := lipgloss.NewStyle().Foreground(colorDim).Render(strings.Repeat("─", innerWidth))

	// ── 各分区卡片 ──
	colW := innerWidth/cols - 1
	cards := make([]string, 0, len(helpSections))
	for _, sec := range helpSections {
		cards = append(cards, renderSection(sec, colW))
	}

	// ── 按高度贪心分配列（避免内容高度差异过大）──
	colContents := distributeCardsToColumns(cards, sectionHeights, cols, colW)

	var body string
	if len(colContents) == 1 {
		body = colContents[0]
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, colContents...)
	}

	// ── 组装并限高 ──
	content := lipgloss.JoinVertical(lipgloss.Left, titleLine, divider, "", body)

	contentLines := strings.Split(content, "\n")
	maxLines := popupHeight - 4 // 边框2 + 内边距上下各1
	if maxLines < 4 {
		maxLines = 4
	}
	if len(contentLines) > maxLines {
		contentLines = contentLines[:maxLines]
		contentLines = append(contentLines,
			lipgloss.NewStyle().Foreground(colorDim).Render(i18n.T("help.more_hint")),
		)
	}
	for len(contentLines) < maxLines {
		contentLines = append(contentLines, "")
	}
	content = strings.Join(contentLines, "\n")

	// ── 弹窗外壳：圆角边框，无背景色 ──
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2).
		Width(popupWidth)

	return popupStyle.Render(content)
}

// distributeCardsToColumns 将卡片按高度贪心分配到指定列数，每列固定宽度并左对齐
func distributeCardsToColumns(cards []string, heights []int, cols int, colWidth int) []string {
	if len(cards) == 0 {
		return nil
	}
	if cols <= 1 || len(cards) <= 1 {
		return []string{lipgloss.JoinVertical(lipgloss.Left, cards...)}
	}

	// 贪心：每张卡片分配给当前最矮的列
	colCards := make([][]string, cols)
	colHeights := make([]int, cols)
	for i := range colCards {
		colCards[i] = make([]string, 0)
	}

	for i, card := range cards {
		minCol := 0
		for c := 1; c < cols; c++ {
			if colHeights[c] < colHeights[minCol] {
				minCol = c
			}
		}
		colCards[minCol] = append(colCards[minCol], card)
		if i < len(heights) {
			colHeights[minCol] += heights[i]
		}
	}

	// 固定列宽 + 左对齐，确保各列等宽、内容统一左对齐
	colStyle := lipgloss.NewStyle().Width(colWidth).Align(lipgloss.Left)

	var result []string
	for _, cc := range colCards {
		if len(cc) > 0 {
			joined := lipgloss.JoinVertical(lipgloss.Left, cc...)
			result = append(result, colStyle.Render(joined))
		}
	}
	return result
}

// renderSection 渲染单个快捷键分区（无背景色）
func renderSection(sec section, width int) string {
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(colorSection)

	keyWidth := 16
	if width < 38 {
		keyWidth = 12
	}
	keyStyle := lipgloss.NewStyle().Foreground(colorKey).Width(keyWidth)
	descStyle := lipgloss.NewStyle().Foreground(colorDesc)

	var lines []string
	lines = append(lines, sectionStyle.Render(sec.title))

	for _, b := range sec.bindings {
		k := b.key
		desc := b.desc

		if sec.title == i18n.T("help.section.latency_colors") {
			var dot string
			switch {
			case strings.Contains(k, "绿") || strings.Contains(strings.ToLower(k), "green"):
				dot = lipgloss.NewStyle().Foreground(colorGreen).Render("●")
			case strings.Contains(k, "黄") || strings.Contains(strings.ToLower(k), "yellow"):
				dot = lipgloss.NewStyle().Foreground(colorYellow).Render("●")
			case strings.Contains(k, "红") || strings.Contains(strings.ToLower(k), "red"):
				dot = lipgloss.NewStyle().Foreground(colorRed).Render("●")
			default:
				dot = "●"
			}
			lines = append(lines, "  "+dot+" "+descStyle.Render(desc))
			continue
		}

		lines = append(lines, "  "+keyStyle.Render(k)+descStyle.Render(desc))
	}

	lines = append(lines, "") // 分区间距
	return strings.Join(lines, "\n")
}

// RenderHelpPage 旧接口，保留兼容（不传上下文时使用全局+颜色图例）
func RenderHelpPage(width, height int) string {
	return renderPopup(width, height, HelpContext{CurrentPage: PageNodes})
}
