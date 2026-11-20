package rules

import (
	"slices"
	"strconv"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// handleRuleFilterMode 规则过滤输入模式
func (s State) handleRuleFilterMode(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Escape):
		s.ruleFilterMode = false
	case key.Matches(msg, common.Keys.Enter):
		s.ruleFilterMode = false
		s.selectedRule = 0
		s.ruleScrollTop = 0
	default:
		query := common.SearchQuery{Text: s.ruleFilter, Engine: s.FilterEngine}
		if query.Edit(msg) {
			s.ruleFilter, s.FilterEngine = query.Text, query.Engine
			s.updateFilteredRules()
			s.selectedRule = 0
			s.ruleScrollTop = 0
		}
	}
	return s, nil
}

func (s *State) toggleType(name string) {
	if !slices.Contains(s.availableTypes, name) {
		return
	}
	if s.isTypeSelected(name) {
		s.selectedTypes = removeString(s.selectedTypes, name)
	} else {
		s.selectedTypes = append(slices.Clone(s.selectedTypes), name)
	}
	s.selectedRule, s.ruleScrollTop = 0, 0
	s.updateFilteredRules()
}

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

	match := (common.SearchQuery{Text: s.ruleFilter, Engine: s.FilterEngine}).Matcher(common.MatchAllKeywords)

	for i, rule := range s.rules {
		// 类型过滤检查
		if hasTypeFilter {
			matched := false
			ruleType := filterTypeName(rule.Type)
			for _, selectedType := range s.selectedTypes {
				if ruleType == selectedType {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 文本过滤检查（按引擎分派）
		if hasTextFilter {
			searchText := rule.Type + " " + rule.Payload + " " + rule.Proxy
			// no-resolve 作为可搜索标签加入文本（输入 no-resolve / resolve / nr-... 即可筛选）
			if rule.NoResolve {
				searchText += " no-resolve"
			}
			// 序号也加入搜索文本（1-based）
			searchText += " " + strconv.Itoa(i+1)
			if !match(searchText) {
				continue
			}
		}

		s.filteredRuleIndices = append(s.filteredRuleIndices, i)
	}
}

// extractAvailableTypes 从规则中提取所有可用的规则类型
func (s *State) extractAvailableTypes() {
	typeSet := make(map[string]struct{})
	for _, rule := range s.rules {
		if rule.Type != "" {
			typeSet[filterTypeName(rule.Type)] = struct{}{}
		}
	}
	s.availableTypes = make([]string, 0, len(typeSet))
	for t := range typeSet {
		s.availableTypes = append(s.availableTypes, t)
	}
	slices.Sort(s.availableTypes)
	s.selectedTypes = slices.DeleteFunc(slices.Clone(s.selectedTypes), func(name string) bool {
		_, present := typeSet[name]
		return !present
	})
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
