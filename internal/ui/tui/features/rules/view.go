package rules

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	rulesFixedLines   = 5 // 头部组件(3) + 间距(1) + 底部(1)
	rulesMinHeight    = 5
	rulesScrollWidth  = 1
	colorAnimationMs  = 250
	rulesHeaderHeight = 3 // 头部组件边框高度（搜索+统计行 + 上下边框）
)

var (
	domainColorKey       = "Domain"
	domainSuffixColorKey = "DomainSuffix"
)

// 规则类型颜色
var ruleTypeColors = map[string]lipgloss.Color{
	// 标准格式
	"DOMAIN":         common.CSecondary,
	"DOMAIN-SUFFIX":  common.CSecondary,
	"DOMAIN-KEYWORD": common.CInfo,
	"IP-CIDR":        common.CPurple,
	"IP-CIDR6":       common.CPurple,
	"GEOIP":          common.CDanger,
	"GEOSITE":        common.COrange,
	"RULE-SET":       common.CSuccess,
	"MATCH":          common.CWarning,
	"DIRECT":         common.CGray,
	// Clash Meta 驼峰格式
	"Domain":        common.CSecondary,
	"DomainSuffix":  common.CSecondary,
	"DomainKeyword": common.CInfo,
	"IPCIDR":        common.CPurple,
	"IPCIDR6":       common.CPurple,
	"GeoIP":         common.CDanger,
	"GeoSite":       common.COrange,
	"RuleSet":       common.CSuccess,
	"Match":         common.CWarning,
}

// filteredRule 带原始索引的规则
type filteredRule struct {
	Index int        // 原始索引
	Rule  model.Rule // 规则数据
}

// PageState 规则页面状态
type PageState struct {
	Rules               []model.Rule // 规则列表
	FilteredRuleIndices []int        // 过滤后的规则索引
	FilterText          string       // 搜索关键词
	FilterMode          bool         // 是否处于过滤输入模式
	FilterEngine        FilterEngine // 搜索匹配引擎
	SelectedRule        int          // 选中的规则索引
	ScrollTop           int          // 滚动偏移
	Width               int          // 页面宽度
	Height              int          // 页面高度
	ColorAdjustLight    float64      // 较浅色调明度增加比例
	ColorAdjustDark     float64      // 较深色调明度降低比例

	// 类型筛选弹窗状态
	ShowTypeFilter   bool     // 是否显示类型筛选弹窗
	SelectedTypes    []string // 已选择的规则类型
	AvailableTypes   []string // 可用规则类型列表
	TypeFilterCursor int      // 光标位置

	// 添加规则弹窗状态
	ShowAddForm bool    // 是否显示添加规则弹窗
	AddForm     addForm // 表单状态快照
}

// RenderRulesPage 渲染规则页面
func RenderRulesPage(state PageState) string {
	var sections []string

	// 过滤规则 (使用缓存的索引)
	var filteredRules []filteredRule
	for _, idx := range state.FilteredRuleIndices {
		if idx >= 0 && idx < len(state.Rules) {
			filteredRules = append(filteredRules, filteredRule{Index: idx, Rule: state.Rules[idx]})
		}
	}

	// 渲染统计信息
	stats := i18n.Tf("rules.stats", len(filteredRules))
	if state.FilterText != "" || len(state.SelectedTypes) > 0 {
		stats += i18n.Tf("rules.stats_filtered", len(state.Rules))
	}

	// 渲染带边框的头部组件（包含标题、统计和搜索框）
	searchBox := renderRuleSearchBox(state.FilterText, state.FilterMode, state.FilterEngine, state.SelectedTypes)
	header := RenderRulesHeaderComponent(stats, searchBox, state.Width)
	sections = append(sections, header)
	sections = append(sections, "")

	// 计算可显示的规则行数
	availableHeight := state.Height - rulesFixedLines
	if availableHeight < rulesMinHeight {
		availableHeight = rulesMinHeight
	}

	// 渲染规则列表（传入颜色调整参数）
	ruleList := renderRuleList(filteredRules, state.SelectedRule, state.ScrollTop, availableHeight, state.Width, state.ColorAdjustLight, state.ColorAdjustDark)
	sections = append(sections, ruleList)

	mainContent := strings.Join(sections, "\n")
	result := mainContent

	// 如果显示类型筛选弹窗，叠加在页面之上
	if state.ShowTypeFilter {
		result = renderTypeFilterOverlay(result, state, state.Width, state.Height)
	}

	// 如果显示添加规则弹窗，叠加在页面之上（优先级高于类型筛选）
	if state.ShowAddForm {
		result = renderAddRuleOverlay(result, state, state.Width, state.Height)
	}

	// 如果策略选择二级弹窗已打开，叠加在添加规则弹窗之上
	if state.ShowAddForm && state.AddForm.isProxyPickerOpen() {
		result = renderProxyPickerOverlay(result, state, state.Width, state.Height)
	}

	return renderRulesInlineHelp(result, state)
}

// ============================================================
//  内联帮助提示面板（右下角浮层）
// ============================================================
//
// 渲染逻辑（InlineHelpHint / FormatInlineHintRow / OverlayHelpAtBottomRight）
// 共享自 components/common。

