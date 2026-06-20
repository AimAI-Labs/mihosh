package rules

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"regexp"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const rulesDoubleClickThreshold = 350 * time.Millisecond

// FilterEngine 规则搜索匹配引擎
type FilterEngine int

const (
	FilterEngineSubstring FilterEngine = iota // 普通子串（多关键词 AND）
	FilterEngineRegex                         // 正则匹配
	FilterEngineFuzzy                         // 模糊匹配（有序子序列）
)

// State 规则页面完整状态
type State struct {
	rules               []model.Rule
	filteredRuleIndices []int // 预分配，重建时重用底层数组
	ruleFilter          string
	ruleFilterMode      bool
	FilterEngine        FilterEngine // 当前搜索匹配引擎
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

	// 鼠标双击检测（用于策略选择二级弹窗内双击选中）
	lastProxyPickerClickIdx int
	lastProxyPickerClickAt  time.Time

	// 添加自定义规则弹窗状态
	showAddForm bool    // 是否显示添加规则弹窗
	addForm     addForm // 表单状态（仅 showAddForm 为 true 时有意义）
	configPath  string  // 当前 Mihomo 配置文件路径（由主 Model 注入）

 // 删除规则确认弹窗状态
	showDeleteConfirm bool        // 是否显示删除确认弹窗
	deleteTarget      model.Rule  // 待删除规则快照（仅 showDeleteConfirm 为 true 时有意义）
	deleteTargetIndex int         // 待删除规则在 rules 列表中的原始索引（用于展示序号）

	// 编辑规则弹窗状态
	showEditForm        bool       // 是否显示编辑规则弹窗
	editOriginIndex     int        // 原始规则在 rules 列表中的 0-based 索引

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
		FilterEngine:        s.FilterEngine,
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
		// 添加规则弹窗状态
		ShowAddForm: s.showAddForm,
		AddForm:     s.addForm,
		// 删除确认弹窗状态
		ShowDeleteConfirm: s.showDeleteConfirm,
		DeleteTarget:      s.deleteTarget,
		DeleteTargetIndex: s.deleteTargetIndex,
		// 编辑规则弹窗状态
		ShowEditForm:    s.showEditForm,
		EditOriginIndex: s.editOriginIndex,
	}
}

// SetConfigPath 由主 Model 在初始化时注入 Mihomo 配置文件路径。
func (s State) SetConfigPath(path string) State {
	s.configPath = path
	return s
}

