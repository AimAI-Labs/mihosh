package sub

// state.go — 订阅页完整状态与按键/鼠标分发。
//
// 交互模式（互斥）：普通列表 / 搜索输入 / 添加表单 / 删除确认。
// 弹窗激活时吞掉底层按键（与 rules 页一致）。
// 「编辑覆写」改为外部编辑器（按 m），不在 TUI 内维护弹窗状态。

import (
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// 交互模式常量（供 help 包镜像，见 help/view.go subMode*）。
const (
	ModeNormal     = 0
	ModeSearch     = 1
	ModeAddForm    = 2
	ModeDeleteConf = 4
)

// State 订阅页完整状态。
type State struct {
	subs         []profile.Profile
	filteredIdx  []int // 过滤后的订阅在 subs 中的索引
	activeUID    string
	filter       string
	filterMode   bool
	filterEngine FilterEngine
	selected     int
	scrollTop    int
	listMode     bool
	pageHeight   int
	pageWidth    int

	// 弹窗/输入模式
	showAddForm    bool
	addForm        *addForm
	showEditForm   bool // 编辑当前选中订阅（名称 + 来源）
	editUID        string
	editForm       *addForm
	showDeleteConf bool
	deleteUID      string // 待删除订阅 UID
	file           fileView
	history        historyView

	// 鼠标双击
	doubleClickDetector common.DoubleClickDetector[struct{}]

	updatingUID   string // 当前正在更新的订阅 UID
	batchUpdating bool   // 当前是否正在批量更新远程订阅

	svc *service.ProfileService
}

// NewState 初始化订阅配置状态
func NewState(svc *service.ProfileService) State {
	return State{
		svc: svc,
	}
}

// FilterEngine 订阅搜索匹配引擎（与 rules 页一致语义）。
type FilterEngine = common.SearchEngine

const (
	FilterSubstring = common.SearchSubstring
	FilterRegex     = common.SearchRegex
	FilterFuzzy     = common.SearchFuzzy
)

// ToPageState 转换为渲染层所需的 PageState。
func (s State) ToPageState(width, height int) PageState {
	var addFormVal, editFormVal addForm
	if s.addForm != nil {
		addFormVal = *s.addForm
	}
	if s.editForm != nil {
		editFormVal = *s.editForm
	}
	return PageState{
		Subs:           s.subs,
		FilteredIdx:    s.filteredIdx,
		ActiveUID:      s.activeUID,
		FilterText:     s.filter,
		FilterMode:     s.filterMode,
		FilterEngine:   s.filterEngine,
		Selected:       s.selected,
		ScrollTop:      s.scrollTop,
		ListMode:       s.listMode,
		Width:          width,
		Height:         height,
		ShowAddForm:    s.showAddForm,
		AddForm:        addFormVal,
		ShowEditForm:   s.showEditForm,
		EditForm:       editFormVal,
		ShowDeleteConf: s.showDeleteConf,
		DeleteUID:      s.deleteUID,
		UpdatingUID:    s.updatingUID,
		File:           s.file,
		History:        s.history,
	}
}

// ActiveUID 返回当前激活订阅的 UID（供 update.go 区分编辑的是否为激活订阅）。
func (s State) ActiveUID() string { return s.activeUID }

// Mode 返回当前交互模式（供 help 弹窗）。
func (s State) Mode() int {
	switch {
	case s.filterMode:
		return ModeSearch
	case s.showAddForm, s.showEditForm:
		return ModeAddForm
	case s.showDeleteConf:
		return ModeDeleteConf
	default:
		return ModeNormal
	}
}

// Querying 是否处于输入捕获模式（供主 Model 的 isInputCapturing 判断）。
func (s State) Querying() bool {
	return s.filterMode || s.showAddForm || s.showEditForm || s.showDeleteConf || s.file.mode != fileClosed || s.history.mode != historyClosed
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
func (s State) Update(msg tea.Msg) (State, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.SubFileLoadedMsg:
		if s.file.loading && s.file.request == msg.Request {
			s.file.loading = false
			s.file.diff, s.file.err = msg.Diff, msg.Err
		}
		return s, nil
	case messages.SubFileViewerClosedMsg:
		if s.file.mode == fileMenu && s.file.request == msg.Request {
			s.file.err = msg.Err
		}
		return s, nil
	case messages.SubHistoryLoadedMsg:
		if s.history.mode != historyClosed && s.history.request == msg.Request {
			s.history.loading = false
			s.history.snapshots = msg.Snapshots
			s.history.err = msg.Err
			if s.history.selected >= len(s.history.snapshots) {
				s.history.selected = max(0, len(s.history.snapshots)-1)
			}
		}
		return s, nil
	case messages.SubHistoryDiffMsg:
		if s.history.mode == historyDiff && s.history.request == msg.Request {
			s.history.loading = false
			s.history.diff = msg.Rows
			s.history.err = msg.Err
		}
		return s, nil
	case messages.PageResizeMsg:
		s.pageWidth = msg.Width
		s.pageHeight = msg.Height
		if s.showAddForm && s.addForm != nil {
			f := *s.addForm
			f.resize(msg.Width, msg.Height)
			s.addForm = &f
		}
		if s.showEditForm && s.editForm != nil {
			f := *s.editForm
			f.resize(msg.Width, msg.Height)
			s.editForm = &f
		}
		s.clampScroll()
		return s, nil
	case messages.PageMouseScrollMsg:
		s.pageWidth, s.pageHeight = msg.Width, msg.Height
		if s.history.mode != historyClosed {
			if s.history.mode == historyList {
				if msg.Up && s.history.selected > 0 {
					s.history.selected--
				} else if !msg.Up && s.history.selected < len(s.history.snapshots)-1 {
					s.history.selected++
				}
			} else if msg.Up && s.history.top > 0 {
				s.history.top--
			} else if !msg.Up {
				s.history.top++
			}
			return s, nil
		}
		if s.file.mode != fileClosed {
			delta := 1
			if msg.Up {
				delta = -1
			}
			s.file.scroll(delta, msg.Width, msg.Height)
			return s, nil
		}
		if s.showAddForm || s.showEditForm {
			return s.scrollForm(msg)
		}
		return s.HandleMouseScroll(msg.Up), nil
	case messages.PageMouseClickMsg:
		if s.history.mode != historyClosed {
			return s.clickHistory(msg)
		}
		if s.file.mode != fileClosed {
			return s.clickFile(msg)
		}
		return s.HandleMouseLeft(msg.X, msg.Y, msg.Width, msg.Height)
	case tea.KeyMsg:
		// 弹窗优先拦截
		if s.history.mode != historyClosed {
			return s.handleHistoryKey(msg)
		}
		if s.file.mode != fileClosed {
			return s.handleFileKey(msg)
		}
		if s.showDeleteConf {
			return s.handleDeleteConfirm(msg)
		}
		if s.showAddForm {
			return s.handleAddFormUpdate(msg)
		}
		if s.showEditForm {
			return s.handleEditFormUpdate(msg)
		}
		if s.filterMode {
			return s.handleFilterMode(msg)
		}

		columns := s.layoutColumns()
		switch {
		case key.Matches(msg, common.Keys.Up):
			s.moveSelection(-columns)
		case key.Matches(msg, common.Keys.Down):
			s.moveSelection(columns)
		case columns > 1 && key.Matches(msg, common.Keys.Left):
			if s.selected%columns > 0 {
				s.moveSelection(-1)
			}
		case columns > 1 && key.Matches(msg, common.Keys.Right):
			if s.selected%columns < columns-1 {
				s.moveSelection(1)
			}
		case msg.String() == "v":
			s.setLayout(!s.listMode)
		case msg.String() == "enter":
			return s.activateSelected()
		case msg.String() == "u":
			return s.updateSelected()
		case msg.String() == "U":
			return s.updateAllRemote()
		case msg.String() == "m":
			return s.openMergeExternalEditor()
		case msg.String() == "c":
			return s.openFileMenu(), nil
		case msg.String() == "H":
			return s.openHistoryModal()
		case msg.String() == "e":
			return s.openEditForm()
		case msg.String() == "n":
			s.showAddForm = true
			f := newAddForm()
			f.resize(s.pageWidth, s.pageHeight)
			s.addForm = &f
		case msg.String() == "d":
			return s.openDeleteConfirm()
		case msg.String() == "/":
			s.filterMode = true
			s.filter = ""
		case key.Matches(msg, common.Keys.Refresh):
			return s, RefreshSubs(s.svc)
		case key.Matches(msg, common.Keys.Escape):
			if s.filter != "" {
				s.filter = ""
				s.selected = 0
				s.scrollTop = 0
				s.updateFiltered()
			}
		}
	}
	return s, nil
}

// handleFilterMode 搜索输入模式。
func (s State) handleFilterMode(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Escape):
		s.filterMode = false
	case key.Matches(msg, common.Keys.Enter):
		s.filterMode = false
		s.selected = 0
		s.scrollTop = 0
	default:
		query := common.SearchQuery{Text: s.filter, Engine: s.filterEngine}
		if query.Edit(msg) {
			s.filter, s.filterEngine = query.Text, query.Engine
			s.updateFiltered()
			s.selected = 0
			s.scrollTop = 0
		}
	}
	return s, nil
}

