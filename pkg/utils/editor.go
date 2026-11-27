package utils

// editor.go — 外部编辑器解析（rules 页与 sub 页共用）。
//
// 解析顺序：VISUAL → EDITOR → 当前 shell 的 rc 文件 → 默认编辑器。

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"
)

// EditorRCRe 从 shell 启动文件中匹配 EDITOR 赋值。
// 支持 export 前缀与单/双引号；未加引号时行内注释被截断。
// 多行匹配取最后一个有效值（与 shell 语义一致）。
var EditorRCRe = regexp.MustCompile(`(?m)^\s*(?:export\s+)?EDITOR=(?:"([^"]*)"|'([^']*)'|([^\s#]+))`)

// ResolveEditor 依次查找环境变量、当前 shell 的 rc 文件和默认编辑器。
func ResolveEditor() string {
	for _, name := range []string{"VISUAL", "EDITOR"} {
		if value := literalEditor(os.Getenv(name)); value != "" {
			return value
		}
	}
	shell := strings.TrimSuffix(strings.ToLower(filepath.Base(os.Getenv("SHELL"))), ".exe")
	if shell == "bash" || shell == "zsh" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if value := ParseEditorFromRCFile(filepath.Join(home, "."+shell+"rc")); value != "" {
				return value
			}
		}
	}
	return DetectDefaultEditor()
}

func literalEditor(value string) string {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "$`") {
		return ""
	}
	return value
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
		if v = literalEditor(v); v != "" {
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
	editor = strings.TrimSpace(editor)
	if filepath.IsAbs(editor) {
		if info, err := os.Stat(editor); err == nil && info.Mode().IsRegular() {
			return []string{editor}
		}
	}
	args := SplitCommandLine(editor)
	if len(args) == 0 || args[0] == "" {
		return nil
	}
	return args
}

// SplitCommandLine separates an executable and arguments using quoted fields.
// It preserves backslashes in paths and does not evaluate shell expressions.
func SplitCommandLine(line string) []string {
	var args []string
	var field strings.Builder
	var quote rune
	started := false
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		char := runes[i]
		if char == '\\' && quote != '\'' && i+1 < len(runes) {
			next := runes[i+1]
			if (next == '\\' && runtime.GOOS != "windows") || next == quote ||
				(quote == 0 && (next == '\'' || next == '"' || unicode.IsSpace(next))) {
				field.WriteRune(next)
				started = true
				i++
				continue
			}
		}
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
			} else {
				field.WriteRune(char)
			}
		case char == '"' || char == '\'':
			quote = char
			started = true
		case unicode.IsSpace(char):
			if started {
				args = append(args, field.String())
				field.Reset()
				started = false
			}
		default:
			field.WriteRune(char)
			started = true
		}
	}
	if quote != 0 {
		return nil
	}
	if started {
		args = append(args, field.String())
	}
	return args
}
