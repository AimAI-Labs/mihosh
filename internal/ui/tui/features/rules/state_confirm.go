package rules

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// openDeleteConfirm 针对当前选中规则打开删除确认弹窗。
// 列表为空时直接忽略（无可删项）。快照目标规则及其原始索引供渲染与提交复用。
func (s State) openDeleteConfirm() (State, tea.Cmd) {
	if len(s.filteredRuleIndices) == 0 || s.selectedRule < 0 || s.selectedRule >= len(s.filteredRuleIndices) {
		return s, nil
	}
	origIdx := s.filteredRuleIndices[s.selectedRule]
	if origIdx < 0 || origIdx >= len(s.rules) {
		return s, nil
	}
	return s.openDeleteConfirmForIndex(origIdx)
}

// openDeleteConfirmForIndex 针对指定原始索引打开删除确认弹窗（鼠标双击复用）。
func (s State) openDeleteConfirmForIndex(origIdx int) (State, tea.Cmd) {
	if origIdx < 0 || origIdx >= len(s.rules) {
		return s, nil
	}
	return State{
		rules:               s.rules,
		filteredRuleIndices: s.filteredRuleIndices,
		ruleFilter:          s.ruleFilter,
		ruleFilterMode:      s.ruleFilterMode,
		FilterEngine:        s.FilterEngine,
		selectedRule:        s.selectedRule,
		ruleScrollTop:       s.ruleScrollTop,
		showTypeFilter:      s.showTypeFilter,
		selectedTypes:       s.selectedTypes,
		availableTypes:      s.availableTypes,
		typeFilterCursor:    s.typeFilterCursor,
		showAddForm:         s.showAddForm,
		addForm:             s.addForm,
		configPath:          s.configPath,
		showDeleteConfirm:   true,
		deleteTarget:        s.rules[origIdx],
		deleteTargetIndex:   origIdx,
		showEditForm:        s.showEditForm,
		editOriginIndex:     s.editOriginIndex,
		ColorAdjustLight:    s.ColorAdjustLight,
		ColorAdjustDark:     s.ColorAdjustDark,
	}, nil
}

// handleDeleteConfirmMode 处理删除确认弹窗按键（吞掉所有按键）。
//   - Enter / y：确认，发起 DeleteRuleCmd 并关闭弹窗；
//   - Esc / n / 其它：取消关闭，不修改配置。
func (s State) handleDeleteConfirmMode(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Enter), msg.String() == "y":
		target := s.deleteTarget
		s.showDeleteConfirm = false
		s.deleteTarget = model.Rule{}
		s.deleteTargetIndex = 0
		return s, DeleteRuleCmd(
			s.configPath,
			normalizeRuleType(target.Type),
			target.Payload,
			target.Proxy,
			target.NoResolve,
		)

	case key.Matches(msg, common.Keys.Escape), msg.String() == "n":
		s.showDeleteConfirm = false
		s.deleteTarget = model.Rule{}
		s.deleteTargetIndex = 0
	}
	return s, nil
}