// buildRulesInlineHelpHints 根据规则页上下文构建内联帮助条目
func buildRulesInlineHelpHints(state PageState) []common.InlineHelpHint {
	// 策略选择二级弹窗：Tab 切分类 + ↑↓ 移动 + Enter 选中 + Esc 返回
	if state.ShowAddForm && state.AddForm.isProxyPickerOpen() {
		return []common.InlineHelpHint{
			{Key: "Tab", Desc: i18n.T("help.rules_add.picker_tab")},
			{Key: "↑/↓", Desc: i18n.T("help.rules_add.picker_pick")},
			{Key: "Enter", Desc: i18n.T("help.rules_add.confirm")},
			{Key: "Esc", Desc: i18n.T("help.rules_add.picker_back")},
		}
	}

	// 添加规则弹窗：类型切换（横向 ◀▶）+ 字段切换（纵向）+ 确认 + 取消
	if state.ShowAddForm {
		return []common.InlineHelpHint{
			{Key: "←/→", Desc: i18n.T("help.rules_add.type")},
			{Key: "↑/↓", Desc: i18n.T("help.rules_add.field")},
			{Key: "Enter", Desc: i18n.T("help.rules_add.confirm")},
			{Key: "Esc", Desc: i18n.T("help.rules_add.cancel")},
		}
	}

	// 类型筛选弹窗：移动 + 切换 + 确认 + 取消
	if state.ShowTypeFilter {
		return []common.InlineHelpHint{
			{Key: "↑/↓", Desc: i18n.T("help.rules_filter.select")},
			{Key: "Space", Desc: i18n.T("help.rules_filter.toggle")},
			{Key: "Enter", Desc: i18n.T("help.rules_filter.confirm")},
			{Key: "Esc", Desc: i18n.T("help.rules_filter.cancel")},
		}
	}

	// 过滤输入模式：确认 + 取消 + 删除 + 引擎切换
	if state.FilterMode {
		return []common.InlineHelpHint{
			{Key: "Enter", Desc: i18n.T("help.rules_search.confirm")},
			{Key: "Esc", Desc: i18n.T("help.rules_search.cancel")},
			{Key: "⌫", Desc: i18n.T("help.rules_search.backspace")},
			{Key: "Ctrl+R/F", Desc: i18n.T("help.rules_search.regex_fuzzy")},
		}
	}

	// 普通模式：核心操作
	return []common.InlineHelpHint{
		{Key: "↑↓", Desc: i18n.T("help.rules.hint_select")},
		{Key: "/", Desc: i18n.T("help.rules.hint_search")},
		{Key: "t", Desc: i18n.T("help.rules.hint_type")},
		{Key: "n", Desc: i18n.T("help.rules.hint_add")},
		{Key: "r", Desc: i18n.T("help.rules.hint_refresh")},
	}
}

// renderRulesInlineHelp 渲染右下角内联帮助面板并叠加到页面上
func renderRulesInlineHelp(page string, state PageState) string {
	hints := buildRulesInlineHelpHints(state)
	if len(hints) == 0 {
		return page
	}

	body := common.FormatInlineHintRow(hints)
	return common.OverlayHelpAtBottomRight(page, body, state.Width, state.Height)
}

// RenderRulesHeaderComponent 渲染规则页面头部组件（带边框，包含统计和搜索框）
// 统计信息右对齐显示在搜索行右侧
func RenderRulesHeaderComponent(stats string, searchBox string, width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue)
	statsStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue)

	// 计算内边框宽度
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	// 搜索行：左侧搜索框 + 右对齐统计
	statsText := statsStyle.Render(stats)
	searchW := lipgloss.Width(searchBox)
	statsW := lipgloss.Width(statsText)
	gap := innerWidth - searchW - statsW
	if gap < 1 {
		gap = 1
	}
	searchRow := borderStyle.Render("│") +
		searchBox + strings.Repeat(" ", gap) + statsText +
		borderStyle.Render("│")

	// 填充行宽（应对极窄情况）
	rowContentWidth := lipgloss.Width(searchBox) + gap + statsW
	if rowContentWidth < innerWidth {
		searchRow = borderStyle.Render("│") +
			searchBox + strings.Repeat(" ", gap) + statsText +
			strings.Repeat(" ", innerWidth-rowContentWidth) +
			borderStyle.Render("│")
	}

	topLine := borderStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")

	return topLine + "\n" + searchRow + "\n" + bottomLine
}

// renderRuleSearchBox 渲染搜索框
func renderRuleSearchBox(filterText string, filterMode bool, engine FilterEngine, selectedTypes []string) string {
	inputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	if filterMode {
		inputStyle = inputStyle.Background(common.CHighlight)
	}

	label := common.MutedStyle.Render(i18n.T("rules.search"))
	input := inputStyle.Render(filterText)

	if filterMode {
		input += inputStyle.Render("█")
	}

	// 引擎徽标（与 nodes 配色一致）
	var engineIndicator string
	switch engine {
	case FilterEngineRegex:
		engineIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).Render(" [RE]")
	case FilterEngineFuzzy:
		engineIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render(" [Fuzzy]")
	}

	hint := ""
	if filterText == "" {
		switch engine {
		case FilterEngineRegex:
			hint = common.MutedStyle.Render(i18n.T("rules.search_hint_regex"))
		case FilterEngineFuzzy:
			hint = common.MutedStyle.Render(i18n.T("rules.search_hint_fuzzy"))
		default:
			hint = common.MutedStyle.Render(i18n.T("rules.search_hint"))
		}
	}
	if len(selectedTypes) > 0 {
		typeNames := strings.Join(selectedTypes, ", ")
		typeIndicator := lipgloss.NewStyle().
			Foreground(common.CSuccess).
			Render(fmt.Sprintf(" [%s]", typeNames))
		return label + input + hint + engineIndicator + typeIndicator
	}
	return label + input + hint + engineIndicator
}

