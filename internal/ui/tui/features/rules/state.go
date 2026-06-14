package rules

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const rulesDoubleClickThreshold = 350 * time.Millisecond

// State 规则页面完整状态
type State struct {
	rules               []model.Rule
	filteredRuleIndices []int // 预分配，重建时重用底层数组
	ruleFilter          string
	ruleFilterMode      bool
	selectedRule        int
	ruleScrollTop       int

	// 规则类型筛选弹窗状态
	showTypeFilter   bool     // 是否显示类型筛选弹窗
	selectedTypes    []string // 已选择的规则类型
	availableTypes   []string // 可用规则类型列表（从规则中提取）
	typeFilterCursor int      // 光标位置（在availableTypes中的索引）

	// 鼠标双击检测（用于类型筛选弹窗内双击切换选中）
	lastTypeFilterClickIdx int       // 上次点击的类型项索引
	lastTypeFilterClickAt  time.Time // 上次点击时间

	ColorAdjustLight float64 // 0.2-0.4 建议明度增加比例
	ColorAdjustDark  float64 // 0.15-0.25 建议明度降低比例
}

// ToPageState 转换为渲染层所需的 PageState
func (s State) ToPageState(width, height int) PageState {
	return PageState{
		Rules:               s.rules,
		FilteredRuleIndices: s.filteredRuleIndices,
		FilterText:          s.ruleFilter,
		FilterMode:          s.ruleFilterMode,
		SelectedRule:        s.selectedRule,
		ScrollTop:           s.ruleScrollTop,
		Width:               width,
		Height:              height,
		ColorAdjustLight:    s.ColorAdjustLight,
		ColorAdjustDark:     s.ColorAdjustDark,
		// 类型筛选弹窗状态
		ShowTypeFilter:   s.showTypeFilter,
		SelectedTypes:    s.selectedTypes,
		AvailableTypes:   s.availableTypes,
		TypeFilterCursor: s.typeFilterCursor,
	}
}

// Update 处理规则页面按键
func (s State) Update(msg tea.KeyMsg, client *api.Client) (State, tea.Cmd) {
	// 类型筛选弹窗模式优先处理
	if s.showTypeFilter {
		return s.handleTypeFilterMode(msg)
	}

	if s.ruleFilterMode {
		return s.handleRuleFilterMode(msg)
	}

	switch {
	case key.Matches(msg, common.Keys.Up):
		if s.selectedRule > 0 {
			s.selectedRule--
			if s.selectedRule < s.ruleScrollTop {
				s.ruleScrollTop = s.selectedRule
			}
		}

	case key.Matches(msg, common.Keys.Down):
		if s.selectedRule < len(s.filteredRuleIndices)-1 {
			s.selectedRule++
		}

	case msg.String() == "/":
		s.ruleFilterMode = true

	case msg.String() == "t":
		// 打开类型筛选弹窗，保留已选类型以便增量调整
		s.showTypeFilter = true
		s.typeFilterCursor = 0
		s.extractAvailableTypes()

	case key.Matches(msg, common.Keys.Refresh):
		return s, FetchRules(client)

	case key.Matches(msg, common.Keys.Escape):
		if s.ruleFilter != "" || len(s.selectedTypes) > 0 {
			s.ruleFilter = ""
			s.selectedTypes = nil
			s.selectedRule = 0
			s.ruleScrollTop = 0
			s.updateFilteredRules()
		}
	}

	return s, nil
}

// HandleMouseScroll 鼠标滚轮处理
func (s State) HandleMouseScroll(up bool) State {
	count := len(s.filteredRuleIndices)
	if up {
		if s.selectedRule > 0 {
			s.selectedRule--
			if s.selectedRule < s.ruleScrollTop {
				s.ruleScrollTop = s.selectedRule
			}
		}
	} else {
		if s.selectedRule < count-1 {
			s.selectedRule++
		}
	}
	return s
}

// ApplyRules 应用新规则列表并重建过滤缓存
func (s State) ApplyRules(rules []model.Rule) State {
	s.rules = rules
	// 预分配容量与规则数相同，避免动态扩容
	if cap(s.filteredRuleIndices) < len(rules) {
		s.filteredRuleIndices = make([]int, 0, len(rules))
	}
	s.updateFilteredRules()
	return s
}

