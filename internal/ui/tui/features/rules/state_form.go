package rules

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// handleAddFormMode 处理「添加自定义规则」弹窗按键（吞掉所有按键）。
// 若策略选择二级弹窗已打开，则优先分派至 handleProxyPickerMode。
func (s State) handleAddFormMode(msg tea.KeyMsg) (State, tea.Cmd) {
	form := s.addForm

	// 策略选择二级弹窗优先拦截（吞掉所有键）
	if form.isProxyPickerOpen() {
		return s.handleProxyPickerMode(msg)
	}

	switch {
	case key.Matches(msg, common.Keys.Escape):
		// 放弃关闭，清空表单
		s.showAddForm = false
		s.addForm = newAddForm(s.configPath)
		return s, nil

	case key.Matches(msg, common.Keys.Enter):
		// 策略行聚焦时 Enter 打开二级弹窗，不提交
		if form.isProxyField() {
			form.openProxyPicker()
			s.addForm = form
			return s, nil
		}
		ok, errKey := form.validate()
		if !ok {
			form.errMsg = i18n.T(errKey)
			s.addForm = form
			return s, nil
		}
		// 校验通过：发起写盘命令，重置表单
		submit := form
		s.showAddForm = false
		s.addForm = newAddForm(s.configPath)
		cmd := AddRuleCmd(
			s.configPath,
			submit.currentType(),
			submit.fields[addFieldPayload].Value(),
			submit.currentProxy(),
			submit.resolveIndex(),
			submit.noResolve,
		)
		return s, cmd

	case msg.String() == "tab":
		form.cycleField(1)

	case msg.String() == "shift+tab":
		form.cycleField(-1)

	case key.Matches(msg, common.Keys.Up):
		// 字段为垂直堆叠，↑/↓ 在字段间上下移动
		form.cycleField(-1)

	case key.Matches(msg, common.Keys.Down):
		// 字段为垂直堆叠，↑/↓ 在字段间上下移动
		form.cycleField(1)

	case key.Matches(msg, common.Keys.Left):
		// ←/→ 语义随焦点变化：
		//   - 类型行聚焦 → 切换规则类型
		//   - 匹配值/位置聚焦 → 透传 textinput 移动光标
		//   - 策略/no-resolve（只读）→ 忽略
		switch {
		case form.isTypeField():
			form.cycleType(-1)
		case form.isProxyField(), form.isNoResolveField():
			// 只读字段，忽略
		default:
			updated, cmd := form.fields[form.fieldCursor].Update(msg)
			form.fields[form.fieldCursor] = updated
			form.errMsg = ""
			s.addForm = form
			return s, cmd
		}

	case key.Matches(msg, common.Keys.Right):
		switch {
		case form.isTypeField():
			form.cycleType(1)
		case form.isProxyField(), form.isNoResolveField():
			// 只读字段，忽略
		default:
			updated, cmd := form.fields[form.fieldCursor].Update(msg)
			form.fields[form.fieldCursor] = updated
			form.errMsg = ""
			s.addForm = form
			return s, cmd
		}

	case msg.String() == " ":
		// Space 切换 no-resolve 复选框
		if form.isNoResolveField() {
			form.toggleNoResolve()
		}

	default:
		// 透传给当前 textinput（Backspace / 可打印字符等）
		// 策略和 no-resolve 行为只读，不接收文本输入
		if form.isProxyField() || form.isNoResolveField() || !isTextInputKey(msg) {
			break
		}
		cur := form.fields[form.fieldCursor]
		updated, cmd := cur.Update(msg)
		form.fields[form.fieldCursor] = updated
		form.errMsg = "" // 输入即清除上次错误
		s.addForm = form
		return s, cmd
	}

	s.addForm = form
	return s, nil
}