// renderRuleList 渲染规则列表（含整体垂直滚动条）
func renderRuleList(rules []filteredRule, selectedIdx, scrollTop, maxLines, width int, colorAdjustLight, colorAdjustDark float64) string {
	if len(rules) == 0 {
		return common.MutedStyle.Render(i18n.T("rules.empty"))
	}

	// 检测 Domain 和 DomainSuffix 是否共享相同颜色，如果是则应用颜色区分
	adjustedColors := detectAndAdjustDomainColors(colorAdjustLight, colorAdjustDark)

	// 调整滚动位置确保选中项可见
	if selectedIdx < scrollTop {
		scrollTop = selectedIdx
	}
	if selectedIdx >= scrollTop+maxLines {
		scrollTop = selectedIdx - maxLines + 1
	}

	endIdx := scrollTop + maxLines
	if endIdx > len(rules) {
		endIdx = len(rules)
	}

	// 渲染规则行（预留滚动条宽度）
	listWidth := width - rulesScrollWidth
	var lines []string
	for i := scrollTop; i < endIdx; i++ {
		fr := rules[i]
		line := renderRuleEntry(fr.Rule, fr.Index, i == selectedIdx, listWidth, adjustedColors)
		lines = append(lines, line)
	}
	listStr := strings.Join(lines, "\n")

	// 构建整体垂直滚动条
	scrollbarStr := buildScrollbar(maxLines, len(rules), scrollTop)

	// 计算滚动块的起止行
	thumbStart, thumbEnd := common.CalcThumbRange(maxLines, len(rules), scrollTop)

	var barLines []string
	for i, ch := range strings.Split(scrollbarStr, "\n") {
		if i >= thumbStart && i < thumbEnd {
			barLines = append(barLines, common.MutedStyle.Foreground(lipgloss.Color("#AAAAAA")).Render(ch))
		} else {
			barLines = append(barLines, common.DimStyle.Render(ch))
		}
	}
	barStr := strings.Join(barLines, "\n")

	// 仅在内容超出可视区域时显示滚动条
	if len(rules) <= maxLines {
		return listStr
	}

	// 将列表整体设置为固定宽度，确保每行等宽，避免滚动条因行宽不一致而坍缩
	fixedList := lipgloss.NewStyle().Width(listWidth).Render(listStr)
	return lipgloss.JoinHorizontal(lipgloss.Top, fixedList, barStr)
}

// buildScrollbar 构建高度为 viewHeight 的滚动条字符串（每行一个字符，换行连接）
func buildScrollbar(viewHeight, total, scrollTop int) string {
	lines := make([]string, viewHeight)
	for i := range lines {
		lines[i] = common.SymbolScrollbarTrack
	}
	// 用实心块覆盖滑块区域
	start, end := common.CalcThumbRange(viewHeight, total, scrollTop)
	for i := start; i < end; i++ {
		if i < viewHeight {
			lines[i] = common.SymbolScrollbarThumb
		}
	}
	return strings.Join(lines, "\n")
}

// renderRuleEntry 渲染单条规则
func renderRuleEntry(rule model.Rule, index int, selected bool, width int, adjustedColors map[string]lipgloss.Color) string {
	// 获取规则类型颜色（可能已被调整）
	color := getAdjustedRuleTypeColor(rule.Type, adjustedColors)

	// 序号
	indexStyle := lipgloss.NewStyle().Foreground(common.CSuccess).Width(6)
	indexStr := indexStyle.Render(fmt.Sprintf("%d.", index+1))

	// 类型标签
	typeStyle := lipgloss.NewStyle().Foreground(color).Bold(true).Width(16)
	typeStr := typeStyle.Render(rule.Type)

	// no-resolve 标签 (固定列宽以保持对齐)
	noResolveColWidth := 12
	noResolveStyle := lipgloss.NewStyle().Width(noResolveColWidth)
	var noResolveStr string
	if rule.NoResolve {
		noResolveStr = noResolveStyle.Foreground(common.CWarning).Render("no-resolve")
	} else {
		noResolveStr = noResolveStyle.Render("")
	}

	// Payload (减少宽度以为 no-resolve 列腾出空间)
	payloadWidth := width - 65
	if payloadWidth < 20 {
		payloadWidth = 20
	}
	payloadStyle := lipgloss.NewStyle().Foreground(common.CSecondary)
	payload := rule.Payload
	if len(payload) > payloadWidth {
		payload = payload[:payloadWidth-3] + "..."
	}
	payloadStr := payloadStyle.Width(payloadWidth).Render(payload)

	// 代理
	proxyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	proxyStr := proxyStyle.Render(rule.Proxy)

	// 构建行 (顺序: 序号 类型 Payload no-resolve 代理)
	line := fmt.Sprintf("%s %s %s %s %s", indexStr, typeStr, payloadStr, noResolveStr, proxyStr)

	// 选中样式
	if selected {
		line = lipgloss.NewStyle().
			Background(common.CHighlight).
			Render(common.SymbolSelectActive + line)
	} else {
		line = common.SymbolSelectInactive + line
	}

	return line
}

// adjustedRuleColors 存储调整后的规则类型颜色缓存
var adjustedRuleColors = make(map[string]lipgloss.Color)

// prevColorAdjustLight 上次的轻度调整值（用于检测变化）
var prevColorAdjustLight float64 = -1

// prevColorAdjustDark 上次的深度调整值（用于检测变化）
var prevColorAdjustDark float64 = -1

// detectAndAdjustDomainColors 检测 Domain 和 DomainSuffix 是否颜色相同，如果是则调整它们
func detectAndAdjustDomainColors(colorAdjustLight, colorAdjustDark float64) map[string]lipgloss.Color {
	// 如果调整参数未设置，使用默认值
	if colorAdjustLight <= 0 {
		colorAdjustLight = 0.25
	}
	if colorAdjustDark <= 0 {
		colorAdjustDark = 0.20
	}

	// 如果参数未变化且已有缓存，直接返回缓存
	if colorAdjustLight == prevColorAdjustLight && colorAdjustDark == prevColorAdjustDark && len(adjustedRuleColors) > 0 {
		return adjustedRuleColors
	}

	// 重置缓存
	adjustedRuleColors = make(map[string]lipgloss.Color)

	// 检查 Domain 和 DomainSuffix 的基础颜色是否相同
	domainBaseColor := ruleTypeColors[domainColorKey]
	domainSuffixBaseColor := ruleTypeColors[domainSuffixColorKey]

	if domainBaseColor == "" || domainSuffixBaseColor == "" {
		return adjustedRuleColors
	}

	baseColorHex := string(domainBaseColor)
	baseColorSuffixHex := string(domainSuffixBaseColor)

	// 如果颜色相同，进行调整
	if utils.ColorStringsEqual(baseColorHex, baseColorSuffixHex) {
		// 生成较浅和较深的变体
		lighterHex, err := utils.LighterColor(baseColorHex, colorAdjustLight)
		if err == nil {
			adjustedRuleColors[domainColorKey] = lipgloss.Color(lighterHex)
		}

		darkerHex, err := utils.DarkerColor(baseColorSuffixHex, colorAdjustDark)
		if err == nil {
			adjustedRuleColors[domainSuffixColorKey] = lipgloss.Color(darkerHex)
		}

		// 同时处理大写格式
		adjustedRuleColors["DOMAIN"] = adjustedRuleColors[domainColorKey]
		adjustedRuleColors["DOMAIN-SUFFIX"] = adjustedRuleColors[domainSuffixColorKey]
	}

	prevColorAdjustLight = colorAdjustLight
	prevColorAdjustDark = colorAdjustDark

	return adjustedRuleColors
}

