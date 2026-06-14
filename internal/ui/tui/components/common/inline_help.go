package common

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ============================================================
//  内联帮助提示面板（右下角浮层）—— 通用渲染辅助
// ============================================================
//
// 各页面共享此实现：页面只需构造 []InlineHelpHint（按键 + 描述），
// 调用 OverlayHelpAtBottomRight 将其叠加到页面右下角即可。
// 与页面无关的纯渲染逻辑集中于此，避免重复造轮子。

var (
	// InlineHelpKeyStyle 按键文本样式（醒目黄）
	InlineHelpKeyStyle = lipgloss.NewStyle().Foreground(TokyoYellow)
	// InlineHelpDimStyle 分隔符等次要文本样式（灰）
	InlineHelpDimStyle = lipgloss.NewStyle().Foreground(TokyoMuted)
	// InlineHelpDescStyle 描述文本样式（柔和蓝）
	InlineHelpDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A9B1D6"))
)

// InlineHelpHint 内联帮助条目（按键 + 描述）
type InlineHelpHint struct {
	Key  string
	Desc string
}

// FormatInlineHintRow 将一组帮助条目格式化为 "key desc · key desc" 单行
func FormatInlineHintRow(hints []InlineHelpHint) string {
	sep := InlineHelpDimStyle.Render(" · ")
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, InlineHelpKeyStyle.Render(h.Key)+" "+InlineHelpDescStyle.Render(h.Desc))
	}
	return strings.Join(parts, sep)
}

// OverlayHelpAtBottomRight 将帮助面板叠加在页面右下角（紧贴底栏上方，距右 1 字符）。
// panel 为已渲染好的单行（或多行）帮助文本；width/height 为页面内容区的可用尺寸。
func OverlayHelpAtBottomRight(base, panel string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	panelLines := strings.Split(panel, "\n")
	panelH := len(panelLines)
	if panelH == 0 {
		return base
	}
	panelW := 0
	for _, l := range panelLines {
		if w := lipgloss.Width(l); w > panelW {
			panelW = w
		}
	}

	// 定位：紧贴底栏上边（pageContent 最后一行），距右 1 字符
	startRow := len(baseLines) - panelH
	if startRow < 0 {
		startRow = 0
	}
	startCol := width - panelW - 1
	if startCol < 0 {
		startCol = 0
	}

	for i, pl := range panelLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}
		bl := baseLines[row]
		blW := lipgloss.Width(bl)
		plW := lipgloss.Width(pl)

		// 截取底层左侧
		leftPart := ""
		if startCol > 0 {
			leftPart = ansi.Cut(bl, 0, startCol)
			if lw := lipgloss.Width(leftPart); lw < startCol {
				leftPart += strings.Repeat(" ", startCol-lw)
			}
		}

		// 截取底层右侧
		rightPart := ""
		if blW > startCol+plW {
			rightPart = ansi.Cut(bl, startCol+plW, blW)
		}

		baseLines[row] = leftPart + pl + rightPart
	}

	return strings.Join(baseLines, "\n")
}
