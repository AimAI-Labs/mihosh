package sub

// state.go — 订阅页完整状态与按键/鼠标分发。
//
// 交互模式（互斥）：普通列表 / 搜索输入 / 添加表单 / merge 编辑器 / 删除确认。
// 弹窗激活时吞掉底层按键（与 rules 页一致）。

import (
	"regexp"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// 交互模式常量（供 help 包镜像，见 help/view.go subMode*）。
const (
	ModeNormal      = 0
	ModeSearch      = 1
	ModeAddForm     = 2
	ModeMergeEditor = 3
	ModeDeleteConf  = 4
)

const subDoubleClickThreshold = 350 * time.Millisecond

// State 订阅页完整状态。
type State struct {
	subs          []profile.Profile
	filteredIdx   []int // 过滤后的订阅在 subs 中的索引
	activeUID     string
	filter        string
	filterMode    bool
	filterEngine  FilterEngine
	selected      int
	scrollTop     int

	// 弹窗/输入模式
	showAddForm     bool
	addForm         addForm
	showMergeEditor bool
	mergeEditor     mergeEditor
	showDeleteConf  bool
	deleteUID       string // 待删除订阅 UID

	// merge 编辑器打开时正在加载的目标 UID（LoadMerge 命令发出后等待回填）
	mergeLoadingUID string

	// 鼠标双击检测
	lastMouseIdx int
	lastMouseAt  time.Time

	updatingUID string // 当前正在更新的订阅 UID
}

// FilterEngine 订阅搜索匹配引擎（与 rules 页一致语义）。
type FilterEngine int

const (
	FilterSubstring FilterEngine = iota
	FilterRegex
	FilterFuzzy
)

// ToPageState 转换为渲染层所需的 PageState。
func (s State) ToPageState(width, height int) PageState {
	return PageState{
		Subs:           s.subs,
		FilteredIdx:    s.filteredIdx,
		ActiveUID:      s.activeUID,
		FilterText:     s.filter,
		FilterMode:     s.filterMode,
		FilterEngine:   s.filterEngine,
		Selected:       s.selected,
		ScrollTop:      s.scrollTop,
		Width:          width,
		Height:         height,
		ShowAddForm:    s.showAddForm,
		AddForm:        s.addForm,
		ShowMergeEdit:  s.showMergeEditor,
		MergeEditor:    s.mergeEditor,
		ShowDeleteConf: s.showDeleteConf,
		DeleteUID:      s.deleteUID,
		UpdatingUID:    s.updatingUID,
	}
}

// Mode 返回当前交互模式（供 help 弹窗）。
func (s State) Mode() int {
	switch {
	case s.filterMode:
		return ModeSearch
	case s.showAddForm:
		return ModeAddForm
	case s.showMergeEditor:
		return ModeMergeEditor
	case s.showDeleteConf:
		return ModeDeleteConf
	default:
		return ModeNormal
	}
}

// Querying 是否处于输入捕获模式（供主 Model 的 isInputCapturing 判断）。
func (s State) Querying() bool {
	return s.filterMode || s.showAddForm || s.showMergeEditor || s.showDeleteConf
}

// ApplySubs 应用加载的订阅列表与激活 UID，并重建过滤缓存。
func (s State) ApplySubs(subs []profile.Profile, active string) State {
	s.subs = subs
	s.activeUID = active
	if cap(s.filteredIdx) < len(subs) {
		s.filteredIdx = make([]int, 0, len(subs))
	}
	s.updateFiltered()
	return s
}

// Update 处理按键，分派到当前激活的子模式。
func (s State) Update(msg tea.KeyMsg, svc *service.ProfileService) (State, tea.Cmd) {
	// 弹窗优先拦截
	if s.showDeleteConf {
		return s.handleDeleteConfirm(msg, svc)
	}
	if s.showMergeEditor {
		return s.handleMergeEditorUpdate(msg, svc)
	}
	if s.showAddForm {
		return s.handleAddFormUpdate(msg, svc)
	}
	if s.filterMode {
		return s.handleFilterMode(msg, svc)
	}

	switch {
	case key.Matches(msg, common.Keys.Up):
		if s.selected > 0 {
			s.selected--
			s.clampScroll()
		}
	case key.Matches(msg, common.Keys.Down):
		if s.selected < len(s.filteredIdx)-1 {
			s.selected++
			s.clampScroll()
		}
	case msg.String() == "enter":
		return s.activateSelected(svc)
	case msg.String() == "u":
		return s.updateSelected(svc)
	case msg.String() == "e":
		return s.openMergeEditor(svc)
	case msg.String() == "n":
		s.showAddForm = true
		s.addForm = newAddForm()
	case msg.String() == "d":
		return s.openDeleteConfirm()
	case msg.String() == "/":
		s.filterMode = true
		s.filter = ""
	case key.Matches(msg, common.Keys.Refresh):
		return s, FetchSubs(svc)
	case key.Matches(msg, common.Keys.Escape):
		if s.filter != "" {
			s.filter = ""
			s.selected = 0
			s.scrollTop = 0
			s.updateFiltered()
		}
	}
	return s, nil
}

// handleFilterMode 搜索输入模式。
func (s State) handleFilterMode(msg tea.KeyMsg, svc *service.ProfileService) (State, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyCtrlR:
		if s.filterEngine == FilterRegex {
			s.filterEngine = FilterSubstring
		} else {
			s.filterEngine = FilterRegex
		}
		s.updateFiltered()
		s.selected = 0
		s.scrollTop = 0
	case msg.Type == tea.KeyCtrlF:
		if s.filterEngine == FilterFuzzy {
			s.filterEngine = FilterSubstring
		} else {
			s.filterEngine = FilterFuzzy
		}
		s.updateFiltered()
		s.selected = 0
		s.scrollTop = 0
	case key.Matches(msg, common.Keys.Escape):
		s.filterMode = false
	case key.Matches(msg, common.Keys.Enter):
		s.filterMode = false
		s.selected = 0
		s.scrollTop = 0
	case key.Matches(msg, common.Keys.Backspace):
		if len(s.filter) > 0 {
			r := []rune(s.filter)
			s.filter = string(r[:len(r)-1])
			s.updateFiltered()
			s.selected = 0
			s.scrollTop = 0
		}
	default:
		input := msg.String()
		r := []rune(input)
		if len(r) == 1 && r[0] >= 32 {
			s.filter += input
			s.updateFiltered()
			s.selected = 0
			s.scrollTop = 0
		}
	}
	return s, nil
}