// getAdjustedRuleTypeColor 获取调整后的规则类型颜色
func getAdjustedRuleTypeColor(ruleType string, adjustedColors map[string]lipgloss.Color) lipgloss.Color {
	if adjustedColor, ok := adjustedColors[ruleType]; ok {
		return adjustedColor
	}
	if baseColor, ok := ruleTypeColors[ruleType]; ok {
		return baseColor
	}
	return lipgloss.Color("#CCCCCC")
}

// animateColor 计算平滑过渡动画后的颜色
func animateColor(from, to lipgloss.Color, progress float64) lipgloss.Color {
	fromStr := string(from)
	toStr := string(to)

	if fromStr == toStr {
		return from
	}

	fromHex := strings.TrimPrefix(fromStr, "#")
	toHex := strings.TrimPrefix(toStr, "#")

	if len(fromHex) != 6 || len(toHex) != 6 {
		return to
	}

	fromR := hexToInt(fromHex[0:2])
	fromG := hexToInt(fromHex[2:4])
	fromB := hexToInt(fromHex[4:6])

	toR := hexToInt(toHex[0:2])
	toG := hexToInt(toHex[2:4])
	toB := hexToInt(toHex[4:6])

	newR := int(float64(fromR) + float64(toR-fromR)*progress)
	newG := int(float64(fromG) + float64(toG-fromG)*progress)
	newB := int(float64(fromB) + float64(toB-fromB)*progress)

	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", newR, newG, newB))
}

// hexToInt 将十六进制字符串转换为整数
func hexToInt(s string) int {
	var val int
	for _, c := range s {
		val *= 16
		switch {
		case c >= '0' && c <= '9':
			val += int(c - '0')
		case c >= 'A' && c <= 'F':
			val += int(c - 'A' + 10)
		case c >= 'a' && c <= 'f':
			val += int(c - 'a' + 10)
		}
	}
	return val
}

// interpolateColor 在两个颜色之间进行线性插值
func interpolateColor(color1, color2 string, t float64) string {
	c1 := strings.TrimPrefix(color1, "#")
	c2 := strings.TrimPrefix(color2, "#")

	if len(c1) != 6 || len(c2) != 6 {
		return color2
	}

	r1 := hexToInt(c1[0:2])
	g1 := hexToInt(c1[2:4])
	b1 := hexToInt(c1[4:6])

	r2 := hexToInt(c2[0:2])
	g2 := hexToInt(c2[2:4])
	b2 := hexToInt(c2[4:6])

	r := int(float64(r1) + float64(r2-r1)*t)
	g := int(float64(g1) + float64(g2-g1)*t)
	b := int(float64(b1) + float64(b2-b1)*t)

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// renderTypeFilterOverlay 渲染类型筛选弹窗叠加层。
//
// 步骤：
//  1. 对背景每行整体套 Faint，让底层内容降亮（半透明效果）
//  2. 渲染弹窗本体 buildTypeFilterModal，并计算居中偏移
//  3. 逐行用 ansi.Cut 截取底层的左右两侧，将弹窗内容嵌入中间
func renderTypeFilterOverlay(background string, state PageState, width, height int) string {
	// ── 1. 暗化底层 ──
	baseLines := strings.Split(background, "\n")
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
	modal := buildTypeFilterModal(state, width, height)
	modalLines := strings.Split(modal, "\n")
	modalHeight := len(modalLines)
	if modalHeight == 0 {
		return strings.Join(dimmed, "\n")
	}

	modalWidth := lipgloss.Width(modalLines[0])
	leftOffset := (width - modalWidth) / 2
	if leftOffset < 0 {
		leftOffset = 0
	}
	topOffset := (height - modalHeight) / 2
	if topOffset < 0 {
		topOffset = 0
	}

	// ── 3. 弹窗行嵌入暗化底层 ──
	for i, pl := range modalLines {
		y := topOffset + i
		if y >= height {
			break
		}

		leftPart := ansi.Cut(dimmed[y], 0, leftOffset)
		leftW := lipgloss.Width(leftPart)
		if leftW < leftOffset {
			leftPart += strings.Repeat(" ", leftOffset-leftW)
		}

		rightPart := ansi.Cut(dimmed[y], leftOffset+modalWidth, width)
		dimmed[y] = leftPart + pl + rightPart
	}

	return strings.Join(dimmed, "\n")
}

// typeFilterModalLayout 描述弹窗的内部布局，供渲染与鼠标命中复用
type typeFilterModalLayout struct {
	modalWidth   int // 弹窗整体宽度（含边框）
	modalHeight  int // 弹窗整体高度（含边框）
	listStartY   int // 列表第一行相对弹窗顶部的偏移
	listHeight   int // 列表可见行数
	visibleStart int // 列表可见起始索引（含滚动）
}

// computeTypeFilterModalLayout 计算弹窗尺寸与列表布局。
// 该尺寸计算必须与 buildTypeFilterModal 保持完全一致，否则鼠标命中会错位。
func computeTypeFilterModalLayout(state PageState, width, height int) typeFilterModalLayout {
	// 弹窗宽度（与 buildTypeFilterModal 保持一致）
	modalWidth := 50
	if modalWidth > width-4 {
		modalWidth = width - 4
	}

	total := len(state.AvailableTypes)

	// 弹窗高度根据实际类型数量动态收缩，避免列表项少时底部留白过大。
	// 总高 = 上边框1 + 分隔空行1 + listHeight + 分隔空行1 + 统计1 + 下边框1 = listHeight + 5
	// listHeight 取「实际类型数」与「上限」的较小值。
	maxListHeight := 14 // 单屏最多展示的列表项数，超出则滚动
	listHeight := total
	if listHeight > maxListHeight {
		listHeight = maxListHeight
	}
	if listHeight < 3 {
		listHeight = 3
	}
	modalHeight := listHeight + 5
	// 不超过可用屏幕高度的 2/3，保留呼吸空间
	if modalHeight > height-6 {
		modalHeight = height - 6
		listHeight = modalHeight - 5
		if listHeight < 3 {
			listHeight = 3
			modalHeight = 8
		}
	}

	// 列表在弹窗内的起始行（上边框1 + 分隔空行1）
	listStartY := 2

	// 滚动偏移
	visibleStart := 0
	if state.TypeFilterCursor >= listHeight {
		visibleStart = state.TypeFilterCursor - listHeight + 1
	}
	if visibleStart > total-listHeight && total > listHeight {
		visibleStart = total - listHeight
	}
	if visibleStart < 0 {
		visibleStart = 0
	}

	return typeFilterModalLayout{
		modalWidth:   modalWidth,
		modalHeight:  modalHeight,
		listStartY:   listStartY,
		listHeight:   listHeight,
		visibleStart: visibleStart,
	}
}

// ResolveTypeFilterModalBounds 返回类型筛选弹窗在页面坐标系中的边界（右下为开区间）。
// 通过渲染真实弹窗取尺寸，确保与 renderTypeFilterOverlay 的居中位置完全一致。
func ResolveTypeFilterModalBounds(state PageState, width, height int) (left, top, right, bottom int) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0
	}
	modal := buildTypeFilterModal(state, width, height)
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)

	leftGap := width - modalWidth
	if leftGap < 0 {
		leftGap = 0
	}
	topGap := height - modalHeight
	if topGap < 0 {
		topGap = 0
	}
	left = leftGap / 2
	top = topGap / 2
	right = left + modalWidth
	bottom = top + modalHeight
	return left, top, right, bottom
}

