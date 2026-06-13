package components

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
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

	containerHeight := height
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

	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)
}

func buildTopNModal(items []TopNItem, width, height, scroll int) string {
	rankStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted)
	nameStyle := lipgloss.NewStyle().Foreground(common.TokyoForeground)
	bytesStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue)

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
		l := lipgloss.Width(item.Name)
		if l > maxNameLen {
			maxNameLen = l
		}
	}
	if maxNameLen < 15 {
		maxNameLen = 15
	}

	// 2. 根据最长名称动态确定弹窗宽度 (但不能超过 maxInnerW)
	// rank(4) + name + sep(3) + bar(15) + sep(1) + bytes(12) = 35 + name
	idealInnerW := rankColWidth + maxNameLen + 3 + barBaseWidth + 1 + bytesColWidth
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
	remaining := innerW - rankColWidth - bytesColWidth - 4 // 减去固定列和分隔符
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
			// 必须同步缩小 innerW，否则右侧会有大量留白
			innerW = rankColWidth + nameColWidth + barWidth + bytesColWidth + 4
		}
	}

	barColor := common.TokyoPurple // Tokyo Night 紫色进度条

	var maxBytes int64
	if len(items) > 0 {
		maxBytes = items[0].TotalBytes
	}

	var rows []string
	if len(items) == 0 {
		rows = append(rows, common.DimStyle.Render(i18n.T("conns.topn_modal_empty")))
	} else {
		for i, item := range items {
			name := item.Name
			if name == "" {
				name = "-"
			}
			name = common.TruncateDisplay(name, nameColWidth)
			name = common.PadString(name, nameColWidth)

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
		content = append(content, common.DimStyle.Render(i18n.Tf("conns.topn_modal_more_up", scroll)))
	}
	content = append(content, rows[scroll:end]...)
	if end < totalRows {
		content = append(content, common.DimStyle.Render(i18n.Tf("conns.topn_modal_more_down", totalRows-end)))
	}

	// 加上下空行使其有类似 Padding(1, 0) 的效果
	body := "\n" + strings.Join(content, "\n") + "\n"
	panelWidth := innerW + 4 // 对应 contentWidth = innerW
	return common.RenderTokyoPanel(i18n.T("conns.topn_modal_title"), body, panelWidth)
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