// handleDeleteConfirm 删除确认弹窗。
func (s State) handleDeleteConfirm(msg tea.KeyMsg, svc *service.ProfileService) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Enter), msg.String() == "y":
		uid := s.deleteUID
		s.showDeleteConf = false
		s.deleteUID = ""
		return s, DeleteSubCmd(svc, uid)
	case key.Matches(msg, common.Keys.Escape), msg.String() == "n":
		s.showDeleteConf = false
		s.deleteUID = ""
	}
	return s, nil
}

// openDeleteConfirm 打开当前选中订阅的删除确认。
func (s State) openDeleteConfirm() (State, tea.Cmd) {
	if len(s.filteredIdx) == 0 || s.selected < 0 || s.selected >= len(s.filteredIdx) {
		return s, nil
	}
	uid := s.subs[s.filteredIdx[s.selected]].UID
	s.showDeleteConf = true
	s.deleteUID = uid
	return s, nil
}

// activateSelected 激活当前选中订阅。
func (s State) activateSelected(svc *service.ProfileService) (State, tea.Cmd) {
	if len(s.filteredIdx) == 0 || s.selected < 0 || s.selected >= len(s.filteredIdx) {
		return s, nil
	}
	uid := s.subs[s.filteredIdx[s.selected]].UID
	return s, ActivateSubCmd(svc, uid)
}

// updateSelected 拉取当前选中订阅。
func (s State) updateSelected(svc *service.ProfileService) (State, tea.Cmd) {
	if len(s.filteredIdx) == 0 || s.selected < 0 || s.selected >= len(s.filteredIdx) {
		return s, nil
	}
	uid := s.subs[s.filteredIdx[s.selected]].UID
	s.updatingUID = uid
	return s, UpdateSubCmd(svc, uid)
}

// SetUpdating 标记指定的订阅为正在更新。
func (s State) SetUpdating(uid string) State {
	s.updatingUID = uid
	return s
}

// ClearUpdating 清除正在更新的订阅标记。
func (s State) ClearUpdating() State {
	s.updatingUID = ""
	return s
}

// openMergeEditor 打开当前选中订阅的 merge 编辑器（先异步载入内容）。
func (s State) openMergeEditor(svc *service.ProfileService) (State, tea.Cmd) {
	if len(s.filteredIdx) == 0 || s.selected < 0 || s.selected >= len(s.filteredIdx) {
		return s, nil
	}
	uid := s.subs[s.filteredIdx[s.selected]].UID
	s.showMergeEditor = true
	s.mergeEditor = newMergeEditor(uid)
	s.mergeLoadingUID = uid
	return s, LoadMergeCmd(svc, uid)
}