// Update 处理规则页面按键
func (s State) Update(msg tea.KeyMsg, client *api.Client) (State, tea.Cmd) {
	// 编辑规则弹窗优先拦截（吞掉所有按键）
	if s.showEditForm {
		return s.handleEditFormMode(msg)
	}

	// 添加规则弹窗优先拦截（吞掉所有按键）
	if s.showAddForm {
		return s.handleAddFormMode(msg)
	}

	// 删除确认弹窗拦截（吞掉所有按键）
	if s.showDeleteConfirm {
		return s.handleDeleteConfirmMode(msg)
	}

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

	case msg.String() == "n":
		// 打开添加自定义规则弹窗（每次打开重置为干净表单）
		s.showAddForm = true
		s.addForm = newAddForm(s.configPath)

	case msg.String() == "d":
		// 打开删除确认弹窗（针对当前选中规则）
		return s.openDeleteConfirm()

	case key.Matches(msg, common.Keys.Enter):
		// Enter 打开编辑规则弹窗（针对当前选中规则）
		return s.openEditForm()
	
	case msg.String() == "e":
		// 在外部编辑器中打开配置文件
		return s.openConfigEditor()

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

// ShowAddForm 返回是否显示添加规则弹窗
func (s State) ShowAddForm() bool { return s.showAddForm }

// ShowDeleteConfirm 返回是否显示删除规则确认弹窗
func (s State) ShowDeleteConfirm() bool { return s.showDeleteConfirm }

// ShowEditForm 返回是否显示编辑规则弹窗
func (s State) ShowEditForm() bool { return s.showEditForm }

// FilterMode 返回是否处于规则过滤模式
func (s State) FilterMode() bool { return s.ruleFilterMode }

// handleRuleFilterMode 规则过滤输入模式
func (s State) handleRuleFilterMode(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyCtrlR:
		// Ctrl+R 切换 正则↔普通
		if s.FilterEngine == FilterEngineRegex {
			s.FilterEngine = FilterEngineSubstring
		} else {
			s.FilterEngine = FilterEngineRegex
		}
		s.updateFilteredRules()
		s.selectedRule = 0
		s.ruleScrollTop = 0
	case msg.Type == tea.KeyCtrlF:
		// Ctrl+F 切换 模糊↔普通
		if s.FilterEngine == FilterEngineFuzzy {
			s.FilterEngine = FilterEngineSubstring
		} else {
			s.FilterEngine = FilterEngineFuzzy
		}
		s.updateFilteredRules()
		s.selectedRule = 0
		s.ruleScrollTop = 0
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
				s.selectedRule = 0
				s.ruleScrollTop = 0
			}
		}
	default:
		input := msg.String()
		// 接受可打印的单字符（ASCII）或多字节字符（如中文）
		runes := []rune(input)
		if len(runes) == 1 && runes[0] >= 32 {
			s.ruleFilter += input
			s.updateFilteredRules()
			s.selectedRule = 0
			s.ruleScrollTop = 0
		}
	}
	return s, nil
}

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

	// 预编译正则 / 预处理子串
	var re *regexp.Regexp
	var keywords []string
	if hasTextFilter {
		switch s.FilterEngine {
		case FilterEngineRegex:
			var err error
			re, err = regexp.Compile("(?i)" + s.ruleFilter)
			if err != nil {
				// 非法正则：文本过滤匹配为空（仍可能因类型过滤保留结果）
				re = nil
			}
		case FilterEngineSubstring:
			// 普通模式：保留多关键词 AND 语义
			keywords = strings.Fields(strings.ToLower(s.ruleFilter))
		}
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

		// 文本过滤检查（按引擎分派）
		if hasTextFilter {
			searchText := rule.Type + " " + rule.Payload + " " + rule.Proxy
			// no-resolve 作为可搜索标签加入文本（输入 no-resolve / resolve / nr-... 即可筛选）
			if rule.NoResolve {
				searchText += " no-resolve"
			}
			switch s.FilterEngine {
			case FilterEngineRegex:
				if re == nil || !re.MatchString(searchText) {
					continue
				}
			case FilterEngineFuzzy:
				if !fuzzyMatch(s.ruleFilter, searchText) {
					continue
				}
			case FilterEngineSubstring:
				fallthrough
			default:
				lower := strings.ToLower(searchText)
				allMatch := true
				for _, kw := range keywords {
					if !strings.Contains(lower, kw) {
						allMatch = false
						break
					}
				}
				if !allMatch {
					continue
				}
			}
		}

		s.filteredRuleIndices = append(s.filteredRuleIndices, i)
	}
}

// fuzzyMatch 检查 pattern 的所有字符是否按顺序出现于 text 中（大小写不敏感）。
// 与 nodes 页面的实现保持一致。
func fuzzyMatch(pattern, text string) bool {
	if pattern == "" {
		return true
	}
	pattern = strings.ToLower(pattern)
	text = strings.ToLower(text)

	pIdx := 0
	for i := 0; i < len(text) && pIdx < len(pattern); i++ {
		if text[i] == pattern[pIdx] {
			pIdx++
		}
	}
	return pIdx == len(pattern)
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
			isDouble := s.isProxyPickerDoubleClick(idx, now)
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

		pageState := PageState{
			ShowAddForm: s.showAddForm,
			AddForm:     s.addForm,
			Width:       pageWidth,
			Height:      pageHeight,
		}
		left, top, right, bottom := ResolveAddFormBounds(pageState, pageWidth, pageHeight)
		if !(pageX >= left && pageX < right && pageY >= top && pageY < bottom) {
			// 点击边框外：取消关闭
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
			isDouble := s.isProxyPickerDoubleClick(idx, now)
			form.pickerCursor = idx
			if isDouble {
				filtered := form.pickerFiltered()
				if idx < len(filtered) {
					form.pickerChoose(filtered[idx])
				}
			}
			return s.applyEditFormUpdate(form)
		}

		pageState := PageState{
			ShowEditForm: s.showEditForm,
			AddForm:      s.addForm,
			Width:        pageWidth,
			Height:       pageHeight,
		}
		left, top, right, bottom := ResolveEditFormBounds(pageState, pageWidth, pageHeight)
		if !(pageX >= left && pageX < right && pageY >= top && pageY < bottom) {
			// 点击边框外：取消关闭
			s.showEditForm = false
			s.addForm = addForm{}
			s.editOriginIndex = 0
		}
		return s, nil
	}

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

// isProxyPickerDoubleClick 检测策略选择弹窗内的双击。
func (s *State) isProxyPickerDoubleClick(idx int, now time.Time) bool {
	isDouble := idx == s.lastProxyPickerClickIdx &&
		!s.lastProxyPickerClickAt.IsZero() &&
		now.Sub(s.lastProxyPickerClickAt) <= rulesDoubleClickThreshold
 s.lastProxyPickerClickIdx = idx
	s.lastProxyPickerClickAt = now
	return isDouble
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
