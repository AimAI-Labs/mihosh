package rules

import (
	"time"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	tea "github.com/charmbracelet/bubbletea"
)

// HandleMouseScroll 鼠标滚轮处理
func (s State) HandleMouseScroll(up bool) State {
	// 如果显示添加表单或编辑表单，滚动切换字段焦点
	if s.showAddForm || s.showEditForm {
		// 如果策略选择器打开，则滚动策略选择器的列表
		if s.addForm.isProxyPickerOpen() {
			form := s.addForm
			if up {
				form.movePickerCursor(-1)
			} else {
				form.movePickerCursor(1)
			}
			s.addForm = form
			return s
		}
		// 否则滚动表单字段
		form := s.addForm
		if up {
			form.cycleField(-1)
		} else {
			form.cycleField(1)
		}
		s.addForm = form
		return s
	}
	// 否则滚动规则列表
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

// HandleMouseLeft 处理规则页面鼠标左键事件。
// 弹窗打开时：点击弹窗外则确认并关闭；点击列表项则移动光标，双击切换选中。
func (s State) HandleMouseLeft(pageX, pageY, pageWidth, pageHeight int, client *api.Client) (State, tea.Cmd) {
	// 添加规则弹窗打开时
	if s.showAddForm {
		// 策略选择二级弹窗优先处理
		if s.addForm.isProxyPickerOpen() {
			pageState := s.ToPageState(pageWidth, pageHeight)
			pLeft, pTop, pRight, pBottom := ResolveProxyPickerBounds(pageState, pageWidth, pageHeight)
			if !(pageX >= pLeft && pageX < pRight && pageY >= pTop && pageY < pBottom) {
				// 点击 picker 外：关闭 picker，保留已选
				form := s.addForm
				form.pickerClose()
				s.addForm = form
				return s, nil
			}
			// 点击在 picker 内：尝试映射到列表项
			idx := ResolveProxyPickerListItemAt(pageState, pageX, pageY, pageWidth, pageHeight)
			if idx < 0 {
				return s, nil
			}
			form := s.addForm
			now := time.Now()
			isDouble := s.proxyPickerDC.IsDoubleClick(struct{}{}, idx, now)
			form.pickerCursor = idx
			if isDouble {
				filtered := form.pickerFiltered()
				if idx < len(filtered) {
					form.pickerChoose(filtered[idx])
				}
			}
			s.addForm = form
			return s, nil
		}

		// 检查是否点击了表单字段
		pageState := s.ToPageState(pageWidth, pageHeight)
		fieldIdx := ResolveFormFieldAt(pageState, pageX, pageY, pageWidth, pageHeight)
		if fieldIdx >= 0 {
			// 点击了表单字段，切换焦点
			form := s.addForm
			// 检查字段是否应该可见
			shouldFocus := true
			if fieldIdx == addFieldNoResolve && !form.isIPType() {
				shouldFocus = false
			}
			if fieldIdx == addFieldPayload && form.isMatchType() {
				shouldFocus = false
			}
			if shouldFocus {
				form.fieldCursor = fieldIdx
				form.focusCurrent()
				s.addForm = form
			}
			return s, nil
		}

		// 点击在弹窗外：取消关闭
		pageState = PageState{
			ShowAddForm: s.showAddForm,
			AddForm:     s.addForm,
			Width:       pageWidth,
			Height:      pageHeight,
		}
		left, top, right, bottom := ResolveAddFormBounds(pageState, pageWidth, pageHeight)
		if !(pageX >= left && pageX < right && pageY >= top && pageY < bottom) {
			s.showAddForm = false
			s.addForm = newAddForm(s.configPath)
		}
		return s, nil
	}

	// 删除确认弹窗打开时：点击弹窗外取消（破坏性操作的安全默认）
	if s.showDeleteConfirm {
		pageState := PageState{
			ShowDeleteConfirm: s.showDeleteConfirm,
			DeleteTarget:      s.deleteTarget,
			DeleteTargetIndex: s.deleteTargetIndex,
			Width:             pageWidth,
			Height:            pageHeight,
		}
		left, top, right, bottom := ResolveDeleteConfirmBounds(pageState, pageWidth, pageHeight)
		if !(pageX >= left && pageX < right && pageY >= top && pageY < bottom) {
			s.showDeleteConfirm = false
			s.deleteTarget = model.Rule{}
			s.deleteTargetIndex = 0
		}
		return s, nil
	}

	// 编辑规则弹窗打开时
	if s.showEditForm {
		// 策略选择二级弹窗优先处理
		if s.addForm.isProxyPickerOpen() {
			pageState := s.ToPageState(pageWidth, pageHeight)
			pLeft, pTop, pRight, pBottom := ResolveProxyPickerBounds(pageState, pageWidth, pageHeight)
			if !(pageX >= pLeft && pageX < pRight && pageY >= pTop && pageY < pBottom) {
				form := s.addForm
				form.pickerClose()
				return s.applyEditFormUpdate(form)
			}
			idx := ResolveProxyPickerListItemAt(pageState, pageX, pageY, pageWidth, pageHeight)
			if idx < 0 {
				return s, nil
			}
			form := s.addForm
			now := time.Now()
			isDouble := s.proxyPickerDC.IsDoubleClick(struct{}{}, idx, now)
			form.pickerCursor = idx
			if isDouble {
				filtered := form.pickerFiltered()
				if idx < len(filtered) {
					form.pickerChoose(filtered[idx])
				}
			}
			return s.applyEditFormUpdate(form)
		}

		// 检查是否点击了表单字段
		pageState := s.ToPageState(pageWidth, pageHeight)
		fieldIdx := ResolveFormFieldAt(pageState, pageX, pageY, pageWidth, pageHeight)
		if fieldIdx >= 0 {
			// 点击了表单字段，切换焦点
			form := s.addForm
			// 检查字段是否应该可见
			shouldFocus := true
			if fieldIdx == addFieldNoResolve && !form.isIPType() {
				shouldFocus = false
			}
			if fieldIdx == addFieldPayload && form.isMatchType() {
				shouldFocus = false
			}
			if shouldFocus {
				form.fieldCursor = fieldIdx
				form.focusCurrent()
				s.addForm = form
			}
			return s, nil
		}

		// 点击在弹窗外：取消关闭
		pageState = PageState{
			ShowEditForm: s.showEditForm,
			AddForm:      s.addForm,
			Width:        pageWidth,
			Height:       pageHeight,
		}
		left, top, right, bottom := ResolveEditFormBounds(pageState, pageWidth, pageHeight)
		if !(pageX >= left && pageX < right && pageY >= top && pageY < bottom) {
			s.showEditForm = false
			s.addForm = addForm{}
			s.editOriginIndex = 0
		}
		return s, nil
	}

	// 类型筛选弹窗打开时处理鼠标点击
	if s.showTypeFilter {
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
			s.typeFilterDC = common.DoubleClickDetector[struct{}]{}
			return s, nil
		}

		// 点击在弹窗内：尝试映射到类型列表项
		idx := ResolveTypeFilterListItemAt(pageState, pageX, pageY, pageWidth, pageHeight)
		if idx < 0 {
			return s, nil
		}

		now := time.Now()
		isDouble := s.typeFilterDC.IsDoubleClick(struct{}{}, idx, now)

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

	// 处理规则列表的单击和双击
	pageState := s.ToPageState(pageWidth, pageHeight)
	idx := ResolveMouseHit(pageState, pageY)
	if idx < 0 {
		return s, nil
	}

	now := time.Now()
	isDouble := s.ruleListDC.IsDoubleClick(struct{}{}, idx, now)

	// 单击：选中规则
	s.selectedRule = idx
	// 确保选中项可见
	if s.selectedRule < s.ruleScrollTop {
		s.ruleScrollTop = s.selectedRule
	}
	availableHeight := pageHeight - rulesFixedLines
	if availableHeight < rulesMinHeight {
		availableHeight = rulesMinHeight
	}
	if s.selectedRule >= s.ruleScrollTop+availableHeight {
		s.ruleScrollTop = s.selectedRule - availableHeight + 1
	}

	// 双击：打开编辑
	if isDouble {
		origIdx := s.filteredRuleIndices[idx]
		if origIdx >= 0 && origIdx < len(s.rules) {
			return s.openEditFormForIndex(origIdx)
		}
	}

	return s, nil
}
