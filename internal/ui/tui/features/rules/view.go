package rules

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

const (
	rulesFixedLines    = 6
	rulesMinHeight     = 5
	rulesScrollWidth   = 2
	colorAnimationMs   = 250
)

var (
	domainColorKey         = "Domain"
	domainSuffixColorKey  = "DomainSuffix"
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
	SelectedRule        int          // 选中的规则索引
	ScrollTop           int          // 滚动偏移
	Width               int          // 页面宽度
	Height              int          // 页面高度
	ColorAdjustLight    float64      // 较浅色调明度增加比例
	ColorAdjustDark     float64      // 较深色调明度降低比例

	// 类型筛选弹窗状态
	ShowTypeFilter   bool     // 是否显示类型筛选弹窗
	TypeFilterSearch string   // 类型搜索文本
	SelectedTypes    []string // 已选择的规则类型
	AvailableTypes   []string // 可用规则类型列表
	TypeFilterCursor int      // 光标位置
}

// RenderRulesPage 渲染规则页面
func RenderRulesPage(state PageState) string {
	var sections []string

	// 渲染搜索框
	searchBox := renderRuleSearchBox(state.FilterText, state.FilterMode, state.SelectedTypes)
	sections = append(sections, searchBox)
	sections = append(sections, "")

	// 过滤规则 (使用缓存的索引)
	var filteredRules []filteredRule
	for _, idx := range state.FilteredRuleIndices {
		if idx >= 0 && idx < len(state.Rules) {
			filteredRules = append(filteredRules, filteredRule{Index: idx, Rule: state.Rules[idx]})
		}
	}

	// 渲染统计信息
	stats := fmt.Sprintf("共 %d 条规则", len(filteredRules))
	if state.FilterText != "" || len(state.SelectedTypes) > 0 {
		stats += fmt.Sprintf(" (过滤自 %d 条)", len(state.Rules))
	}
	sections = append(sections, common.MutedStyle.Render(stats))
	sections = append(sections, "")

	// 计算可显示的规则行数 (搜索框 + 统计 + 间隔)
	availableHeight := state.Height - rulesFixedLines
	if availableHeight < rulesMinHeight {
		availableHeight = rulesMinHeight
	}

	// 渲染规则列表（传入颜色调整参数）
	ruleList := renderRuleList(filteredRules, state.SelectedRule, state.ScrollTop, availableHeight, state.Width, state.ColorAdjustLight, state.ColorAdjustDark)
	sections = append(sections, ruleList)

	// 统一底部的提示信息
	helpText := "[↑/↓]选择 [/]搜索 [t]类型筛选 [Esc]清除 [r]刷新"
	mainContent := strings.Join(sections, "\n")
	contentLines := strings.Count(mainContent, "\n") + 1

	footer := common.RenderFooter(state.Width, state.Height, contentLines, helpText)
	result := mainContent + footer

	// 如果显示类型筛选弹窗，叠加在页面之上
	if state.ShowTypeFilter {
		return renderTypeFilterOverlay(result, state, state.Width, state.Height)
	}

	return result
}

// renderRuleSearchBox 渲染搜索框
func renderRuleSearchBox(filterText string, filterMode bool, selectedTypes []string) string {
	inputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	if filterMode {
		inputStyle = inputStyle.Background(common.CHighlight)
	}

	label := common.MutedStyle.Render("搜索: ")
	input := inputStyle.Render(filterText)

	if filterMode {
		input += inputStyle.Render("█")
	}

	hint := common.MutedStyle.Render(" (多个关键词用空格分隔)")
	if len(selectedTypes) > 0 {
		typeNames := strings.Join(selectedTypes, ", ")
		typeIndicator := lipgloss.NewStyle().
			Foreground(common.CSuccess).
			Render(fmt.Sprintf(" [%s]", typeNames))
		return label + input + hint + typeIndicator
	}
	return label + input + hint
}

