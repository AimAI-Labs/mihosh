package rules

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

// renderRuleSearchBox 渲染搜索框
func renderRuleSearchBox(filterText string, filterMode bool, engine FilterEngine, selectedTypes []string) string {
	inputStyle := lipgloss.NewStyle().Foreground(common.Bright())

	if filterMode {
		inputStyle = inputStyle.Background(common.Highlight())
	}

	label := common.MutedStyle().Render(i18n.T("rules.search"))
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
			hint = common.MutedStyle().Render(i18n.T("rules.search_hint_regex"))
		case FilterEngineFuzzy:
			hint = common.MutedStyle().Render(i18n.T("rules.search_hint_fuzzy"))
		default:
			hint = common.MutedStyle().Render(i18n.T("rules.search_hint"))
		}
	}
	if len(selectedTypes) > 0 {
		typeNames := strings.Join(selectedTypes, ", ")
		typeIndicator := lipgloss.NewStyle().
			Foreground(common.Success()).
			Render(fmt.Sprintf(" [%s]", typeNames))
		return label + input + hint + engineIndicator + typeIndicator
	}
	return label + input + hint + engineIndicator
}

// renderRuleList 渲染规则列表（含整体垂直滚动条）
func renderRuleList(rules []filteredRule, selectedIdx, scrollTop, maxLines, width int, colorAdjustLight, colorAdjustDark float64) string {
	if len(rules) == 0 {
		return common.MutedStyle().Render(i18n.T("rules.empty"))
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
			barLines = append(barLines, common.MutedStyle().Foreground(common.Gray()).Render(ch))
		} else {
			barLines = append(barLines, common.DimStyle().Render(ch))
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
	indexStyle := lipgloss.NewStyle().Foreground(common.Success()).Width(6)
	indexStr := indexStyle.Render(fmt.Sprintf("%d.", index+1))

	// 类型标签
	typeStyle := lipgloss.NewStyle().Foreground(color).Bold(true).Width(16)
	typeStr := typeStyle.Render(rule.Type)

	// no-resolve 标签 (固定列宽以保持对齐)
	noResolveColWidth := 12
	noResolveStyle := lipgloss.NewStyle().Width(noResolveColWidth)
	var noResolveStr string
	if rule.NoResolve {
		noResolveStr = noResolveStyle.Foreground(common.Warning()).Render("no-resolve")
	} else {
		noResolveStr = noResolveStyle.Render("")
	}

	// Payload (减少宽度以为 no-resolve 列腾出空间)
	payloadWidth := width - 65
	if payloadWidth < 20 {
		payloadWidth = 20
	}
	payloadStyle := lipgloss.NewStyle().Foreground(common.Secondary())
	payload := rule.Payload
	if len(payload) > payloadWidth {
		payload = payload[:payloadWidth-3] + "..."
	}
	payloadStr := payloadStyle.Width(payloadWidth).Render(payload)

	// 代理
	proxyStyle := lipgloss.NewStyle().Foreground(common.Bright())
	proxyStr := proxyStyle.Render(rule.Proxy)

	// 构建行 (顺序: 序号 类型 Payload no-resolve 代理)
	line := fmt.Sprintf("%s %s %s %s %s", indexStr, typeStr, payloadStr, noResolveStr, proxyStr)

	// 选中样式
	if selected {
		line = lipgloss.NewStyle().
			Background(common.Highlight()).
			Render(common.SymbolSelectActive + line)
	} else {
		line = common.SymbolSelectInactive + line
	}

	return line
}