// handleDeleteConfirm 删除确认弹窗。
func (s State) handleDeleteConfirm(msg tea.KeyMsg) (State, tea.Cmd) {
	switch {
	case key.Matches(msg, common.Keys.Enter), msg.String() == "y":
		uid := s.deleteUID
		s.showDeleteConf = false
		s.deleteUID = ""
		return s, DeleteSubCmd(s.svc, uid)
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
func (s State) activateSelected() (State, tea.Cmd) {
	if len(s.filteredIdx) == 0 || s.selected < 0 || s.selected >= len(s.filteredIdx) {
		return s, nil
	}
	uid := s.subs[s.filteredIdx[s.selected]].UID
	return s, ActivateSubCmd(s.svc, uid)
}

// updateSelected 拉取当前选中订阅。
func (s State) updateSelected() (State, tea.Cmd) {
	if s.updatingUID != "" || s.batchUpdating || len(s.filteredIdx) == 0 || s.selected < 0 || s.selected >= len(s.filteredIdx) {
		return s, nil
	}
	uid := s.subs[s.filteredIdx[s.selected]].UID
	s.updatingUID = uid
	return s, UpdateSubCmd(s.svc, uid)
}

// updateAllRemote 一键批量更新全部远程订阅。
func (s State) updateAllRemote() (State, tea.Cmd) {
	if s.updatingUID != "" || s.batchUpdating {
		return s, nil
	}
	s.batchUpdating = true
	return s, UpdateAllRemoteSubsCmd(s.svc)
}

// BatchUpdating 返回是否正在执行批量/自动订阅更新。
func (s State) BatchUpdating() bool { return s.batchUpdating }

// SetBatchUpdating 标记正在批量更新远程订阅。
func (s State) SetBatchUpdating() State {
	s.batchUpdating = true
	return s
}

// ClearBatchUpdating 清除批量更新标记。
func (s State) ClearBatchUpdating() State {
	s.batchUpdating = false
	return s
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

// openEditForm 打开当前选中订阅的编辑表单（预填名称 + 来源）。
func (s State) openEditForm() (State, tea.Cmd) {
	if len(s.filteredIdx) == 0 || s.selected < 0 || s.selected >= len(s.filteredIdx) {
		return s, nil
	}
	p := s.subs[s.filteredIdx[s.selected]]
	form := newAddForm()
	form.nameField.SetValue(p.Name)
	form.srcField.SetValue(p.Source.Display())
	form.kindRemote = p.Source.Kind == profile.SourceRemote
	form.useProxy = p.Source.UseProxy
	form.uaField.SetValue(p.Source.UserAgent)
	form.skipTLS = p.Source.InsecureSkipVerify
	form.updateInterval = p.Source.UpdateInterval
	form.copySource = sourceClipboardAvailable()
	form.resize(s.pageWidth, s.pageHeight)
	// 焦点重新落到名称字段（SetValue 后 textinput 仍处于 Focus 状态）
	form.focusCurrent()
	s.showEditForm = true
	s.editUID = p.UID
	s.editForm = &form
	return s, nil
}

// handleEditFormUpdate 处理编辑表单按键（与添加表单逻辑共用）。
func (s State) handleEditFormUpdate(msg tea.KeyMsg) (State, tea.Cmd) {
	form := newAddForm()
	if s.editForm != nil {
		form = *s.editForm
	}
	next, submit, cmd, closed := updateFormFields(msg, form)
	if closed {
		// Esc：关闭并清空
		s.showEditForm = false
		s.editUID = ""
		s.editForm = nil
		return s, nil
	}
	if submit {
		name := next.nameField.Value()
		src := next.buildSource()
		uid := s.editUID
		s.showEditForm = false
		s.editUID = ""
		s.editForm = nil
		return s, EditSubCmd(s.svc, uid, name, src)
	}
	s.editForm = &next
	return s, cmd
}

// HandleMouseScroll 鼠标滚轮。
func (s State) HandleMouseScroll(up bool) State {
	if s.Querying() {
		return s
	}
	step := s.layoutColumns()
	if up {
		step = -step
	}
	s.moveSelection(step)
	return s
}

// HandleMouseLeft 处理列表单击/双击。
func (s State) HandleMouseLeft(pageX, pageY, pageWidth, pageHeight int) (State, tea.Cmd) {
	s.pageWidth, s.pageHeight = pageWidth, pageHeight
	s.clampScroll()
	// 弹窗激活时点击弹窗外 → 取消关闭（删除确认/添加表单）
	if s.showDeleteConf || s.showAddForm || s.showEditForm {
		// 简化处理：点击任意位置不自动关闭破坏性弹窗（需 Esc/Enter），
		// 避免误触。仅非破坏性的添加/编辑表单点击外部关闭。
		if (s.showAddForm || s.showEditForm) && !s.showDeleteConf {
			ps := s.ToPageState(pageWidth, pageHeight)
			left, top, right, bottom := resolveFormBounds(ps, pageWidth, pageHeight)
			if pageX < left || pageX >= right || pageY < top || pageY >= bottom {
				s.showAddForm = false
				s.addForm = nil
				s.showEditForm = false
				s.editUID = ""
				s.editForm = nil
			} else {
				return s.clickForm(pageX-left-2, pageY-top-1, pageWidth, pageHeight)
			}
		}
		return s, nil
	}

	// 列表命中
	if s.filterMode {
		return s, nil
	}
	if list, hit := subLayoutHit(pageX, pageY, pageWidth); hit {
		s.setLayout(list)
		return s, nil
	}
	grid := layoutSubList(s.ToPageState(pageWidth, pageHeight))
	y := pageY - subFixedLines
	idx := grid.ItemAt(pageX, y)
	if idx < 0 {
		return s, nil
	}

	originX, originY, cardWidth := grid.CardOrigin(idx)
	if grid.CardHeight >= 3 && y == originY+1 && pageX == originX+cardWidth-3 &&
		s.subs[s.filteredIdx[idx]].Source.Kind == profile.SourceRemote {
		s.selected = idx
		return s.updateSelected()
	}

	now := time.Now()
	isDouble := s.doubleClickDetector.IsDoubleClick(struct{}{}, idx, now)

	s.selected = idx
	s.clampScroll()

	if isDouble {
		// 双击激活
		return s.activateSelected()
	}
	return s, nil
}

func (s *State) moveSelection(delta int) {
	s.selected += delta
	s.clampScroll()
}

func (s State) layoutColumns() int {
	if s.listMode {
		return 1
	}
	return subCardColumns(s.pageWidth)
}

func (s *State) setLayout(list bool) {
	if s.listMode != list {
		s.listMode = list
		s.doubleClickDetector = common.DoubleClickDetector[struct{}]{}
		s.clampScroll()
	}
}

// clampScroll 确保选中项可见。
func (s *State) clampScroll() {
	window := layoutSubList(s.ToPageState(s.pageWidth, s.pageHeight)).Window
	s.selected, s.scrollTop = window.Selected, window.Start
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

	match := (common.SearchQuery{Text: s.filter, Engine: s.filterEngine}).Matcher(common.MatchAllKeywords)
	for i, p := range s.subs {
		if match(p.Name + " " + p.Source.Display() + " " + p.Source.Kind.String()) {
			s.filteredIdx = append(s.filteredIdx, i)
		}
	}
}

// InputCapture declares which input belongs to the current page mode.
func (s State) InputCapture() common.InputCapture {
	return common.InputCapture{Active: s.Querying(), Interrupt: s.Querying()}
}
