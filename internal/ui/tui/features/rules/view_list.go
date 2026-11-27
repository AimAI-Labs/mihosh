package rules

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

	engineIndicator := common.RenderSearchEngineBadge(engine)

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

	window := common.ResolveListWindow(len(rules), selectedIdx, scrollTop, maxLines)
	selectedIdx, scrollTop = window.Selected, window.Start

	// 渲染规则行（预留滚动条宽度）
	listWidth := max(1, width-rulesScrollWidth)
	var lines []string
	for i := scrollTop; i < window.End; i++ {
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
	prefixWidth := common.DisplayWidth(common.SymbolSelectInactive)

	indexColWidth := 6
	typeColWidth := 16
	noResolveColWidth := 12
	payloadWidth := max(20, width-65)
	if width < 65 {
		indexColWidth = 4
		typeColWidth = 10
		noResolveColWidth = 0
		if rule.NoResolve {
			noResolveColWidth = 10
		}
		gaps := 3
		if noResolveColWidth > 0 {
			gaps = 4
		}
		proxyBudget := min(common.DisplayWidth(rule.Proxy), max(6, width/5))
		payloadWidth = max(4, width-prefixWidth-indexColWidth-typeColWidth-noResolveColWidth-proxyBudget-gaps)
	}

	// 序号
	indexStyle := lipgloss.NewStyle().Foreground(common.Success()).Width(indexColWidth)
	indexStr := indexStyle.Render(fmt.Sprintf("%d.", index+1))

	// 类型标签
	typeStyle := lipgloss.NewStyle().Foreground(color).Bold(true).Width(typeColWidth)
	typeStr := typeStyle.Render(common.TruncateDisplay(rule.Type, typeColWidth))

	// no-resolve 标签
	var noResolveStr string
	if noResolveColWidth > 0 {
		noResolveStyle := lipgloss.NewStyle().Width(noResolveColWidth)
		if rule.NoResolve {
			noResolveStr = noResolveStyle.Foreground(common.Warning()).Render("no-resolve")
		} else {
			noResolveStr = noResolveStyle.Render("")
		}
	}

	// Payload
	payloadStyle := lipgloss.NewStyle().Foreground(common.Secondary())
	payload := common.TruncateDisplay(rule.Payload, payloadWidth)
	payloadStr := payloadStyle.Width(payloadWidth).Render(payload)

	// 代理
	proxyStyle := lipgloss.NewStyle().Foreground(common.Bright())
	proxyStr := proxyStyle.Render(rule.Proxy)

	// 构建行 (顺序: 序号 类型 Payload [no-resolve] 代理)
	var line string
	if noResolveColWidth > 0 {
		line = fmt.Sprintf("%s %s %s %s %s", indexStr, typeStr, payloadStr, noResolveStr, proxyStr)
	} else {
		line = fmt.Sprintf("%s %s %s %s", indexStr, typeStr, payloadStr, proxyStr)
	}
	line = ansi.Truncate(line, max(1, width-prefixWidth), "")

	// 选中样式
	if selected {
		line = lipgloss.NewStyle().
			Background(common.Highlight()).
			Render(ansi.Truncate(common.SymbolSelectActive+line, max(1, width), ""))
	} else {
		line = ansi.Truncate(common.SymbolSelectInactive+line, max(1, width), "")
	}

	return line
}
