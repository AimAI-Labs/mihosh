package utils

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// DisplayWidth 计算字符串的终端显示宽度（正确处理宽字符及窄 Unicode 符号）。
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

// TruncateString 根据显示宽度截断字符串（支持中文及窄列边界）
func TruncateString(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if DisplayWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= 2 {
		return runewidth.Truncate(s, maxWidth, "")
	}
	return runewidth.Truncate(s, maxWidth, "..")
}