// handleEditFormMode 处理编辑规则弹窗按键（吞掉所有按键）。
// 逻辑与 handleAddFormMode 完全相同（复用 addForm），仅在提交时：
//   - 调用 EditRuleCmd（ReplaceRule：删旧+插新）而非 AddRuleCmd。
func (s State) handleEditFormMode(msg tea.KeyMsg) (State, tea.Cmd) {
	form := s.addForm

	// 策略选择二级弹窗优先拦截（吞掉所有键）
	if form.isProxyPickerOpen() {
		return s.handleProxyPickerMode(msg)
	}

	switch {
	case key.Matches(msg, common.Keys.Escape):
		// 取消编辑，关闭弹窗
		s.showEditForm = false
		s.addForm = addForm{}
		s.editOriginIndex = 0
		return s, nil

	case key.Matches(msg, common.Keys.Enter):
		// 策略行聚焦时 Enter 打开二级弹窗，不提交
		if form.isProxyField() {
			form.openProxyPicker()
			s.addForm = form
			return s, nil
		}
		ok, errKey := form.validate()
		if !ok {
			form.errMsg = i18n.T(errKey)
			s.addForm = form
			return s, nil
		}
		// 校验通过：发起编辑命令（ReplaceRule：删旧+插新），重置弹窗
		submit := form
		s.showEditForm = false
		s.addForm = addForm{}
		s.editOriginIndex = 0
		cmd := EditRuleCmd(
			s.configPath,
			submit.editOrigin.ruleType,
			submit.editOrigin.payload,
			submit.editOrigin.proxy,
			submit.editOrigin.noResolve,
			submit.currentType(),
			submit.fields[addFieldPayload].Value(),
			submit.currentProxy(),
			submit.resolveIndex(),
			submit.noResolve,
		)
		return s, cmd

	case msg.String() == "tab":
		form.cycleField(1)

	case msg.String() == "shift+tab":
		form.cycleField(-1)

	case key.Matches(msg, common.Keys.Up):
		form.cycleField(-1)

	case key.Matches(msg, common.Keys.Down):
		form.cycleField(1)

	case key.Matches(msg, common.Keys.Left):
		switch {
		case form.isTypeField():
			form.cycleType(-1)
		case form.isProxyField(), form.isNoResolveField():
			// 只读字段，忽略
		default:
			updated, cmd := form.fields[form.fieldCursor].Update(msg)
			form.fields[form.fieldCursor] = updated
			form.errMsg = ""
			s.addForm = form
			return s, cmd
		}

	case key.Matches(msg, common.Keys.Right):
		switch {
		case form.isTypeField():
			form.cycleType(1)
		case form.isProxyField(), form.isNoResolveField():
			// 只读字段，忽略
		default:
			updated, cmd := form.fields[form.fieldCursor].Update(msg)
			form.fields[form.fieldCursor] = updated
			form.errMsg = ""
			s.addForm = form
			return s, cmd
		}

	case msg.String() == " ":
		if form.isNoResolveField() {
			form.toggleNoResolve()
		}

	default:
		if form.isProxyField() || form.isNoResolveField() || !isTextInputKey(msg) {
			break
		}
		cur := form.fields[form.fieldCursor]
		updated, cmd := cur.Update(msg)
		form.fields[form.fieldCursor] = updated
		form.errMsg = ""
		s.addForm = form
		return s, cmd
	}

	s.addForm = form
	return s, nil
}

// openEditForm 针对当前选中规则打开编辑弹窗。
// 列表为空时直接忽略。编辑弹窗使用 newEditForm 预填充原始规则值。
func (s State) openEditForm() (State, tea.Cmd) {
	if len(s.filteredRuleIndices) == 0 || s.selectedRule < 0 || s.selectedRule >= len(s.filteredRuleIndices) {
		return s, nil
	}
	origIdx := s.filteredRuleIndices[s.selectedRule]
	if origIdx < 0 || origIdx >= len(s.rules) {
		return s, nil
	}
	return s.openEditFormForIndex(origIdx)
}

// openEditFormForIndex 针对指定原始索引打开编辑弹窗（鼠标双击复用）。
func (s State) openEditFormForIndex(origIdx int) (State, tea.Cmd) {
	if origIdx < 0 || origIdx >= len(s.rules) {
		return s, nil
	}
	form := newEditForm(s.configPath, s.rules[origIdx], origIdx)
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
		showAddForm:         false,
		addForm:             form,
		configPath:          s.configPath,
		showDeleteConfirm:   false,
		deleteTarget:        model.Rule{},
		deleteTargetIndex:   0,
		showEditForm:        true,
		editOriginIndex:     origIdx,
		ColorAdjustLight:    s.ColorAdjustLight,
		ColorAdjustDark:     s.ColorAdjustDark,
	}, nil
}

// applyEditFormUpdate 将更新后的 addForm 应用到 State，保留其他字段不变。
func (s State) applyEditFormUpdate(form addForm) (State, tea.Cmd) {
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
		addForm:             form,
		configPath:          s.configPath,
		showDeleteConfirm:   s.showDeleteConfirm,
		deleteTarget:        s.deleteTarget,
		deleteTargetIndex:   s.deleteTargetIndex,
		showEditForm:        s.showEditForm,
		editOriginIndex:     s.editOriginIndex,
		ColorAdjustLight:    s.ColorAdjustLight,
		ColorAdjustDark:     s.ColorAdjustDark,
	}, nil
}

// handleProxyPickerMode 处理策略选择二级弹窗按键（吞掉所有键）。
func (s State) handleProxyPickerMode(msg tea.KeyMsg) (State, tea.Cmd) {
	form := s.addForm

	switch {
	case key.Matches(msg, common.Keys.Escape):
		// Esc：关闭弹窗返回主表单，保留已选策略
		form.pickerClose()

	case key.Matches(msg, common.Keys.Enter):
		// Enter：选中当前过滤项并回填
		filtered := form.pickerFiltered()
		if len(filtered) > 0 && form.pickerCursor < len(filtered) {
			form.pickerChoose(filtered[form.pickerCursor])
		}
		// 过滤列表为空时忽略

	case msg.String() == "tab":
		form.cyclePickerTab(1)

	case msg.String() == "shift+tab":
		form.cyclePickerTab(-1)

	case key.Matches(msg, common.Keys.Up):
		form.movePickerCursor(-1)

	case key.Matches(msg, common.Keys.Down):
		form.movePickerCursor(1)

	case key.Matches(msg, common.Keys.Backspace):
		form.pickerBackspace()

	default:
		// 可打印字符追加到搜索框
		runes := msg.Runes
		if len(runes) == 0 && msg.Type == tea.KeyRunes {
			runes = []rune(msg.String())
		}
		for _, r := range runes {
			if r >= 32 {
				form.pickerTypeChar(r)
			}
		}
	}

	s.addForm = form
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