// ResolveTypeFilterListItemAt 判断页面坐标 (pageX, pageY) 命中的类型列表项索引。
// 若不在列表行上则返回 -1。bounds 与 layout 用于复用渲染时的计算。
func ResolveTypeFilterListItemAt(state PageState, pageX, pageY, width, height int) int {
	left, top, right, bottom := ResolveTypeFilterModalBounds(state, width, height)
	if pageX < left || pageX >= right || pageY < top || pageY >= bottom {
		return -1
	}
	layout := computeTypeFilterModalLayout(state, width, height)

	// 列表在弹窗内的起始行（含 padding）
	listTopRow := top + layout.listStartY
	listBottomRow := listTopRow + layout.listHeight
	if pageY < listTopRow || pageY >= listBottomRow {
		return -1
	}

	rowOffset := pageY - listTopRow
	idx := layout.visibleStart + rowOffset
	if idx < 0 || idx >= len(state.AvailableTypes) {
		return -1
	}
	return idx
}

// buildTypeFilterModal 渲染类型筛选弹窗本体（无暗化背景）。
// 标题嵌入顶部边框左上角（╭─ title ───╮），尺寸计算与 computeTypeFilterModalLayout 必须一致。
func buildTypeFilterModal(state PageState, width, height int) string {
	layout := computeTypeFilterModalLayout(state, width, height)
	modalWidth := layout.modalWidth
	listHeight := layout.listHeight

	// 渲染类型列表
	total := len(state.AvailableTypes)
	visibleStart := layout.visibleStart
	visibleEnd := visibleStart + listHeight
	if visibleEnd > total {
		visibleEnd = total
	}

	var typeLines []string
	for i := visibleStart; i < visibleEnd; i++ {
		typeName := state.AvailableTypes[i]
		isSelected := false
		for _, st := range state.SelectedTypes {
			if st == typeName {
				isSelected = true
				break
			}
		}

		// 获取类型颜色
		color := getAdjustedRuleTypeColor(typeName, nil)

		// 构建行内容
		var line string
		if i == state.TypeFilterCursor {
			// 当前光标行 — Tokyo Night 选中态
			cursorStyle := lipgloss.NewStyle().
				Background(common.TokyoSelected).
				Foreground(common.TokyoCyan)
			var checkMark string
			if isSelected {
				checkMark = lipgloss.NewStyle().Foreground(common.TokyoGreen).Render("✓ ")
			} else {
				checkMark = "  "
			}
			typeStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
			line = cursorStyle.Render(" " + checkMark + typeStyle.Render(typeName) + " ")
		} else {
			var checkMark string
			if isSelected {
				checkMark = lipgloss.NewStyle().Foreground(common.TokyoGreen).Render("✓ ")
			} else {
				checkMark = "  "
			}
			typeStyle := lipgloss.NewStyle().Foreground(color)
			line = " " + checkMark + typeStyle.Render(typeName)
		}
		typeLines = append(typeLines, line)
	}

	// 填充空行
	for len(typeLines) < listHeight {
		typeLines = append(typeLines, "")
	}

	typeList := strings.Join(typeLines, "\n")

	// 统计信息 — Tokyo Night
	statsText := i18n.Tf("rules.filter_stats", len(state.SelectedTypes), len(state.AvailableTypes))
	stats := common.TokyoMutedStyle().Render(statsText)

	// 组装内容区（标题已嵌入边框；列表上方留一空行与标题边框分隔）
	modalContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		typeList,
		"",
		stats,
	)

	// 标题嵌入顶部边框左上角，padding(1,2) 对齐 RenderBorderedPanel 的左右各1空格
	return common.RenderBorderedPanel(
		i18n.T("rules.filter_title"),
		modalContent,
		modalWidth,
		common.TokyoBlue,
		common.TokyoForeground,
	)
}

