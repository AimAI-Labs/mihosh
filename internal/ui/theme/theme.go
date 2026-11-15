package theme

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Theme 主题语义色集合
type Theme struct {
	Name string

	// 前景
	Foreground lipgloss.Color
	Muted      lipgloss.Color
	Dim        lipgloss.Color
	Bright     lipgloss.Color

	// 背景
	Background lipgloss.Color
	Surface    lipgloss.Color
	Overlay    lipgloss.Color
	Selected   lipgloss.Color

	// 主强调
	Primary   lipgloss.Color
	Secondary lipgloss.Color

	// 语义
	Success lipgloss.Color
	Warning lipgloss.Color
	Danger  lipgloss.Color
	Info    lipgloss.Color

	// 扩展
	Orange lipgloss.Color
	Purple lipgloss.Color
	Border lipgloss.Color
	Active lipgloss.Color
}

var current atomic.Pointer[Theme]

func init() {
	t := builtinThemes["tokyo-night"]
	current.Store(&t)
}

// Current 返回当前主题的副本（每次渲染时调用）
func Current() Theme { return *current.Load() }

// SetTheme 切换当前主题，成功返回 true
func SetTheme(name string) bool {
	t, ok := builtinThemes[name]
	if !ok {
		return false
	}
	current.Store(&t)
	return true
}

// CurrentName 返回当前主题名
func CurrentName() string { return current.Load().Name }

// IsValid 检查主题名是否有效
func IsValid(name string) bool {
	_, ok := builtinThemes[name]
	return ok
}

// Names 返回所有内置主题名（顺序稳定）
func Names() []string {
	return []string{"tokyo-night", "catppuccin", "gruvbox", "nord", "dracula"}
}

var resolvedColorCache sync.Map

// Resolve 将十六进制色值缓存为 CompleteColor，避免在 ANSI256/ANSI 终端下每帧重复计算 HSLuv 调色板匹配。
func Resolve(c lipgloss.Color) lipgloss.TerminalColor {
	s := string(c)
	if !strings.HasPrefix(s, "#") {
		return c
	}
	if cached, ok := resolvedColorCache.Load(s); ok {
		return cached.(lipgloss.CompleteColor)
	}
	rgb := termenv.RGBColor(s)
	a256, ok256 := termenv.ANSI256.Convert(rgb).(termenv.ANSI256Color)
	a16, ok16 := termenv.ANSI.Convert(rgb).(termenv.ANSIColor)
	if !ok256 || !ok16 {
		return c
	}
	resolved := lipgloss.CompleteColor{
		TrueColor: s,
		ANSI256:   strconv.Itoa(int(a256)),
		ANSI:      strconv.Itoa(int(a16)),
	}
	actual, _ := resolvedColorCache.LoadOrStore(s, resolved)
	return actual.(lipgloss.CompleteColor)
}
