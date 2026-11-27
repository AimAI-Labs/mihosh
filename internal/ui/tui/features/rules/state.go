package rules

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// FilterEngine 规则搜索匹配引擎
type FilterEngine = common.SearchEngine

const (
	FilterEngineSubstring = common.SearchSubstring
	FilterEngineRegex     = common.SearchRegex
	FilterEngineFuzzy     = common.SearchFuzzy
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

	// 规则类型筛选栏状态
	showTypeFilter   bool     // 是否显示类型筛选栏
	selectedTypes    []string // 已选择的规则类型
	availableTypes   []string // 可用规则类型列表（从规则中提取）
	typeFilterCursor int      // 光标位置（在availableTypes中的索引）

	// 鼠标双击检测（用于策略选择二级弹窗内双击选中）
	proxyPickerDC common.DoubleClickDetector[struct{}]

	// 鼠标双击检测（用于规则列表项双击打开编辑）
	ruleListDC common.DoubleClickDetector[struct{}]

	// 添加自定义规则弹窗状态
	showAddForm bool    // 是否显示添加规则弹窗
	addForm     addForm // 表单状态（仅 showAddForm 为 true 时有意义）
	configPath  string  // 当前 Mihomo 配置文件路径（由主 Model 注入）

	// 删除规则确认弹窗状态
	showDeleteConfirm bool       // 是否显示删除确认弹窗
	deleteTarget      model.Rule // 待删除规则快照（仅 showDeleteConfirm 为 true 时有意义）
	deleteTargetIndex int        // 待删除规则在 rules 列表中的原始索引（用于展示序号）

	// 编辑规则弹窗状态
	showEditForm    bool // 是否显示编辑规则弹窗
	editOriginIndex int  // 原始规则在 rules 列表中的 0-based 索引

	ColorAdjustLight float64 // 0.2-0.4 建议明度增加比例
	ColorAdjustDark  float64 // 0.15-0.25 建议明度降低比例
	client           *service.Client
	configSvc        *service.ConfigService
	coreSvc          *service.CoreService
}

// NewState 初始化规则状态
func NewState(client *service.Client, configSvc *service.ConfigService, coreSvc *service.CoreService) State {
	return State{client: client, configSvc: configSvc, coreSvc: coreSvc}
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
		// 类型筛选栏状态
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
func (s State) Update(msg tea.Msg) (State, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.CoreTargetChangedMsg:
		if msg.Reset {
			next := NewState(s.client, s.configSvc, s.coreSvc)
			next.FilterEngine = s.FilterEngine
			next.ColorAdjustLight, next.ColorAdjustDark = s.ColorAdjustLight, s.ColorAdjustDark
			s = next
		}
		return s.SetConfigPath(msg.ConfigPath), nil

	case messages.PageMouseScrollMsg:
		return s.HandleMouseScroll(msg.Up), nil
	case messages.PageMouseClickMsg:
		return s.HandleMouseLeft(msg.X, msg.Y, msg.Width, msg.Height)
	case tea.KeyMsg:
		if s.showAddForm || s.showEditForm {
			if s.addForm.typePickerOpen {
				return s.updateTypePicker(msg)
			}
			if !s.addForm.isProxyPickerOpen() && s.addForm.isTypeField() && msg.String() == " " {
				cmd := s.addForm.openTypePicker()
				return s, cmd
			}
		}
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
			s.showTypeFilter = !s.showTypeFilter
			s.typeFilterCursor = 0

		case s.showTypeFilter && msg.String() == "left":
			s.typeFilterCursor = max(0, s.typeFilterCursor-1)
		case s.showTypeFilter && msg.String() == "right":
			s.typeFilterCursor = min(len(ruleFilterTypes(s.availableTypes))-1, s.typeFilterCursor+1)
		case s.showTypeFilter && msg.String() == " ":
			types := ruleFilterTypes(s.availableTypes)
			s.typeFilterCursor = min(s.typeFilterCursor, len(types)-1)
			s.toggleType(types[s.typeFilterCursor])

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
			return s, FetchRules(s.client)

		case key.Matches(msg, common.Keys.Escape):
			s.showTypeFilter = false
			if s.ruleFilter != "" || len(s.selectedTypes) > 0 {
				s.ruleFilter = ""
				s.selectedTypes = nil
				s.selectedRule = 0
				s.ruleScrollTop = 0
				s.updateFilteredRules()
			}
		}

	}
	return s, nil
}

// ApplyRules 应用新规则列表并重建过滤缓存
func (s State) ApplyRules(rules []model.Rule) State {
	s.rules = rules
	s.extractAvailableTypes()
	s.typeFilterCursor = min(s.typeFilterCursor, len(ruleFilterTypes(s.availableTypes))-1)
	// 预分配容量与规则数相同，避免动态扩容
	if cap(s.filteredRuleIndices) < len(rules) {
		s.filteredRuleIndices = make([]int, 0, len(rules))
	}
	s.updateFilteredRules()
	if len(s.filteredRuleIndices) == 0 {
		s.selectedRule = 0
		s.ruleScrollTop = 0
	} else {
		if s.selectedRule >= len(s.filteredRuleIndices) {
			s.selectedRule = len(s.filteredRuleIndices) - 1
		}
		if s.selectedRule < 0 {
			s.selectedRule = 0
		}
		if s.ruleScrollTop > s.selectedRule {
			s.ruleScrollTop = s.selectedRule
		}
	}
	return s
}

// ShowTypeFilter 返回是否显示类型筛选栏
func (s State) ShowTypeFilter() bool { return s.showTypeFilter }

// ShowAddForm 返回是否显示添加规则弹窗
func (s State) ShowAddForm() bool { return s.showAddForm }

// ShowDeleteConfirm 返回是否显示删除规则确认弹窗
func (s State) ShowDeleteConfirm() bool { return s.showDeleteConfirm }

// ShowEditForm 返回是否显示编辑规则弹窗
func (s State) ShowEditForm() bool { return s.showEditForm }

// FilterMode 返回是否处于规则过滤模式
func (s State) FilterMode() bool { return s.ruleFilterMode }

// InputCapture declares which input belongs to the current page mode.
func (s State) InputCapture() common.InputCapture {
	textInput := s.ruleFilterMode || s.showAddForm || s.showEditForm
	active := textInput || s.showDeleteConfirm
	return common.InputCapture{Active: active, Help: textInput, Interrupt: active}
}