// ============================================================
//  添加自定义规则弹窗（renderAddRuleOverlay / buildAddRuleModal）
// ============================================================
//
// 布局完全复刻类型筛选弹窗：暗化底层 → 居中弹窗 → 逐行嵌入。
// 表单字段使用 bubbles/textinput 的 View() 渲染（自带光标）。

// addFormModalWidth 弹窗整体宽度（含边框）。
const addFormModalWidth = 54

// addFormLayout 描述添加规则弹窗的内部布局，供渲染与鼠标命中复用。
type addFormLayout struct {
	modalWidth  int // 弹窗整体宽度（含边框）
	modalHeight int // 弹窗整体高度（含边框）
}

// computeAddFormLayout 计算弹窗尺寸，需与 buildAddRuleModal 保持一致。
func computeAddFormLayout(state PageState, width, height int) addFormLayout {
	modalWidth := addFormModalWidth
	if modalWidth > width-4 {
		modalWidth = width - 4
	}
	if modalWidth < 24 {
		modalWidth = 24
	}
	// 基础高度：上边框1 + 类型行1 + 分隔1 + 三字段3 + 分隔1 + 说明/错误1 + 下边框1 = 9
	// IP 类规则额外显示 no-resolve 复选框行 +1
	modalHeight := 9
	if state.AddForm.isIPType() {
		modalHeight = 10
	}
	if modalHeight > height-2 {
		modalHeight = height - 2
	}
	return addFormLayout{modalWidth: modalWidth, modalHeight: modalHeight}
}

// renderAddRuleOverlay 渲染添加规则弹窗叠加层（同 renderTypeFilterOverlay 结构）。
func renderAddRuleOverlay(background string, state PageState, width, height int) string {
	// ── 1. 暗化底层 ──
	baseLines := strings.Split(background, "\n")
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
	modal := buildAddRuleModal(state, width, height)
	modalLines := strings.Split(modal, "\n")
	modalHeight := len(modalLines)
	if modalHeight == 0 {
		return strings.Join(dimmed, "\n")
	}

	modalWidth := lipgloss.Width(modalLines[0])
	leftOffset := (width - modalWidth) / 2
	if leftOffset < 0 {
		leftOffset = 0
	}
	topOffset := (height - modalHeight) / 2
	if topOffset < 0 {
		topOffset = 0
	}

	// ── 3. 弹窗行嵌入暗化底层 ──
	for i, pl := range modalLines {
		y := topOffset + i
		if y >= height {
			break
		}

		leftPart := ansi.Cut(dimmed[y], 0, leftOffset)
		leftW := lipgloss.Width(leftPart)
		if leftW < leftOffset {
			leftPart += strings.Repeat(" ", leftOffset-leftW)
		}

		rightPart := ansi.Cut(dimmed[y], leftOffset+modalWidth, width)
		dimmed[y] = leftPart + pl + rightPart
	}

	return strings.Join(dimmed, "\n")
}

// buildAddRuleModal 渲染添加规则弹窗本体（无暗化背景）。
// 尺寸计算与 computeAddFormLayout 必须一致。
func buildAddRuleModal(state PageState, width, height int) string {
	layout := computeAddFormLayout(state, width, height)
	innerWidth := layout.modalWidth - 4 // 减去左右边框 + padding
	if innerWidth < 10 {
		innerWidth = 10
	}

	form := state.AddForm

	// ── 类型行：◀ TYPE ▶ ──
	typeColor := getAdjustedRuleTypeColor(form.currentType(), nil)
	typeValStyle := lipgloss.NewStyle().Foreground(typeColor).Bold(true)
	typeLabel := common.TokyoMutedStyle().Render(i18n.T("rules.add_field_type"))
	typeVal := typeValStyle.Render(fmt.Sprintf("◀ %s ▶", form.currentType()))
	typeRow := typeLabel + " " + typeVal

	// ── 分隔行 ──
	divider := lipgloss.NewStyle().Foreground(common.TokyoBlue).Render(strings.Repeat("─", innerWidth))

	// ── 字段渲染 ──
	// MATCH 无 payload，省略匹配值字段
	payloadRow := renderAddFormFieldRow(form, addFieldPayload, i18n.T("rules.add_field_payload"), innerWidth, form.isMatchType())
	proxyRow := renderAddFormProxyRow(form)
	indexRow := renderAddFormFieldRow(form, addFieldIndex, i18n.T("rules.add_field_index"), innerWidth, false)

	// ── 第二分隔行 ──
	divider2 := divider

	// ── no-resolve 复选框行（仅 IP 类规则显示）──
	var noResolveRow string
	if form.isIPType() {
		noResolveRow = renderAddFormNoResolveRow(form)
	}

	// ── 说明/错误行 ──
	var footer string
	if form.errMsg != "" {
		footer = lipgloss.NewStyle().Foreground(common.CDanger).Render("✗ " + form.errMsg)
	} else if form.isProxyField() {
		// 策略行聚焦时提示 ←/→ 切换、来源为源配置文件
		footer = common.TokyoMutedStyle().Render(i18n.T("rules.add_proxy_hint"))
	} else if form.isNoResolveField() {
		footer = common.TokyoMutedStyle().Render(i18n.T("rules.add_noresolve_hint"))
	} else {
		footer = common.TokyoMutedStyle().Render(i18n.T("rules.add_index_hint"))
	}

	var contentRows []string
	contentRows = append(contentRows, typeRow, divider)
	if payloadRow != "" {
		contentRows = append(contentRows, payloadRow)
	}
	contentRows = append(contentRows, proxyRow, indexRow)
	if noResolveRow != "" {
		contentRows = append(contentRows, noResolveRow)
	}
	contentRows = append(contentRows, divider2, footer)

	modalContent := strings.Join(contentRows, "\n")

	return common.RenderBorderedPanel(
		i18n.T("rules.add_title"),
		modalContent,
		layout.modalWidth,
		common.TokyoBlue,
		common.TokyoForeground,
	)
}

