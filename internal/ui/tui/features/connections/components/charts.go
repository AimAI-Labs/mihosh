package components

import (
	"fmt"
	"math"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/lipgloss"
)

const (
	chartPanelMinW   = 28 // 面板最小宽度
	chartPanelChrome = 4  // 面板边框+内边距占用宽度
)

const (
	maxSymmetricHalfHeight = 4 // 上半或下半最大行数
	minSymmetricHalfHeight = 2 // 上半或下半最小行数（低于此值不渲染图表）
)

// chartSectionChrome 图表面板边框占用行数（上下边框）
const chartSectionChrome = 3 // 上边框(1) + 中轴(1) + 下边框(1)

// ComputeChartSectionHeight 根据可用高度计算图表区域实际占用行数。
// maxHeight <= 0 或空间不足时返回 0。
func ComputeChartSectionHeight(maxHeight int) int {
	// 面板总行数 = 2*halfH + chartSectionChrome
	// 最少需要 halfH=2: 2*2+3 = 7
	if maxHeight < 2*minSymmetricHalfHeight+chartSectionChrome {
		return 0
	}
	halfH := (maxHeight - chartSectionChrome) / 2
	if halfH > maxSymmetricHalfHeight {
		halfH = maxSymmetricHalfHeight
	}
	if halfH < minSymmetricHalfHeight {
		return 0
	}
	return 2*halfH + chartSectionChrome
}

// RenderChartsSection 渲染监控图表区域（对称柱状图）
// maxHeight 控制图表最大行数，实现高度响应式；<= 0 时使用默认最大高度。
func RenderChartsSection(chartData *model.ChartData, width int, maxHeight int) string {
	if chartData == nil {
		return ""
	}

	// 动态计算 halfHeight
	halfH := maxSymmetricHalfHeight
	if maxHeight > 0 {
		computed := (maxHeight - chartSectionChrome) / 2
		if computed < halfH {
			halfH = computed
		}
	}
	if halfH < minSymmetricHalfHeight {
		return ""
	}

	panelWidth := width - 4 // 页面左右边距
	if panelWidth < chartPanelMinW {
		panelWidth = chartPanelMinW
	}

	chartWidth := panelWidth - chartPanelChrome
	if chartWidth < 16 {
		chartWidth = 16
	}

	body := RenderSymmetricBarChart(
		chartData.SpeedUpHistory,
		chartData.SpeedDownHistory,
		FormatSpeed,
		chartWidth,
		halfH,
	)
	return common.RenderTokyoPanel("上传/下载速度", body, panelWidth)
}


// FormatSpeed 格式化速度
func FormatSpeed(bytesPerSec int64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%d B/s", bytesPerSec)
	} else if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", float64(bytesPerSec)/1024)
	} else {
		return fmt.Sprintf("%.1f MB/s", float64(bytesPerSec)/(1024*1024))
	}
}

// FormatMemory 格式化内存
func FormatMemory(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.0f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.0f MB", float64(bytes)/(1024*1024))
	} else {
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	}
}

// RenderSymmetricBarChart 渲染对称柱状图（下载在上，上传在下）
// halfH 控制上半或下半的高度（行数），实现高度响应式。
func RenderSymmetricBarChart(uploadData, downloadData []int64, formatFunc func(int64) string, width int, halfH int) string {
	if width < 16 {
		width = 16
	}

	// 计算 Y 轴标签宽度
	maxVal := common.FindMax(uploadData)
	downloadMax := common.FindMax(downloadData)
	if downloadMax > maxVal {
		maxVal = downloadMax
	}
	if maxVal < 1 {
		maxVal = 1
	}

	labelMax := formatFunc(maxVal)
	labelHalf := formatFunc(maxVal / 2)
	labelWidth := len(labelMax)
	if w := len(labelHalf); w > labelWidth {
		labelWidth = w
	}
	if labelWidth < 8 {
		labelWidth = 8
	}

	// 图表内容宽度 = 总宽度 - 标签宽度 - 分隔符(2)
	chartWidth := width - labelWidth - 2
	if chartWidth < 8 {
		chartWidth = 8
	}

	// 采样数据
	sampledUp := common.SampleData(uploadData, chartWidth)
	sampledDown := common.SampleData(downloadData, chartWidth)

	// 颜色样式
	purpleStyle := lipgloss.NewStyle().Foreground(common.TokyoPurple)
	blueStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue)
	labelStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted)
	axisStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted)

	halfHF := float64(halfH)

	var lines []string

	// === 上半部分（下载柱，从中轴往上生长） ===
	for row := 0; row < halfH; row++ {
		// Y 轴标签
		var label string
		if row == 0 {
			label = labelStyle.Render(fmt.Sprintf("%*s", labelWidth, labelMax))
		} else if row == halfH/2 {
			label = labelStyle.Render(fmt.Sprintf("%*s", labelWidth, labelHalf))
		} else {
			label = strings.Repeat(" ", labelWidth)
		}

		// 渲染柱子（从中轴往上生长：row(halfH-1) = 靠近中轴，row 0 = 顶部）
		var bars strings.Builder
		for i := 0; i < chartWidth; i++ {
			barH := math.Round(float64(sampledDown[i]) / float64(maxVal) * halfHF)
			// 从中轴往上填充：barH=1 填 row(halfH-1)，barH=halfH 填 row0-row(halfH-1)
			if row >= halfH-int(barH) {
				bars.WriteString(purpleStyle.Render("█"))
			} else {
				bars.WriteRune(' ')
			}
		}

		separator := axisStyle.Render(" ┤")
		lines = append(lines, label+separator+bars.String())
	}

	// === 中轴行 ===
	var centerLabel strings.Builder
	centerLabel.WriteString(strings.Repeat(" ", labelWidth))
	centerLabel.WriteString(axisStyle.Render(" ┼"))

	for i := 0; i < chartWidth; i++ {
		if sampledDown[i] > 0 {
			centerLabel.WriteString(purpleStyle.Render("█"))
		} else {
			centerLabel.WriteString(axisStyle.Render("─"))
		}
	}
	lines = append(lines, centerLabel.String())

	// === 下半部分（上传柱，从中心轴向下生长） ===
	for row := 0; row < halfH; row++ {
		// Y 轴标签（靠近中轴时显示半值，最底部显示最大值）
		var label string
		if row == halfH-1 {
			label = labelStyle.Render(fmt.Sprintf("%*s", labelWidth, labelMax))
		} else if row == halfH/2 {
			label = labelStyle.Render(fmt.Sprintf("%*s", labelWidth, labelHalf))
		} else {
			label = strings.Repeat(" ", labelWidth)
		}

		// 渲染柱子
		var bars strings.Builder
		for i := 0; i < chartWidth; i++ {
			barH := math.Round(float64(sampledUp[i]) / float64(maxVal) * halfHF)
			// row 0 是最靠近中轴的行，需要 barH > row
			if barH > float64(row) {
				bars.WriteString(blueStyle.Render("█"))
			} else {
				bars.WriteRune(' ')
			}
		}

		separator := axisStyle.Render(" ┤")
		lines = append(lines, label+separator+bars.String())
	}

	return strings.Join(lines, "\n")
}