// ShowTypeFilter 返回是否显示类型筛选弹窗
func (s State) ShowTypeFilter() bool { return s.showTypeFilter }

// FilterMode 返回是否处于规则过滤模式
func (s State) FilterMode() bool { return s.ruleFilterMode }

// handleRuleFilterMode 规则过滤输入模式
func (s State) handleRuleFilterMode(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Escape):
		s.ruleFilterMode = false
	case key.Matches(msg, common.Keys.Enter):
		s.ruleFilterMode = false
		s.selectedRule = 0
		s.ruleScrollTop = 0
	case key.Matches(msg, common.Keys.Backspace):
		if len(s.ruleFilter) > 0 {
			runes := []rune(s.ruleFilter)
			if len(runes) > 0 {
				s.ruleFilter = string(runes[:len(runes)-1])
				s.updateFilteredRules()
			}
		}
	default:
		input := msg.String()
		if len(input) == 1 && input[0] >= 32 && input[0] < 127 {
			s.ruleFilter += input
			s.updateFilteredRules()
		}
	}
	return s, nil
}

// updateFilteredRules 重建规则过滤索引缓存（重用底层数组，避免频繁分配）
func (s *State) updateFilteredRules() {
	if len(s.rules) == 0 {
		s.filteredRuleIndices = s.filteredRuleIndices[:0]
		return
	}
	// 重置长度，保留底层数组
	s.filteredRuleIndices = s.filteredRuleIndices[:0]

	hasTextFilter := s.ruleFilter != ""
	hasTypeFilter := len(s.selectedTypes) > 0

	// 无过滤条件时显示全部
	if !hasTextFilter && !hasTypeFilter {
		for i := range s.rules {
			s.filteredRuleIndices = append(s.filteredRuleIndices, i)
		}
		return
	}

	// 准备关键词
	var keywords []string
	if hasTextFilter {
		keywords = strings.Fields(strings.ToLower(s.ruleFilter))
	}

	for i, rule := range s.rules {
		// 类型过滤检查
		if hasTypeFilter {
			matched := false
			ruleType := strings.ToUpper(rule.Type)
			for _, selectedType := range s.selectedTypes {
				if ruleType == strings.ToUpper(selectedType) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 文本过滤检查
		if hasTextFilter && len(keywords) > 0 {
			searchText := strings.ToLower(rule.Type + " " + rule.Payload + " " + rule.Proxy)
			allMatch := true
			for _, kw := range keywords {
				if !strings.Contains(searchText, kw) {
					allMatch = false
					break
				}
			}
			if !allMatch {
				continue
			}
		}

		s.filteredRuleIndices = append(s.filteredRuleIndices, i)
	}
}

// handleTypeFilterMode 处理类型筛选弹窗的按键
func (s State) handleTypeFilterMode(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Escape):
		// Esc 取消筛选：关闭弹窗、清空选择并恢复未筛选状态
		s.cancelAndClose()

	case key.Matches(msg, common.Keys.Enter):
		// Enter 确认：关闭弹窗并应用当前选择
		s.confirmAndClose()

	case msg.String() == " ":
		// 空格切换选择
		if s.typeFilterCursor < len(s.availableTypes) {
			typeName := s.availableTypes[s.typeFilterCursor]
			if s.isTypeSelected(typeName) {
				s.selectedTypes = removeString(s.selectedTypes, typeName)
			} else {
				s.selectedTypes = append(s.selectedTypes, typeName)
			}
			s.updateFilteredRules()
		}

	case key.Matches(msg, common.Keys.Up):
		if s.typeFilterCursor > 0 {
			s.typeFilterCursor--
		}

	case key.Matches(msg, common.Keys.Down):
		if s.typeFilterCursor < len(s.availableTypes)-1 {
			s.typeFilterCursor++
		}
	}
	return s, nil
}