// HandleMergeLoaded 处理 merge 内容加载结果（由主 Model 收到 MergeLoadedMsg 后回调）。
func (s State) HandleMergeLoaded(uid string, data []byte, err error) State {
	// 仅当当前正在加载该 UID 且编辑器确实为其打开时才回填，
	// 避免编辑器未初始化时操作 textarea（viewport nil panic）。
	if s.mergeLoadingUID != uid || s.mergeEditor.uid != uid {
		return s
	}
	editor := s.mergeEditor
	if err != nil {
		editor.errMsg = i18n.Tf("sub.merge_load_err", err.Error())
		editor.ready = true // 允许编辑（从空开始）
	} else {
		editor = editor.applyLoaded(data)
	}
	s.mergeEditor = editor
	return s
}

// HandleMouseScroll 鼠标滚轮。
func (s State) HandleMouseScroll(up bool) State {
	// merge 编辑器打开时滚轮滚文本
	if s.showMergeEditor && s.mergeEditor.ready {
		editor := s.mergeEditor
		// bubbles/textarea 的滚轮通过 Viewport 渲染处理，此处不干预
		s.mergeEditor = editor
		return s
	}
	if up {
		if s.selected > 0 {
			s.selected--
			s.clampScroll()
		}
	} else {
		if s.selected < len(s.filteredIdx)-1 {
			s.selected++
			s.clampScroll()
		}
	}
	return s
}

// HandleMouseLeft 处理列表单击/双击。
func (s State) HandleMouseLeft(pageX, pageY, pageWidth, pageHeight int, svc *service.ProfileService) (State, tea.Cmd) {
	// 弹窗激活时点击弹窗外 → 取消关闭（删除确认/merge 编辑器/添加表单）
	if s.showDeleteConf || s.showMergeEditor || s.showAddForm {
		// 简化处理：点击任意位置不自动关闭破坏性弹窗（需 Esc/Enter），
		// 避免误触。仅非破坏性的添加表单点击外部关闭。
		if s.showAddForm && !s.showDeleteConf && !s.showMergeEditor {
			ps := s.ToPageState(pageWidth, pageHeight)
			left, top, right, bottom := resolveAddFormBounds(ps, pageWidth, pageHeight)
			if !(pageX >= left && pageX < right && pageY >= top && pageY < bottom) {
				s.showAddForm = false
				s.addForm = newAddForm()
			}
		}
		return s, nil
	}

	// 列表命中
	idx := resolveListItemAt(s.ToPageState(pageWidth, pageHeight), pageY, pageHeight)
	if idx < 0 {
		return s, nil
	}

	now := time.Now()
	isDouble := idx == s.lastMouseIdx &&
		!s.lastMouseAt.IsZero() &&
		now.Sub(s.lastMouseAt) <= subDoubleClickThreshold
	s.lastMouseIdx = idx
	s.lastMouseAt = now

	s.selected = idx
	s.clampScroll()

	if isDouble {
		// 双击激活
		return s.activateSelected(svc)
	}
	return s, nil
}

// clampScroll 确保选中项可见。
func (s *State) clampScroll() {
	maxLines := subVisibleLinesHint
	if s.selected < s.scrollTop {
		s.scrollTop = s.selected
	}
	if s.selected >= s.scrollTop+maxLines {
		s.scrollTop = s.selected - maxLines + 1
	}
}

// updateFiltered 重建过滤缓存。
func (s *State) updateFiltered() {
	s.filteredIdx = s.filteredIdx[:0]
	if len(s.subs) == 0 {
		return
	}
	if s.filter == "" {
		for i := range s.subs {
			s.filteredIdx = append(s.filteredIdx, i)
		}
		return
	}

	var re *regexp.Regexp
	var keywords []string
	switch s.filterEngine {
	case FilterRegex:
		var err error
		re, err = regexp.Compile("(?i)" + s.filter)
		if err != nil {
			re = nil
		}
	case FilterSubstring:
		keywords = strings.Fields(strings.ToLower(s.filter))
	}

	for i, p := range s.subs {
		hay := p.Name + " " + p.Source.Display() + " " + p.Source.Kind.String()
		switch s.filterEngine {
		case FilterRegex:
			if re == nil || !re.MatchString(hay) {
				continue
			}
		case FilterFuzzy:
			if !subFuzzy(s.filter, hay) {
				continue
			}
		default:
			lower := strings.ToLower(hay)
			all := true
			for _, kw := range keywords {
				if !strings.Contains(lower, kw) {
					all = false
					break
				}
			}
			if !all {
				continue
			}
		}
		s.filteredIdx = append(s.filteredIdx, i)
	}
}

// subFuzzy 有序子序列匹配（与 rules.fuzzyMatch 一致）。
func subFuzzy(pattern, text string) bool {
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