// renderAddFormFieldRow 渲染单个表单字段行（label + textinput.View）。
// skip=true 时返回空字符串（用于 MATCH 类型省略 payload 字段）。
func renderAddFormFieldRow(form addForm, fieldIdx int, label string, innerWidth int, skip bool) string {
	if skip {
		return ""
	}
	labelStyle := common.TokyoMutedStyle()
	labelText := labelStyle.Render(label)

	// textinput 的 View() 自带光标；为聚焦字段添加高亮背景
	field := form.fields[fieldIdx]
	inputView := field.View()
	if form.fieldCursor == fieldIdx {
		// 聚焦态：青色高亮
		inputView = lipgloss.NewStyle().Foreground(common.TokyoCyan).Render(inputView)
	} else {
		inputView = lipgloss.NewStyle().Foreground(common.TokyoForeground).Render(inputView)
	}

	return labelText + " " + inputView
}

// renderAddFormProxyRow 渲染策略选择器行：策略: NAME (Enter 改)。
// 策略来源颜色区分：策略组（TokyoBlue）/ 具体节点（TokyoGreen）。
func renderAddFormProxyRow(form addForm) string {
	labelStyle := common.TokyoMutedStyle()
	labelText := labelStyle.Render(i18n.T("rules.add_field_proxy"))

	// 策略值颜色：策略组用蓝，节点用绿
	valColor := common.TokyoBlue
	if !form.isProxyGroupSelected() {
		valColor = common.TokyoGreen
	}
	val := form.currentProxy()
	var valView string
	if form.isProxyField() {
		valView = lipgloss.NewStyle().Foreground(valColor).Bold(true).Render(val)
		valView += " " + common.TokyoMutedStyle().Render(i18n.T("rules.add_proxy_change"))
	} else {
		valView = lipgloss.NewStyle().Foreground(common.TokyoForeground).Render(val)
	}
	return labelText + " " + valView
}

// renderAddFormNoResolveRow 渲染 no-resolve 复选框行。
// 仅当规则类型为 IP-CIDR/IP-CIDR6/SRC-IP-CIDR 时显示。
func renderAddFormNoResolveRow(form addForm) string {
	labelStyle := common.TokyoMutedStyle()
	labelText := labelStyle.Render(i18n.T("rules.add_field_noresolve"))

	checkbox := "[ ]"
	if form.noResolve {
		checkbox = "[✓]"
	}

	var valView string
	if form.isNoResolveField() {
		valView = lipgloss.NewStyle().Foreground(common.TokyoCyan).Bold(true).Render(checkbox)
	} else {
		valView = lipgloss.NewStyle().Foreground(common.TokyoForeground).Render(checkbox)
	}
	return labelText + " " + valView
}

// ResolveAddFormBounds 返回添加规则弹窗在页面坐标系中的边界（右下为开区间）。
// 通过渲染真实弹窗取尺寸，确保与 renderAddRuleOverlay 居中位置完全一致。
func ResolveAddFormBounds(state PageState, width, height int) (left, top, right, bottom int) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0
	}
	modal := buildAddRuleModal(state, width, height)
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)

	leftGap := width - modalWidth
	if leftGap < 0 {
		leftGap = 0
	}
	topGap := height - modalHeight
	if topGap < 0 {
		topGap = 0
	}
	left = leftGap / 2
	top = topGap / 2
	right = left + modalWidth
	bottom = top + modalHeight
	return left, top, right, bottom
}

// ============================================================
//  策略选择二级弹窗（renderProxyPickerOverlay / buildProxyPickerModal）
// ============================================================
//
// 镜像类型筛选弹窗：暗化底层 → 居中弹窗 → 逐行嵌入。
// 内部布局：Tab 行 + 搜索框 + 可滚动列表 + 统计栏。

// proxyPickerModalWidth 弹窗整体宽度（含边框）。
const proxyPickerModalWidth = 52

// proxyPickerLayout 描述策略弹窗的内部布局，供渲染与鼠标命中复用。
type proxyPickerLayout struct {
	modalWidth   int // 弹窗整体宽度（含边框）
	modalHeight  int // 弹窗整体高度（含边框）
	listStartY   int // 列表第一行相对弹窗顶部的偏移
	listHeight   int // 列表可见行数
	visibleStart int // 列表可见起始索引（含滚动）
}

// computeProxyPickerLayout 计算弹窗尺寸与列表布局。
// 必须与 buildProxyPickerModal 保持完全一致。
func computeProxyPickerLayout(state PageState, width, height int) proxyPickerLayout {
	modalWidth := proxyPickerModalWidth
	if modalWidth > width-4 {
		modalWidth = width - 4
	}
	if modalWidth < 24 {
		modalWidth = 24
	}

	form := state.AddForm
	filtered := form.pickerFiltered()
	total := len(filtered)

	// 总高 = 上边框1 + Tab行1 + 搜索行1 + 列表listHeight + 统计行1 + 下边框1 = listHeight + 5
	listHeight := total
	if listHeight > addPickerListMax {
		listHeight = addPickerListMax
	}
	if listHeight < 3 {
		listHeight = 3
	}
	modalHeight := listHeight + 5
	if modalHeight > height-4 {
		modalHeight = height - 4
		listHeight = modalHeight - 5
		if listHeight < 3 {
			listHeight = 3
			modalHeight = 8
		}
	}

	// 列表在弹窗内的起始行（上边框1 + Tab行1 + 搜索行1 = 3）
	listStartY := 3

	// 滚动偏移
	visibleStart := form.pickerScrollTop
	if visibleStart < 0 {
		visibleStart = 0
	}
	if visibleStart > total-listHeight && total > listHeight {
		visibleStart = total - listHeight
	}
	if visibleStart < 0 {
		visibleStart = 0
	}

	return proxyPickerLayout{
		modalWidth:   modalWidth,
		modalHeight:  modalHeight,
		listStartY:   listStartY,
		listHeight:   listHeight,
		visibleStart: visibleStart,
	}
}

