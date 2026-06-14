package common

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// CalcThumbRange 计算滑块在垂直滚动条中的起止行（左闭右开）。
// viewHeight 为滚动条可视高度，total 为内容总行数，scrollTop 为当前顶部偏移。
// 适用于任意需要「等比映射」滚动位置的列表/详情面板。
func CalcThumbRange(viewHeight, total, scrollTop int) (start, end int) {
	if total <= 0 {
		return 0, viewHeight
	}
	thumbSize := float64(viewHeight) * float64(viewHeight) / float64(total)
	if thumbSize < 1 {
		thumbSize = 1
	}
	thumbStart := float64(scrollTop) * float64(viewHeight) / float64(total)
	start = int(thumbStart)
	end = start + int(thumbSize+0.5)
	if end > viewHeight {
		end = viewHeight
	}
	if start >= end {
		end = start + 1
	}
	return
}

// BuildVerticalScrollbar 构建高度为 viewHeight 的垂直滚动条字符串。
// 仅当内容溢出（total > viewHeight）时返回非空字符串；否则返回空串表示无需显示。
// thumbFocused 控制滑块是否使用焦点高亮色。
func BuildVerticalScrollbar(viewHeight, total, scrollTop int, thumbFocused bool) string {
	if total <= viewHeight {
		return ""
	}

	thumbStyle := lipgloss.NewStyle().Foreground(TokyoMuted)
	if thumbFocused {
		thumbStyle = lipgloss.NewStyle().Foreground(TokyoCyan)
	}
	trackStyle := DimStyle

	thumbStart, thumbEnd := CalcThumbRange(viewHeight, total, scrollTop)

	lines := make([]string, viewHeight)
	for i := 0; i < viewHeight; i++ {
		if i >= thumbStart && i < thumbEnd {
			lines[i] = thumbStyle.Render(SymbolScrollbarThumb)
		} else {
			lines[i] = trackStyle.Render(SymbolScrollbarTrack)
		}
	}
	return strings.Join(lines, "\n")
}
