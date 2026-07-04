package rules

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
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

	// 删除确认弹窗状态
	ShowDeleteConfirm bool       // 是否显示删除确认弹窗
	DeleteTarget      model.Rule // 待删除规则快照
	DeleteTargetIndex int        // 待删除规则原始索引（用于显示序号）

	// 编辑规则弹窗状态
	ShowEditForm    bool // 是否显示编辑规则弹窗
	EditOriginIndex int  // 原始规则在 rules 列表中的原始索引
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

	// 如果显示删除确认弹窗，叠加在所有弹窗之上
	if state.ShowDeleteConfirm {
		result = renderDeleteConfirmOverlay(result, state, state.Width, state.Height)
	}

	// 如果显示编辑规则弹窗，叠加在所有弹窗之上
	if state.ShowEditForm {
		result = renderEditRuleOverlay(result, state, state.Width, state.Height)
	}

	// 如果策略选择二级弹窗已打开（编辑模式），叠加在编辑弹窗之上
	if state.ShowEditForm && state.AddForm.isProxyPickerOpen() {
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
			{Key: "↵", Desc: i18n.T("help.rules_add.confirm")},
			{Key: "Esc", Desc: i18n.T("help.rules_add.picker_back")},
		}
	}

	// 策略选择二级弹窗（编辑模式）：与添加模式共用相同的提示
	if state.ShowEditForm && state.AddForm.isProxyPickerOpen() {
		return []common.InlineHelpHint{
			{Key: "Tab", Desc: i18n.T("help.rules_add.picker_tab")},
			{Key: "↑/↓", Desc: i18n.T("help.rules_add.picker_pick")},
			{Key: "↵", Desc: i18n.T("help.rules_add.confirm")},
			{Key: "Esc", Desc: i18n.T("help.rules_add.picker_back")},
		}
	}

	// 删除确认弹窗：确认删除 + 取消
	if state.ShowDeleteConfirm {
		return []common.InlineHelpHint{
			{Key: "Enter/y", Desc: i18n.T("help.rules_delete.confirm")},
			{Key: "Esc/n", Desc: i18n.T("help.rules_delete.cancel")},
		}
	}

	// 添加规则弹窗：类型切换（横向 ◀▶）+ 字段切换（纵向）+ 确认 + 取消
	if state.ShowAddForm {
		return []common.InlineHelpHint{
			{Key: "←/→", Desc: i18n.T("help.rules_add.type")},
			{Key: "↑/↓", Desc: i18n.T("help.rules_add.field")},
			{Key: "↵", Desc: i18n.T("help.rules_add.confirm")},
			{Key: "Esc", Desc: i18n.T("help.rules_add.cancel")},
		}
	}

	// 编辑规则弹窗：类型切换（横向 ◀▶）+ 字段切换（纵向）+ 确认 + 取消
	if state.ShowEditForm {
		return []common.InlineHelpHint{
			{Key: "←/→", Desc: i18n.T("help.rules_add.type")},
			{Key: "↑/↓", Desc: i18n.T("help.rules_add.field")},
			{Key: "↵", Desc: i18n.T("help.rules_add.confirm")},
			{Key: "Esc", Desc: i18n.T("help.rules_add.cancel")},
		}
	}

	// 类型筛选弹窗：移动 + 切换 + 确认 + 取消
	if state.ShowTypeFilter {
		return []common.InlineHelpHint{
			{Key: "↑/↓", Desc: i18n.T("help.rules_filter.select")},
			{Key: "Space", Desc: i18n.T("help.rules_filter.toggle")},
			{Key: "↵", Desc: i18n.T("help.rules_filter.confirm")},
			{Key: "Esc", Desc: i18n.T("help.rules_filter.cancel")},
		}
	}

	// 过滤输入模式：确认 + 取消 + 删除 + 引擎切换
	if state.FilterMode {
		return []common.InlineHelpHint{
			{Key: "↵", Desc: i18n.T("help.rules_search.confirm")},
			{Key: "Esc", Desc: i18n.T("help.rules_search.cancel")},
			{Key: "⌫", Desc: i18n.T("help.rules_search.backspace")},
			{Key: "Ctrl+R/F", Desc: i18n.T("help.rules_search.regex_fuzzy")},
		}
	}

	// 普通模式：核心操作
	return []common.InlineHelpHint{
		{Key: "↑↓", Desc: i18n.T("help.rules.hint_select")},
		{Key: "↵", Desc: i18n.T("help.rules.hint_modify")},
		{Key: "/", Desc: i18n.T("help.rules.hint_search")},
		{Key: "t", Desc: i18n.T("help.rules.hint_type")},
		{Key: "n", Desc: i18n.T("help.rules.hint_add")},
		{Key: "d", Desc: i18n.T("help.rules.hint_delete")},
		{Key: "e", Desc: i18n.T("help.rules.hint_edit")},
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
	borderStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue())
	statsStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue())

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

// ResolveMouseHit 根据 pageContent 内的 Y 坐标定位命中的规则行。
func ResolveMouseHit(state PageState, pageY int) int {
	// 计算规则列表起始行：
	// - 头部组件占 rulesHeaderHeight (3) 行（包括边框）
	// - 然后有一个空行
	listStartY := rulesHeaderHeight + 1

	if pageY < listStartY {
		return -1
	}

	// 计算规则在列表中的相对偏移
	ruleOffset := pageY - listStartY
	if ruleOffset < 0 {
		return -1
	}

	// 计算可用的规则行数
	availableHeight := state.Height - rulesFixedLines
	if availableHeight < rulesMinHeight {
		availableHeight = rulesMinHeight
	}

	// 检查是否在规则可见范围内
	numFiltered := len(state.FilteredRuleIndices)
	visibleRuleIdx := state.ScrollTop + ruleOffset

	if visibleRuleIdx < 0 || visibleRuleIdx >= numFiltered || ruleOffset >= availableHeight {
		return -1
	}

	return visibleRuleIdx
}