// renderProxyPickerOverlay 渲染策略选择弹窗叠加层（复用暗化+居中+嵌入三步）。
func renderProxyPickerOverlay(background string, state PageState, width, height int) string {
	baseLines := strings.Split(background, "\n")
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

	modal := buildProxyPickerModal(state, width, height)
	modalLines := strings.Split(modal, "\n")
	modalHeight := len(modalLines)
	if modalHeight == 0 {
		return strings.Join(dimmed, "\n")
	}

	modalWidth := lipgloss.Width(modalLines[0])
	leftOffset := (width - modalWidth) / 2
	if leftOffset < 0 {
		leftOffset = 0
	}
	topOffset := (height - modalHeight) / 2
	if topOffset < 0 {
		topOffset = 0
	}

	for i, pl := range modalLines {
		y := topOffset + i
		if y >= height {
			break
		}
		leftPart := ansi.Cut(dimmed[y], 0, leftOffset)
		leftW := lipgloss.Width(leftPart)
		if leftW < leftOffset {
			leftPart += strings.Repeat(" ", leftOffset-leftW)
		}
		rightPart := ansi.Cut(dimmed[y], leftOffset+modalWidth, width)
		dimmed[y] = leftPart + pl + rightPart
	}

	return strings.Join(dimmed, "\n")
}

// buildProxyPickerModal 渲染策略选择弹窗本体（无暗化背景）。
func buildProxyPickerModal(state PageState, width, height int) string {
	layout := computeProxyPickerLayout(state, width, height)
	form := state.AddForm

	// ── Tab 行 ──
	tabGroup := i18n.T("rules.picker_tab_group")
	tabNode := i18n.T("rules.picker_tab_node")
	activeStyle := lipgloss.NewStyle().Foreground(common.TokyoCyan).Bold(true).Underline(true)
	inactiveStyle := common.TokyoMutedStyle()
	var tabRow string
	if form.pickerTab == addPickerTabGroup {
		tabRow = activeStyle.Render("▸"+tabGroup) + "  " + inactiveStyle.Render(tabNode)
	} else {
		tabRow = inactiveStyle.Render(tabGroup) + "  " + activeStyle.Render("▸"+tabNode)
	}

	// ── 搜索行 ──
	searchLabel := common.TokyoMutedStyle().Render(i18n.T("rules.picker_search"))
	searchVal := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(form.pickerSearch + "█")
	searchRow := searchLabel + searchVal

	// ── 列表 ──
	filtered := form.pickerFiltered()
	total := len(filtered)
	visibleStart := layout.visibleStart
	visibleEnd := visibleStart + layout.listHeight
	if visibleEnd > total {
		visibleEnd = total
	}

	var listLines []string
	for i := visibleStart; i < visibleEnd; i++ {
		name := filtered[i]
		isCurrentSelected := name == form.proxySelected
		isGroup := false
		for _, g := range form.proxyGroups {
			if g == name {
				isGroup = true
				break
			}
		}
		nameColor := common.TokyoBlue
		if !isGroup {
			nameColor = common.TokyoGreen
		}

		if i == form.pickerCursor {
			cursorStyle := lipgloss.NewStyle().Background(common.TokyoSelected).Foreground(common.TokyoCyan)
			var mark string
			if isCurrentSelected {
				mark = lipgloss.NewStyle().Foreground(common.TokyoGreen).Render("✓ ")
			} else {
				mark = "  "
			}
			nameStyle := lipgloss.NewStyle().Foreground(nameColor).Bold(true)
			listLines = append(listLines, cursorStyle.Render(" "+mark+nameStyle.Render(name)+" "))
		} else {
			var mark string
			if isCurrentSelected {
				mark = lipgloss.NewStyle().Foreground(common.TokyoGreen).Render("✓ ")
			} else {
				mark = "  "
			}
			nameStyle := lipgloss.NewStyle().Foreground(nameColor)
			listLines = append(listLines, " "+mark+nameStyle.Render(name))
		}
	}

	// 填充空行
	for len(listLines) < layout.listHeight {
		listLines = append(listLines, "")
	}
	listStr := strings.Join(listLines, "\n")

	// ── 统计栏 ──
	var statsText string
	if total == 0 {
		statsText = common.TokyoMutedStyle().Render(i18n.T("rules.picker_empty"))
	} else {
		statsText = common.TokyoMutedStyle().Render(
			i18n.Tf("rules.picker_stats", form.proxySelected, total, len(form.pickerCandidates())),
		)
	}

	// ── 组装 ──
	modalContent := lipgloss.JoinVertical(lipgloss.Left,
		tabRow,
		searchRow,
		listStr,
		statsText,
	)

	return common.RenderBorderedPanel(
		i18n.T("rules.picker_title"),
		modalContent,
		layout.modalWidth,
		common.TokyoBlue,
		common.TokyoForeground,
	)
}

// ResolveProxyPickerBounds 返回策略选择弹窗在页面坐标系中的边界（右下为开区间）。
func ResolveProxyPickerBounds(state PageState, width, height int) (left, top, right, bottom int) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0
	}
	modal := buildProxyPickerModal(state, width, height)
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)

	leftGap := width - modalWidth
	if leftGap < 0 {
		leftGap = 0
	}
	topGap := height - modalHeight
	if topGap < 0 {
		topGap = 0
	}
	left = leftGap / 2
	top = topGap / 2
	right = left + modalWidth
	bottom = top + modalHeight
	return left, top, right, bottom
}

// ResolveProxyPickerListItemAt 判断页面坐标 (pageX, pageY) 命中的策略列表项索引。
// 若不在列表行上则返回 -1。
func ResolveProxyPickerListItemAt(state PageState, pageX, pageY, width, height int) int {
	left, top, right, bottom := ResolveProxyPickerBounds(state, width, height)
	if pageX < left || pageX >= right || pageY < top || pageY >= bottom {
		return -1
	}
	layout := computeProxyPickerLayout(state, width, height)

	listTopRow := top + layout.listStartY
	listBottomRow := listTopRow + layout.listHeight
	if pageY < listTopRow || pageY >= listBottomRow {
		return -1
	}

	rowOffset := pageY - listTopRow
	idx := layout.visibleStart + rowOffset
	filtered := state.AddForm.pickerFiltered()
	if idx < 0 || idx >= len(filtered) {
		return -1
	}
	return idx
}
