package rules

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	tea "github.com/charmbracelet/bubbletea"
)

// openConfigEditor 在外部编辑器中打开当前 Mihomo 配置文件。
// 通过 tea.ExecProcess 暂停 TUI 并把终端交给编辑器，退出后恢复。
// 配置路径缺失或无可用编辑器时返回 ConfigEditFinishedMsg（携带错误）。
func (s State) openConfigEditor() (State, tea.Cmd) {
	if strings.TrimSpace(s.configPath) == "" {
		return s, configEditError(errors.New("未找到 mihomo 配置文件路径，无法打开编辑器"))
	}
	editor := utils.ResolveEditor()
	if editor == "" {
		return s, configEditError(errors.New("未找到可用的编辑器，请在 ~/.bashrc 或 ~/.zshrc 中设置 EDITOR 变量"))
	}
	fields := utils.SplitEditorCommand(editor)
	args := make([]string, 0, len(fields))
	args = append(args, fields[1:]...)
	args = append(args, s.configPath)
	c := exec.Command(fields[0], args...)
	return s, tea.ExecProcess(c, func(err error) tea.Msg {
		return messages.ConfigEditFinishedMsg{Err: err}
	})
}

// configEditError 构造一个返回 ConfigEditFinishedMsg（携带错误）的命令。
func configEditError(err error) tea.Cmd {
	return func() tea.Msg {
		return messages.ConfigEditFinishedMsg{Err: err}
	}
}
