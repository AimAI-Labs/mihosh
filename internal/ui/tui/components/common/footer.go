package common

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
	"github.com/charmbracelet/lipgloss"
)

// RenderFooter 渲染统一的底部提示栏
// width: 终端宽度
// height: 终端高度
// currentContentHeight: 当前页面主要内容的高度（不含 footer）
// helpText: 提示信息内容
func RenderFooter(width, height, currentContentHeight int, helpText string) string {
	if helpText == "" {
		return ""
	}

	// 计算需要填充的空行数，以确保 footer 固定在底部
	// footer 占用 1 行，但如果 width 太窄导致 wrap 可能会占用多行
	// 使用 lipgloss 计算渲染后的高度
	styledFooter := styles.FooterStyle.Width(width).Render(helpText)
	footerHeight := lipgloss.Height(styledFooter)

	paddingLines := height - currentContentHeight - footerHeight
	if paddingLines < 0 {
		paddingLines = 0
	}

	padding := strings.Repeat("\n", paddingLines)

	return padding + styledFooter
}
