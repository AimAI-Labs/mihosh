package components

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/lipgloss"
)

// TopNItem 上行+下行流量统计项
type TopNItem struct {
	Name       string
	TotalBytes int64
}

// RenderTopNSection 渲染吞吐量 Top N 排行榜
func RenderTopNSection(items []TopNItem, width int) string {
	if len(items) == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue).Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(common.TokyoForeground)
	bytesStyle := lipgloss.NewStyle().Foreground(common.TokyoCyan)
	barColor := common.TokyoPurple // 紫色进度条

	var lines []string
	title := titleStyle.Render("Top 5 吞吐量排行 (过去5分钟)")
	lines = append(lines, title)

	maxBytes := items[0].TotalBytes // 第一项是最大的

	// 根据可用宽度动态调整列宽
	// 窄屏时缩短名称列，确保进度条有空间
	nameWidth := 20
	if width < 60 {
		nameWidth = 10
	} else if width < 80 {
		nameWidth = 15
	}

	// 15是数值留宽, 4 是边距（│ + 前后空格）
	barsWidth := width - nameWidth - 15 - 4
	if barsWidth < 5 {
		barsWidth = 5
	}

	for _, item := range items {
		name := item.Name
		nameRunes := []rune(name)
		if len(nameRunes) > nameWidth {
			name = string(nameRunes[:nameWidth-3]) + "..."
			nameRunes = []rune(name)
		}

		// 格式化名字固定宽度
		padLen := nameWidth - len(nameRunes)
		if padLen < 0 {
			padLen = 0
		}
		nameStr := nameStyle.Render(name + strings.Repeat(" ", padLen))

		// 格式化数值，固定宽度 10
		bytesStr := FormatMemory(item.TotalBytes)
		padBytes := 12 - len(bytesStr)
		if padBytes < 0 {
			padBytes = 0
		}
		bytesStrRendered := bytesStyle.Render(strings.Repeat(" ", padBytes) + bytesStr)

		// 计算进度条
		ratio := float64(0)
		if maxBytes > 0 {
			ratio = float64(item.TotalBytes) / float64(maxBytes)
		}

		barLen := int(ratio * float64(barsWidth))
		if barLen < 1 && item.TotalBytes > 0 {
			barLen = 1
		}

		bar := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", barLen))
		emptyBar := lipgloss.NewStyle().Foreground(common.TokyoMuted).Render(strings.Repeat("░", barsWidth-barLen))

		sep := common.TokyoMutedStyle().Render(" │ ")
		line := nameStr + sep + bar + emptyBar + " " + bytesStrRendered
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
