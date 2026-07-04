package rules

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

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

	typeFilterDC common.DoubleClickDetector[struct{}]

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
