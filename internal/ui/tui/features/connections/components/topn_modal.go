package components

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/lipgloss"
)

// ResolveTopNModalBounds 返回 TopN 弹窗在页面坐标系中的边界（右下为开区间）。
func ResolveTopNModalBounds(items []TopNItem, width, height, scroll int) (left, top, right, bottom int) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0
	}

	modal := buildTopNModal(items, width, height, scroll)
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)

	containerHeight := height - 2
	if containerHeight < 1 {
		containerHeight = 1
	}

	leftGap := width - modalWidth
	if leftGap < 0 {
		leftGap = 0
	}
	topGap := containerHeight - modalHeight
	if topGap < 0 {
		topGap = 0
	}

	left = leftGap / 2
	top = topGap / 2
	right = left + modalWidth
	bottom = top + modalHeight
	return left, top, right, bottom
}

// RenderTopNModal 渲染吞吐量全量排行弹窗。
func RenderTopNModal(items []TopNItem, width, height, scroll int) string {
	modal := buildTopNModal(items, width, height, scroll)
	helpText := common.DimStyle.Render("[↑/↓/k/j] 滚动  [q/Esc/Enter] 关闭")

	centeredModal := lipgloss.Place(
		width,
		height-2,
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)

	return lipgloss.JoinVertical(lipgloss.Left, centeredModal, helpText)
}

func buildTopNModal(items []TopNItem, width, height, scroll int) string {
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.CSecondary).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.CWarning)

	rankStyle := lipgloss.NewStyle().Foreground(common.CSecondary)
	nameStyle := lipgloss.NewStyle().Foreground(common.CWhite)
	bytesStyle := lipgloss.NewStyle().Foreground(common.CPrimary)

	innerW := width - 20
	maxInnerW := width - 8
	if maxInnerW < 1 {
		maxInnerW = 1
	}
	if innerW < 50 {
		innerW = 50
	}
	if innerW > maxInnerW {
		innerW = maxInnerW
	}

	innerH := height - 10
	maxInnerH := height - 6
	if maxInnerH < 1 {
		maxInnerH = 1
	}
	if innerH < 10 {
		innerH = 10
	}
	if innerH > maxInnerH {
		innerH = maxInnerH
	}

	const rankColWidth = 4
	const bytesColWidth = 12
	const barBaseWidth = 15 // 进度条基础宽度

	// 1. 计算最长网址/名称宽度
	maxNameLen := 0
	for _, item := range items {
		l := len([]rune(item.Name))
		if l > maxNameLen {
			maxNameLen = l
		}
	}
	if maxNameLen < 15 {
		maxNameLen = 15
	}

	// 2. 根据最长名称动态确定弹窗宽度 (但不能超过 maxInnerW)
	// rank(4) + name + sep(3) + bar(15) + bytes(12) = 34 + name
	idealInnerW := rankColWidth + maxNameLen + 3 + barBaseWidth + bytesColWidth
	if innerW < idealInnerW {
		innerW = idealInnerW
	}
	if innerW > maxInnerW {
		innerW = maxInnerW
	}

	// 3. 动态分配各列宽度，优先保证名称不截断
	barWidth := barBaseWidth
	nameColWidth := maxNameLen

	// 如果总宽度不足以容纳，则先压缩进度条，再压缩名称
	remaining := innerW - rankColWidth - bytesColWidth - 3 // 减去固定列和分隔符
	if remaining < nameColWidth+barWidth {
		// 尝试压缩进度条到最小 10
		if remaining-nameColWidth >= 10 {
			barWidth = remaining - nameColWidth
		} else {
			// 必须压缩名称了
			barWidth = 10
			nameColWidth = remaining - barWidth
		}
	} else {
		// 如果有多余空间，可以给进度条
		barWidth = remaining - nameColWidth
		if barWidth > 40 { // 进度条不要太夸张
			barWidth = 40
			// 多出来的给名称或留白，这里保持 nameColWidth
		}
	}

	barColor := lipgloss.Color("#9370DB") // 紫色进度条

	var maxBytes int64
	if len(items) > 0 {
		maxBytes = items[0].TotalBytes
	}

	var rows []string
	if len(items) == 0 {
		rows = append(rows, common.DimStyle.Render("暂无吞吐量数据"))
	} else {
		for i, item := range items {
			name := item.Name
			if name == "" {
				name = "-"
			}
			name = truncateRunes(name, nameColWidth)

			nameRunes := []rune(name)
			if len(nameRunes) < nameColWidth {
				name += strings.Repeat(" ", nameColWidth-len(nameRunes))
			}

			rank := rankStyle.Render(fmt.Sprintf("%2d. ", i+1))
			nameText := nameStyle.Render(name)

			// 计算进度条
			ratio := float64(0)
			if maxBytes > 0 {
				ratio = float64(item.TotalBytes) / float64(maxBytes)
			}
			barLen := int(ratio * float64(barWidth))
			if barLen < 1 && item.TotalBytes > 0 {
				barLen = 1
			}
			bar := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", barLen))
			emptyBar := lipgloss.NewStyle().Foreground(common.CMuted).Render(strings.Repeat("░", barWidth-barLen))

			bytesStr := FormatMemory(item.TotalBytes)
			if len([]rune(bytesStr)) < bytesColWidth {
				bytesStr = strings.Repeat(" ", bytesColWidth-len([]rune(bytesStr))) + bytesStr
			}
			sizeText := bytesStyle.Render(bytesStr)
			rows = append(rows, rank+nameText+" │ "+bar+emptyBar+" "+sizeText)
		}
	}

	displayH := innerH
	if displayH < 5 {
		displayH = 5
	}

	totalRows := len(rows)
	if scroll > totalRows-displayH {
		scroll = totalRows - displayH
	}
	if scroll < 0 {
		scroll = 0
	}

	end := scroll + displayH
	if end > totalRows {
		end = totalRows
	}

	var content []string
	if scroll > 0 {
		content = append(content, common.DimStyle.Render(fmt.Sprintf("↑ 还有 %d 项", scroll)))
	}
	content = append(content, rows[scroll:end]...)
	if end < totalRows {
		content = append(content, common.DimStyle.Render(fmt.Sprintf("↓ 还有 %d 项", totalRows-end)))
	}

	title := titleStyle.Render("吞吐量排行 (过去5分钟)")
	modalContent := lipgloss.JoinVertical(lipgloss.Left, title, "", strings.Join(content, "\n"))
	return modalStyle.Render(modalContent)
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
