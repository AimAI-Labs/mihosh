package common

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ============================================================
//  Tokyo Night — 共享调色板与面板组件
// ============================================================

// Tokyo 颜色访问函数定义于 colors.go（TokyoForeground/TokyoBlue 等），
// 此处仅保留样式函数，动态读取当前主题。

func TokyoTextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoForeground())
}

func TokyoHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoMuted()).Bold(true)
}

func TokyoMutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoMuted())
}

func TokyoCyanStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoCyan()).Bold(true)
}

func TokyoGreenStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoGreen()).Bold(true)
}

func TokyoRedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoRed())
}

func TokyoBlueStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(TokyoBlue())
}

// ============================================================
//  显示宽度工具函数
// ============================================================

// DisplayWidth 计算字符串的显示宽度（精确处理中文等宽字符）
func DisplayWidth(s string) int {
	return ansi.StringWidth(s)
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

	return ansi.Truncate(s, width, "~")
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

	// 逐字符填充顶部边框，确保总宽度精确等于 innerWidth+2（含 ╭ 和 ╮）
	topBorder := "╭─ " + title + " "
	for lipgloss.Width(topBorder) < innerWidth+1 {
		topBorder += "─"
	}
	topBorder += "╮"
	topLine := TokyoBlueStyle().Render(topBorder)
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

// RenderBorderedPanel 渲染自定义边框颜色的圆角面板（支持标题嵌入）
// 标题嵌入顶部边框：╭─ title ───╮
func RenderBorderedPanel(title, body string, width int, borderColor lipgloss.Color, titleColor lipgloss.Color) string {
	if width < 24 {
		width = 24
	}
	innerWidth := width - 2

	titleLen := DisplayWidth(title)
	maxTitleLen := innerWidth - 6
	if maxTitleLen > 0 && titleLen > maxTitleLen {
		title = TruncateDisplay(title, maxTitleLen)
	}

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Foreground(titleColor).Bold(true)

	topBorderStart := "╭─ "
	topBorderEnd := " "
	dashCount := innerWidth + 1 - DisplayWidth(topBorderStart) - DisplayWidth(title) - DisplayWidth(topBorderEnd)
	if dashCount < 0 {
		dashCount = 0
	}
	topBorderEnd += strings.Repeat("─", dashCount) + "╮"

	topLine := borderStyle.Render(topBorderStart) + titleStyle.Render(title) + borderStyle.Render(topBorderEnd)
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")

	bodyLines := strings.Split(body, "\n")
	var middleLines []string
	contentWidth := innerWidth - 2

	for _, line := range bodyLines {
		lineLen := lipgloss.Width(line)
		pad := contentWidth - lineLen
		if pad < 0 {
			line = TruncateDisplay(line, contentWidth)
			pad = 0
		}
		middleLine := borderStyle.Render("│ ") +
			line +
			strings.Repeat(" ", pad) +
			borderStyle.Render(" │")
		middleLines = append(middleLines, middleLine)
	}

	return topLine + "\n" + strings.Join(middleLines, "\n") + "\n" + bottomLine
}
