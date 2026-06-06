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

	nameStyle := lipgloss.NewStyle().Foreground(common.TokyoForeground)
	bytesStyle := lipgloss.NewStyle().Foreground(common.TokyoCyan)
	barColor := common.TokyoPurple // 紫色进度条

	// 总面板宽度为 width - 4
	panelWidth := width - 4
	if panelWidth < 24 {
		panelWidth = 24
	}
	// contentWidth = 面板宽度 - 左右边框(2) - 左右padding(2) = panelWidth - 4
	contentWidth := panelWidth - 4

	var lines []string
	titleStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue).Bold(true)
	title := titleStyle.Render("Top 5 吞吐量(5min)")
	lines = append(lines, title)

	maxBytes := items[0].TotalBytes // 第一项是最大的

	// 根据可用宽度动态调整列宽
	// 窄屏时缩短名称列，确保进度条有空间
	nameWidth := 20
	if contentWidth < 60 {
		nameWidth = 10
	} else if contentWidth < 80 {
		nameWidth = 15
	}

	// 15是数值留宽, 4 是边距（│ + 前后空格）
	barsWidth := contentWidth - nameWidth - 15 - 4
	if barsWidth < 5 {
		barsWidth = 5
	}

	for _, item := range items {
		name := item.Name
		name = common.TruncateDisplay(name, nameWidth)
		name = common.PadString(name, nameWidth)

		nameStr := nameStyle.Render(name)

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

	body := strings.Join(lines, "\n")
	return common.RenderTokyoPanel("Top 5 吞吐量排行", body, panelWidth)
}
