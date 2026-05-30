package common

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// ============================================================
//  Tokyo Night — 共享调色板与面板组件
// ============================================================

// Tokyo Night 颜色常量
var (
	TokyoForeground = lipgloss.Color("#C0CAF5")
	TokyoMuted      = lipgloss.Color("#565F89")
	TokyoBlue       = lipgloss.Color("#7AA2F7")
	TokyoCyan       = lipgloss.Color("#7DCFFF")
	TokyoGreen      = lipgloss.Color("#9ECE6A")
	TokyoRed        = lipgloss.Color("#F7768E")
	TokyoYellow     = lipgloss.Color("#E0AF68")
	TokyoPurple     = lipgloss.Color("#BB9AF7")
	TokyoPanel      = lipgloss.Color("#1A1B26")
	TokyoSelected   = lipgloss.Color("#292E42")
)

// Tokyo 样式函数

func TokyoTextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoForeground)
}

func TokyoHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoMuted).Bold(true)
}

func TokyoMutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoMuted)
}

func TokyoCyanStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoCyan).Bold(true)
}

func TokyoGreenStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoGreen).Bold(true)
}

func TokyoRedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoRed)
}

func TokyoBlueStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoBlue)
}

// ============================================================
//  显示宽度工具函数
// ============================================================

// DisplayWidth 计算字符串的显示宽度（精确处理中文等宽字符）
func DisplayWidth(s string) int {
	return runewidth.StringWidth(s)
}

// PadString 将字符串填充到指定显示宽度
func PadString(s string, targetWidth int) string {
	currentWidth := DisplayWidth(s)
	if currentWidth >= targetWidth {
		return s
	}
	return s + strings.Repeat(" ", targetWidth-currentWidth)
}

// TruncateDisplay 按显示宽度截断字符串，过长时末尾加 ~
func TruncateDisplay(s string, width int) string {
	if width <= 0 || DisplayWidth(s) <= width {
		return s
	}
	if width == 1 {
		return "~"
	}

	var b strings.Builder
	current := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if rw < 0 {
			rw = 0
		}
		if current+rw > width-1 {
			break
		}
		b.WriteRune(r)
		current += rw
	}
	b.WriteRune('~')
	return b.String()
}

// ============================================================
//  Tokyo 面板渲染
// ============================================================

// RenderTokyoPanel 渲染带圆角边框的 Tokyo 风格面板
// 标题嵌入顶部边框：╭─ title ───╮
func RenderTokyoPanel(title, body string, width int) string {
	if width < 24 {
		width = 24
	}
	innerWidth := width - 2

	// 安全截断过长标题，防止折行
	titleLen := DisplayWidth(title)
	maxTitleLen := innerWidth - 6
	if maxTitleLen > 0 && titleLen > maxTitleLen {
		title = TruncateDisplay(title, maxTitleLen)
		titleLen = DisplayWidth(title)
	}

	topPrefix := "╭─ " + title + " "
	remaining := innerWidth - titleLen - 3
	if remaining < 0 {
		remaining = 0
	}
	topLine := TokyoBlueStyle().Render(topPrefix + strings.Repeat("─", remaining) + "╮")
	bottomLine := TokyoBlueStyle().Render("╰" + strings.Repeat("─", innerWidth) + "╯")

	bodyLines := strings.Split(body, "\n")
	var middleLines []string
	contentWidth := innerWidth - 2 // 左右 padding 各 1

	for _, line := range bodyLines {
		lineLen := lipgloss.Width(line)
		pad := contentWidth - lineLen
		if pad < 0 {
			pad = 0
			line = TruncateDisplay(line, contentWidth)
		}
		middleLine := TokyoBlueStyle().Render("│ ") +
			line +
			strings.Repeat(" ", pad) +
			TokyoBlueStyle().Render(" │")
		middleLines = append(middleLines, middleLine)
	}

	return topLine + "\n" + strings.Join(middleLines, "\n") + "\n" + bottomLine
}
