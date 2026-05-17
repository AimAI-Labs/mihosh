package rules

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

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
	typeFilterSearch string   // 类型搜索文本
	selectedTypes    []string // 已选择的规则类型
	availableTypes   []string // 可用规则类型列表（从规则中提取）
	typeFilterCursor int      // 光标位置（在availableTypes中的索引）

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
		TypeFilterSearch: s.typeFilterSearch,
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
		s.showTypeFilter = true
		s.typeFilterSearch = ""
		s.typeFilterCursor = 0
		s.selectedTypes = nil
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
			s.ruleFilter = s.ruleFilter[:len(s.ruleFilter)-1]
			s.updateFilteredRules()
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
		s.showTypeFilter = false
		s.typeFilterSearch = ""

	case key.Matches(msg, common.Keys.Enter):
		s.showTypeFilter = false
		s.typeFilterSearch = ""
		s.selectedRule = 0
		s.ruleScrollTop = 0
		s.updateFilteredRules()

	case msg.String() == " ":
		// 空格切换选择
		filteredTypes := s.getFilteredTypes()
		if s.typeFilterCursor < len(filteredTypes) {
			typeName := filteredTypes[s.typeFilterCursor]
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
		filteredTypes := s.getFilteredTypes()
		if s.typeFilterCursor < len(filteredTypes)-1 {
			s.typeFilterCursor++
		}

	case key.Matches(msg, common.Keys.Backspace):
		if len(s.typeFilterSearch) > 0 {
			s.typeFilterSearch = s.typeFilterSearch[:len(s.typeFilterSearch)-1]
			s.typeFilterCursor = 0
		}

	default:
		// 搜索文本输入
		input := msg.String()
		if len(input) == 1 && input[0] >= 32 && input[0] < 127 {
			s.typeFilterSearch += input
			s.typeFilterCursor = 0
		}
	}
	return s, nil
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

// getFilteredTypes 根据搜索文本过滤可用类型
func (s State) getFilteredTypes() []string {
	if s.typeFilterSearch == "" {
		return s.availableTypes
	}
	search := strings.ToLower(s.typeFilterSearch)
	var filtered []string
	for _, t := range s.availableTypes {
		if strings.Contains(strings.ToLower(t), search) {
			filtered = append(filtered, t)
		}
	}
	return filtered
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
