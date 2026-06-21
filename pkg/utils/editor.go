package utils

// editor.go — 外部编辑器解析（rules 页与 sub 页共用）。
//
// 解析顺序：~/.bashrc → ~/.zshrc → VISUAL → EDITOR → 默认编辑器。

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// EditorRCRe 从 shell 启动文件中匹配 EDITOR 赋值。
// 支持 export 前缀与单/双引号；未加引号时行内注释被截断。
// 多行匹配取最后一个有效值（与 shell 语义一致）。
var EditorRCRe = regexp.MustCompile(`(?m)^\s*(?:export\s+)?EDITOR=(?:"([^"]*)"|'([^']*)'|([^\s#]+))`)

// ResolveEditor 按优先级解析编辑器：
//  1. ~/.bashrc 中的 EDITOR
//  2. ~/.zshrc 中的 EDITOR
//  3. VISUAL / EDITOR 环境变量（通常由 shell rc 导出）
//  4. 默认编辑器（Unix: vim→vi→nano；Windows: notepad）
//
// 返回值为编辑器命令（可能含参数，如 "code --wait"）；全部失败时返回空串。
func ResolveEditor() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		for _, name := range []string{".bashrc", ".zshrc"} {
			if v := ParseEditorFromRCFile(filepath.Join(home, name)); v != "" {
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
	return DetectDefaultEditor()
}

// ParseEditorFromRCFile 读取并解析单个 shell rc 文件中的 EDITOR 赋值。
func ParseEditorFromRCFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return ParseEditorFromRC(string(data))
}

// ParseEditorFromRC 从 rc 文件内容中提取 EDITOR 的值（取最后一个有效赋值）。
func ParseEditorFromRC(content string) string {
	var last string
	for _, m := range EditorRCRe.FindAllStringSubmatch(content, -1) {
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

// DetectDefaultEditor 在 PATH 中查找默认编辑器。
// TUI 场景下优先终端编辑器（阻塞式），避免 GUI 编辑器（如 code）立即返回导致误以为编辑完成。
func DetectDefaultEditor() string {
	for _, c := range DefaultEditorCandidates() {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return ""
}

// DefaultEditorCandidates 返回各平台默认编辑器候选（按优先级）。
func DefaultEditorCandidates() []string {
	if runtime.GOOS == "windows" {
		return []string{"notepad"}
	}
	return []string{"vim", "vi", "nano"}
}

// SplitEditorCommand 把编辑器命令字符串拆分为程序与参数切片。
// 例如 "code --wait" → ["code", "--wait"]。
func SplitEditorCommand(editor string) []string {
	return strings.Fields(editor)
}