// confirmAndClose 关闭类型筛选弹窗并应用当前选择（行为同 Enter / 点击弹窗外）。
func (s *State) confirmAndClose() {
	s.showTypeFilter = false
	s.typeFilterCursor = 0
	s.selectedRule = 0
	s.ruleScrollTop = 0
	s.updateFilteredRules()
}

// cancelAndClose 取消类型筛选：关闭弹窗、清空选择并恢复未筛选状态（行为同 Esc）。
func (s *State) cancelAndClose() {
	s.showTypeFilter = false
	s.typeFilterCursor = 0
	if len(s.selectedTypes) > 0 {
		s.selectedTypes = nil
		s.selectedRule = 0
		s.ruleScrollTop = 0
		s.updateFilteredRules()
	}
}

// HandleMouseLeft 处理规则页面鼠标左键事件。
// 弹窗打开时：点击弹窗外则确认并关闭；点击列表项则移动光标，双击切换选中。
func (s State) HandleMouseLeft(pageX, pageY, pageWidth, pageHeight int) (State, tea.Cmd) {
	// 仅在类型筛选弹窗打开时处理鼠标点击
	if !s.showTypeFilter {
		return s, nil
	}

	pageState := PageState{
		ShowTypeFilter:   s.showTypeFilter,
		SelectedTypes:    s.selectedTypes,
		AvailableTypes:   s.availableTypes,
		TypeFilterCursor: s.typeFilterCursor,
		Width:            pageWidth,
		Height:           pageHeight,
	}

	// 点击在弹窗内？
	left, top, right, bottom := ResolveTypeFilterModalBounds(pageState, pageWidth, pageHeight)
	insideModal := pageX >= left && pageX < right && pageY >= top && pageY < bottom
	if !insideModal {
		// 点击边框外：确认并关闭（保留已勾选的过滤）
		s.confirmAndClose()
		// 重置双击状态，避免下次打开误触发
		s.lastTypeFilterClickIdx = 0
		s.lastTypeFilterClickAt = time.Time{}
		return s, nil
	}

	// 点击在弹窗内：尝试映射到类型列表项
	idx := ResolveTypeFilterListItemAt(pageState, pageX, pageY, pageWidth, pageHeight)
	if idx < 0 {
		return s, nil
	}

	now := time.Now()
	isDouble := s.isTypeFilterDoubleClick(idx, now)

	// 单击：移动光标到该项
	s.typeFilterCursor = idx

	// 双击：切换该项选中状态
	if isDouble {
		typeName := s.availableTypes[idx]
		if s.isTypeSelected(typeName) {
			s.selectedTypes = removeString(s.selectedTypes, typeName)
		} else {
			s.selectedTypes = append(s.selectedTypes, typeName)
		}
		s.updateFilteredRules()
	}
	return s, nil
}

// isTypeFilterDoubleClick 检测类型筛选弹窗内的双击（与 nodes/settings 模式一致）。
func (s *State) isTypeFilterDoubleClick(idx int, now time.Time) bool {
	isDouble := idx == s.lastTypeFilterClickIdx &&
		!s.lastTypeFilterClickAt.IsZero() &&
		now.Sub(s.lastTypeFilterClickAt) <= rulesDoubleClickThreshold
	s.lastTypeFilterClickIdx = idx
	s.lastTypeFilterClickAt = now
	return isDouble
}

// extractAvailableTypes 从规则中提取所有可用的规则类型
func (s *State) extractAvailableTypes() {
	typeSet := make(map[string]struct{})
	for _, rule := range s.rules {
		if rule.Type != "" {
			typeSet[rule.Type] = struct{}{}
		}
	}
	s.availableTypes = make([]string, 0, len(typeSet))
	for t := range typeSet {
		s.availableTypes = append(s.availableTypes, t)
	}
}

// isTypeSelected 检查类型是否已被选中
func (s State) isTypeSelected(typeName string) bool {
	for _, t := range s.selectedTypes {
		if t == typeName {
			return true
		}
	}
	return false
}

// removeString 从切片中移除指定字符串
func removeString(slice []string, target string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != target {
			result = append(result, s)
		}
	}
	return result
}
