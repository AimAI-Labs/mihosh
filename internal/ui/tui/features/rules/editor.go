package rules

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

// editorRCRe 从 shell 启动文件中匹配 EDITOR 赋值。
// 支持 export 前缀与单/双引号；未加引号时行内注释被截断。
// 多行匹配取最后一个有效值（与 shell 语义一致）。
var editorRCRe = regexp.MustCompile(`(?m)^\s*(?:export\s+)?EDITOR=(?:"([^"]*)"|'([^']*)'|([^\s#]+))`)

// resolveEditor 按优先级解析编辑器：
//  1. ~/.bashrc 中的 EDITOR
//  2. ~/.zshrc 中的 EDITOR
//  3. VISUAL / EDITOR 环境变量（通常由 shell rc 导出）
//  4. 默认编辑器（Unix: vim→vi→nano；Windows: notepad）
//
// 返回值为编辑器命令（可能含参数，如 "code --wait"）；全部失败时返回空串。
func resolveEditor() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		for _, name := range []string{".bashrc", ".zshrc"} {
			if v := parseEditorFromRCFile(filepath.Join(home, name)); v != "" {
				return v
			}
		}
	}
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	if v := os.Getenv("EDITOR"); v != "" {
		return v
	}
	return detectDefaultEditor()
}

// parseEditorFromRCFile 读取并解析单个 shell rc 文件中的 EDITOR 赋值。
func parseEditorFromRCFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return parseEditorFromRC(string(data))
}

// parseEditorFromRC 从 rc 文件内容中提取 EDITOR 的值（取最后一个有效赋值）。
func parseEditorFromRC(content string) string {
	var last string
	for _, m := range editorRCRe.FindAllStringSubmatch(content, -1) {
		var v string
		switch {
		case m[1] != "":
			v = m[1] // 双引号
		case m[2] != "":
			v = m[2] // 单引号
		default:
			v = m[3] // 未加引号
		}
		if v != "" {
			last = v
		}
	}
	return last
}

// detectDefaultEditor 在 PATH 中查找默认编辑器。
// TUI 场景下优先终端编辑器（阻塞式），避免 GUI 编辑器（如 code）立即返回导致误以为编辑完成。
func detectDefaultEditor() string {
	for _, c := range defaultEditorCandidates() {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return ""
}

func defaultEditorCandidates() []string {
	if runtime.GOOS == "windows" {
		return []string{"notepad"}
	}
	return []string{"vim", "vi", "nano"}
}

// openConfigEditor 在外部编辑器中打开当前 Mihomo 配置文件。
// 通过 tea.ExecProcess 暂停 TUI 并把终端交给编辑器，退出后恢复。
// 配置路径缺失或无可用编辑器时返回 ConfigEditFinishedMsg（携带错误）。
func (s State) openConfigEditor() (State, tea.Cmd) {
	if strings.TrimSpace(s.configPath) == "" {
		return s, configEditError(errors.New("未找到 mihomo 配置文件路径，无法打开编辑器"))
	}
	editor := resolveEditor()
	if editor == "" {
		return s, configEditError(errors.New("未找到可用的编辑器，请在 ~/.bashrc 或 ~/.zshrc 中设置 EDITOR 变量"))
	}
	fields := strings.Fields(editor)
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
