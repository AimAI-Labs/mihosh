package sub

// raw_editor.go — 在外部编辑器（$EDITOR / notepad）中查看/编辑具体的订阅原始文件。

import (
	"errors"
	"os"
	"os/exec"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	tea "github.com/charmbracelet/bubbletea"
)

// openRawExternalEditor 在外部编辑器中以“安全沙盒模式”打开指定的订阅 raw.yaml 文件。
func (s State) openRawExternalEditor(svc *service.ProfileService) (State, tea.Cmd) {
	if len(s.filteredIdx) == 0 || s.selected < 0 || s.selected >= len(s.filteredIdx) {
		return s, nil
	}
	uid := s.subs[s.filteredIdx[s.selected]].UID

	rawPath, err := profile.RawPath(uid)
	if err != nil {
		return s, rawEditError(uid, err)
	}

	// 预检文件是否存在（未拉取过的远程订阅不存在）
	data, err := os.ReadFile(rawPath)
	if err != nil {
		if os.IsNotExist(err) {
			return s, rawEditError(uid, errors.New(i18n.T("sub.err.raw_not_found")))
		}
		return s, rawEditError(uid, err)
	}

	editor := utils.ResolveEditor()
	if editor == "" {
		return s, rawEditError(uid, errors.New("未找到可用的编辑器，请在 ~/.bashrc 或 ~/.zshrc 中设置 EDITOR 变量"))
	}

	// 创建临时副本作纯只读查看，防弹保护原始文件
	tempFile, err := os.CreateTemp("", "mihosh-raw-*-view.yaml")
	if err != nil {
		return s, rawEditError(uid, err)
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return s, rawEditError(uid, err)
	}
	tempFile.Close()

	fields := utils.SplitEditorCommand(editor)
	args := make([]string, 0, len(fields))
	args = append(args, fields[1:]...)
	args = append(args, tempPath)
	c := exec.Command(fields[0], args...)

	return s, tea.ExecProcess(c, func(err error) tea.Msg {
		// 编辑结束后直接清理临时文件（绝对不回写）
		os.Remove(tempPath)
		return messages.SubRawEditFinishedMsg{UID: uid, Err: err}
	})
}

// rawEditError 构造一个返回 SubRawEditFinishedMsg（携带错误）的命令。
func rawEditError(uid string, err error) tea.Cmd {
	return func() tea.Msg {
		return messages.SubRawEditFinishedMsg{UID: uid, Err: err}
	}
}