// renderRuleList 渲染规则列表（含整体垂直滚动条）
func renderRuleList(rules []filteredRule, selectedIdx, scrollTop, maxLines, width int, colorAdjustLight, colorAdjustDark float64) string {
	if len(rules) == 0 {
		return common.MutedStyle.Render("暂无规则")
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
	thumbStart, thumbEnd := calcThumbRange(maxLines, len(rules), scrollTop)

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

	return lipgloss.JoinHorizontal(lipgloss.Top, listStr, " "+barStr)
}

// buildScrollbar 构建高度为 viewHeight 的滚动条字符串（每行一个字符，换行连接）
func buildScrollbar(viewHeight, total, scrollTop int) string {
	lines := make([]string, viewHeight)
	for i := range lines {
		lines[i] = common.SymbolScrollbarTrack
	}
	// 用实心块覆盖滑块区域
	start, end := calcThumbRange(viewHeight, total, scrollTop)
	for i := start; i < end; i++ {
		if i < viewHeight {
			lines[i] = common.SymbolScrollbarThumb
		}
	}
	return strings.Join(lines, "\n")
}

// calcThumbRange 计算滑块在滚动条中的起止行（左闭右开）
func calcThumbRange(viewHeight, total, scrollTop int) (start, end int) {
	if total <= 0 {
		return 0, viewHeight
	}
	thumbSize := float64(viewHeight) * float64(viewHeight) / float64(total)
	if thumbSize < 1 {
		thumbSize = 1
	}
	thumbStart := float64(scrollTop) * float64(viewHeight) / float64(total)
	start = int(thumbStart)
	end = start + int(thumbSize+0.5)
	if end > viewHeight {
		end = viewHeight
	}
	if start >= end {
		end = start + 1
	}
	return
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

	// Payload
	payloadStyle := lipgloss.NewStyle().Foreground(common.CSecondary)
	payloadWidth := width - 50
	if payloadWidth < 20 {
		payloadWidth = 20
	}
	payload := rule.Payload
	if len(payload) > payloadWidth {
		payload = payload[:payloadWidth-3] + "..."
	}
	payloadStr := payloadStyle.Width(payloadWidth).Render(payload)

	// 代理
	proxyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	proxyStr := proxyStyle.Render(rule.Proxy)

	// 构建行
	line := fmt.Sprintf("%s %s %s %s", indexStr, typeStr, payloadStr, proxyStr)

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

// renderTypeFilterOverlay 渲染类型筛选弹窗叠加层
func renderTypeFilterOverlay(background string, state PageState, width, height int) string {
	// 根据搜索文本过滤可用类型
	var filteredTypes []string
	if state.TypeFilterSearch == "" {
		filteredTypes = state.AvailableTypes
	} else {
		search := strings.ToLower(state.TypeFilterSearch)
		for _, t := range state.AvailableTypes {
			if strings.Contains(strings.ToLower(t), search) {
				filteredTypes = append(filteredTypes, t)
			}
		}
	}

	// 弹窗尺寸
	modalWidth := 50
	if modalWidth > width-4 {
		modalWidth = width - 4
	}
	modalHeight := 20
	if modalHeight > height-6 {
		modalHeight = height - 6
	}

	// 弹窗样式
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.CSecondary).
		Padding(1, 2).
		Width(modalWidth)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.CWarning)

	// 标题
	title := titleStyle.Render("🔍 规则类型筛选")

	// 搜索框
	searchBoxStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))
	searchText := "搜索: "
	if state.TypeFilterSearch == "" {
		searchText += "_"
	} else {
		searchText += state.TypeFilterSearch + "█"
	}
	searchBox := searchBoxStyle.Render(searchText)

	// 类型列表区域高度
	listHeight := modalHeight - 6 // 标题 + 搜索框 + 提示
	if listHeight < 3 {
		listHeight = 3
	}

	// 渲染类型列表
	var typeLines []string
	visibleStart := 0
	if state.TypeFilterCursor >= listHeight {
		visibleStart = state.TypeFilterCursor - listHeight + 1
	}
	visibleEnd := visibleStart + listHeight
	if visibleEnd > len(filteredTypes) {
		visibleEnd = len(filteredTypes)
	}

	for i := visibleStart; i < visibleEnd && i < len(filteredTypes); i++ {
		typeName := filteredTypes[i]
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
			// 当前光标行
			cursorStyle := lipgloss.NewStyle().
				Background(common.CHighlight).
				Foreground(lipgloss.Color("#FFFFFF"))
			var checkMark string
			if isSelected {
				checkMark = lipgloss.NewStyle().Foreground(common.CSuccess).Render("✓ ")
			} else {
				checkMark = "  "
			}
			typeStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
			line = cursorStyle.Render(" " + checkMark + typeStyle.Render(typeName) + " ")
		} else {
			var checkMark string
			if isSelected {
				checkMark = lipgloss.NewStyle().Foreground(common.CSuccess).Render("✓ ")
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

	// 统计信息
	statsText := fmt.Sprintf("已选 %d/%d 个类型", len(state.SelectedTypes), len(state.AvailableTypes))
	if state.TypeFilterSearch != "" {
		statsText += fmt.Sprintf(" (搜索匹配 %d 个)", len(filteredTypes))
	}
	stats := common.MutedStyle.Render(statsText)

	// 帮助提示
	helpText := common.DimStyle.Render("[↑/↓]移动 [Space]选择 [Enter]确认 [Esc]取消")

	// 组装弹窗内容
	modalContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		searchBox,
		"",
		typeList,
		"",
		stats,
	)

	modal := modalStyle.Render(modalContent)

	// 使用 Place 实现居中
	centeredModal := lipgloss.Place(
		width,
		height-2, // 为底部帮助留出空间
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Left, modal, "", helpText),
	)

	// 将弹窗叠加在背景之上
	return centeredModal
}
