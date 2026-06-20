package sub

// merge_editor.go — merge.yaml 多行编辑器（基于 bubbles/textarea）。
//
// 打开时异步载入当前订阅的 merge.yaml 内容（LoadMergeCmd → MergeLoadedMsg）。
//   - Esc：取消关闭，不保存；
//   - Ctrl+S：保存并关闭（SaveMergeCmd）。
//
// 内容加载后先存入 pending（延迟赋值）。bubbles/textarea 的 SetValue 依赖
// viewport 初始化（viewport 仅在 SetWidth/SetHeight/View 后可用），过早调用
// 会触发 nil panic。因此在弹窗渲染（已知宽度）时通过 flushPending 把内容
// 真正写入 textarea。

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// mergeEditor merge 编辑器状态。
type mergeEditor struct {
	editor  textarea.Model
	uid     string
	ready   bool   // 内容是否已加载
	errMsg  string // 保存/加载错误（行内显示）
	pending string // 待写入 textarea 的内容（渲染时 flush，避免 viewport nil panic）
	flushed bool   // pending 是否已写入
}

// newMergeEditor 构造编辑器（内容待 LoadMerge 回填）。
func newMergeEditor(uid string) mergeEditor {
	ta := textarea.New()
	ta.Placeholder = i18n.T("sub.merge_placeholder")
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.SetHeight(10)
	ta.Focus()
	return mergeEditor{uid: uid, editor: ta}
}

// applyLoaded 内容加载完成后暂存到 pending（渲染时才 flush 到 textarea）。
func (m mergeEditor) applyLoaded(data []byte) mergeEditor {
	m.pending = string(data)
	m.ready = true
	m.flushed = false
	return m
}

// flushPending 在 textarea 已设置尺寸（渲染期）后，把 pending 内容写入。
// 重复调用安全（flushed 标记避免重复赋值）。
func (m mergeEditor) flushPending() mergeEditor {
	if m.flushed || !m.ready {
		return m
	}
	if m.pending != "" {
		m.editor.SetValue(m.pending)
	}
	m.flushed = true
	return m
}

// handleUpdate 处理编辑器按键（弹窗激活时由 state 分派到此）。
func (s State) handleMergeEditorUpdate(msg tea.KeyMsg, svc *service.ProfileService) (State, tea.Cmd) {
	editor := s.mergeEditor

	switch {
	case msg.String() == "esc":
		// 取消关闭
		s.showMergeEditor = false
		s.mergeEditor = newMergeEditor("")
		return s, nil

	case msg.String() == "ctrl+s":
		if !editor.ready {
			// 内容未加载，忽略保存
			return s, nil
		}
		// 优先用 pending（尚未 flush 的情况），否则用编辑器当前值
		content := editor.pending
		if editor.flushed {
			content = editor.editor.Value()
		}
		uid := editor.uid
		s.showMergeEditor = false
		s.mergeEditor = newMergeEditor("")
		return s, SaveMergeCmd(svc, uid, []byte(content))

	default:
		if !editor.ready {
			// 未就绪：吞掉除 Esc 外的所有键
			return s, nil
		}
		editor = editor.flushPending()
		updated, cmd := editor.editor.Update(msg)
		editor.editor = updated
		s.mergeEditor = editor
		return s, cmd
	}
}
