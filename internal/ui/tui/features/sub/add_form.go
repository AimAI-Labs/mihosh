package sub

// add_form.go — 添加订阅表单状态。
//
// 字段（Tab/方向键切换）：
//   0 名称（textinput）
//   1 来源类型（remote↔local 切换，只读选择器）
//   2 来源值（textinput：URL 或 Path）
//
// 提交时来源值自动判断类型（http(s):// → remote，其余 → local）。

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	addFieldName = 0
	addFieldKind = 1
	addFieldSrc  = 2
	addFieldCnt  = 3
)

// addForm 添加订阅表单。
type addForm struct {
	fields      []textinput.Model
	fieldCursor int
	kindRemote  bool // true=remote, false=local
	errMsg      string
}

// newAddForm 构造初始空表单，焦点在名称字段。
func newAddForm() addForm {
	name := textinput.New()
	name.Prompt = ""
	name.Placeholder = i18n.T("sub.add_placeholder_name")
	name.Focus()

	src := textinput.New()
	src.Prompt = ""
	src.Placeholder = i18n.T("sub.add_placeholder_src")

	// kind 槽位保留占位 textinput（不参与输入，仅维持索引与布局高度）
	kind := textinput.New()
	kind.Prompt = ""

	return addForm{
		fields:      []textinput.Model{name, kind, src},
		fieldCursor: addFieldName,
		kindRemote:  true,
	}
}

func (f addForm) currentKind() profile.SourceKind {
	if f.kindRemote {
		return profile.SourceRemote
	}
	return profile.SourceLocal
}

func (f addForm) isNameField() bool   { return f.fieldCursor == addFieldName }
func (f addForm) isKindField() bool   { return f.fieldCursor == addFieldKind }
func (f addForm) isSrcField() bool    { return f.fieldCursor == addFieldSrc }

// cycleField 按方向循环切换字段。
func (f *addForm) cycleField(dir int) {
	n := addFieldCnt
	f.fieldCursor = (f.fieldCursor + dir%n + n) % n
	f.focusCurrent()
}

// toggleKind 切换来源类型（仅在 kind 字段聚焦时有效）。
func (f *addForm) toggleKind() {
	f.kindRemote = !f.kindRemote
	// 切换类型时更新占位符提示
	src := f.fields[addFieldSrc]
	if f.kindRemote {
		src.Placeholder = i18n.T("sub.add_placeholder_src_remote")
	} else {
		src.Placeholder = i18n.T("sub.add_placeholder_src_local")
	}
	f.fields[addFieldSrc] = src
}

func (f *addForm) focusCurrent() {
	for i := range f.fields {
		if i == f.fieldCursor && (i == addFieldName || i == addFieldSrc) {
			f.fields[i].Focus()
		} else {
			f.fields[i].Blur()
		}
	}
}

// validate 校验表单：名称非空、来源值非空。
// 返回 (ok, errKey)；errKey 为 i18n 键。
func (f addForm) validate() (bool, string) {
	name := f.fields[addFieldName].Value()
	src := f.fields[addFieldSrc].Value()
	if name == "" {
		return false, "sub.add_err_name"
	}
	if src == "" {
		return false, "sub.add_err_src"
	}
	return true, ""
}

// buildSource 根据当前输入构造 SubSource（自动判断类型）。
func (f addForm) buildSource() profile.SubSource {
	val := f.fields[addFieldSrc].Value()
	kind := profile.ParseSourceKind(val) // 自动判断，覆盖手动切换
	src := profile.SubSource{Kind: kind}
	if kind == profile.SourceRemote {
		src.URL = val
	} else {
		src.Path = val
	}
	return src
}

// handleUpdate 处理表单内的按键（弹窗激活时由 state 分派到此）。
func (s State) handleAddFormUpdate(msg tea.KeyMsg, svc *service.ProfileService) (State, tea.Cmd) {
	form := s.addForm
	next, submit, cmd, closed := updateFormFields(msg, form)
	if closed {
		// Esc：关闭并清空表单
		s.showAddForm = false
		s.addForm = newAddForm()
		return s, nil
	}
	if submit {
		// Enter 校验通过：调用方提交
		name := next.fields[addFieldName].Value()
		src := next.buildSource()
		s.showAddForm = false
		s.addForm = newAddForm()
		return s, AddSubCmd(svc, name, src)
	}
	s.addForm = next
	return s, cmd
}

// updateFormFields 表单按键的共用逻辑（添加/编辑表单共享）。
//
// 返回：
//   - next：处理后的表单状态；
//   - submit：是否触发了 Enter 且校验通过（调用方据此决定提交命令）；
//   - cmd：文本输入产生的命令（无则 nil）；
//   - closed：是否按 Esc 取消（调用方据此关闭弹窗）。
func updateFormFields(msg tea.KeyMsg, form addForm) (next addForm, submit bool, cmd tea.Cmd, closed bool) {
	switch {
	case msg.String() == "esc":
		return form, false, nil, true

	case msg.String() == "enter":
		ok, errKey := form.validate()
		if !ok {
			form.errMsg = i18n.T(errKey)
			return form, false, nil, false
		}
		return form, true, nil, false

	case msg.String() == "tab":
		form.cycleField(1)
	case msg.String() == "shift+tab":
		form.cycleField(-1)

	case msg.String() == "up":
		form.cycleField(-1)
	case msg.String() == "down":
		form.cycleField(1)

	case msg.String() == "left":
		if form.isKindField() {
			form.toggleKind()
		}
	case msg.String() == "right":
		if form.isKindField() {
			form.toggleKind()
		}

	default:
		// 仅名称与来源值字段接收文本输入
		if form.isKindField() {
			return form, false, nil, false
		}
		cur := form.fields[form.fieldCursor]
		updated, c := cur.Update(msg)
		form.fields[form.fieldCursor] = updated
		form.errMsg = ""
		return form, false, c, false
	}
	return form, false, nil, false
}
