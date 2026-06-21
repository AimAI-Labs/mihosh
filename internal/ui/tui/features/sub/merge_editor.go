package sub

// merge_editor.go — 在外部编辑器（$EDITOR / notepad）中打开 merge.yaml。
//
// 与 rules 页编辑 mihomo 配置一致：通过 tea.ExecProcess 暂停 TUI，
// 把终端交给编辑器，编辑器直接写盘 merge.yaml；退出后由主 Model 负责重载核心。
// 删除了原 bubbles/textarea 弹窗与 Load/Save 来回，磁盘作为单一事实源。

import (
	"errors"
	"os/exec"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	tea "github.com/charmbracelet/bubbletea"
)

// openMergeExternalEditor 在外部编辑器中打开当前选中订阅的 merge.yaml。
//
// 空列表/无有效选中项时返回 nil cmd；无可用编辑器或路径解析失败时
// 返回携带 MergeEditFinishedMsg{Err} 的命令。
func (s State) openMergeExternalEditor(svc *service.ProfileService) (State, tea.Cmd) {
	if len(s.filteredIdx) == 0 || s.selected < 0 || s.selected >= len(s.filteredIdx) {
		return s, nil
	}
	uid := s.subs[s.filteredIdx[s.selected]].UID

	mergePath, err := profile.MergePath(uid)
	if err != nil {
		return s, mergeEditError(uid, err)
	}
	editor := utils.ResolveEditor()
	if editor == "" {
		return s, mergeEditError(uid, errors.New("未找到可用的编辑器，请在 ~/.bashrc 或 ~/.zshrc 中设置 EDITOR 变量"))
	}
	fields := utils.SplitEditorCommand(editor)
	args := make([]string, 0, len(fields))
	args = append(args, fields[1:]...)
	args = append(args, mergePath)
	c := exec.Command(fields[0], args...)
	return s, tea.ExecProcess(c, func(err error) tea.Msg {
		return messages.MergeEditFinishedMsg{UID: uid, Err: err}
	})
}

// mergeEditError 构造一个返回 MergeEditFinishedMsg（携带错误）的命令。
func mergeEditError(uid string, err error) tea.Cmd {
	return func() tea.Msg {
		return messages.MergeEditFinishedMsg{UID: uid, Err: err}
	}
}
