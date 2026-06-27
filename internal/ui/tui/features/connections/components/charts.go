package components

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/NimbleMarkets/ntcharts/sparkline"
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

	panelWidth := width // 与导航栏宽度对齐
	if panelWidth < chartPanelMinW {
		panelWidth = chartPanelMinW
	}

	chartWidth := panelWidth - chartPanelChrome
	if chartWidth < 16 {
		chartWidth = 16
	}

	body := RenderSpeedChart(
		chartData.SpeedUpHistory,
		chartData.SpeedDownHistory,
		FormatSpeed,
		chartWidth,
		halfH,
	)
	return common.RenderTokyoPanel(i18n.T("conns.chart_title"), body, panelWidth)
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

// toFloatSlice 转换 int64 为 float64 供 sparkline 使用
func toFloatSlice(data []int64) []float64 {
	res := make([]float64, len(data))
	for i, v := range data {
		res[i] = float64(v)
	}
	return res
}

// RenderSpeedChart 渲染固定时间采样的双向堆叠柱状图
// halfH 控制上半（下载）或下半（上传）的高度（行数），实现高度响应式。
func RenderSpeedChart(uploadData, downloadData []int64, formatFunc func(int64) string, width int, halfH int) string {
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
	labelWidth := len(labelMax)
	if labelWidth < 8 {
		labelWidth = 8
	}

	// 图表内容宽度 = 总宽度 - 标签宽度 - 分隔符(2)
	chartWidth := width - labelWidth - 2
	if chartWidth < 8 {
		chartWidth = 8
	}

	// 颜色样式
	purpleStyle := lipgloss.NewStyle().Foreground(common.TokyoPurple())
	blueStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue())
	labelStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted())
	axisStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted())

	// === 下载图表 ===
	downSL := sparkline.New(chartWidth, halfH,
		sparkline.WithData(toFloatSlice(downloadData)),
		sparkline.WithStyle(purpleStyle),
		sparkline.WithMaxValue(float64(maxVal)),
	)
	downSL.Draw()
	downView := downSL.View()

	var downLabels strings.Builder
	for i := 0; i < halfH; i++ {
		if i == 0 {
			downLabels.WriteString(labelStyle.Render(fmt.Sprintf("%*s", labelWidth, labelMax)))
		} else {
			downLabels.WriteString(strings.Repeat(" ", labelWidth))
		}
		downLabels.WriteString(axisStyle.Render(" ┤"))
		if i < halfH-1 {
			downLabels.WriteString("\n")
		}
	}
	downSection := lipgloss.JoinHorizontal(lipgloss.Top, downLabels.String(), downView)

	// === 中轴行 ===
	var center strings.Builder
	center.WriteString(strings.Repeat(" ", labelWidth))
	center.WriteString(axisStyle.Render(" ┼"))
	center.WriteString(axisStyle.Render(strings.Repeat("─", chartWidth)))

	// === 上传图表 ===
	upSL := sparkline.New(chartWidth, halfH,
		sparkline.WithData(toFloatSlice(uploadData)),
		sparkline.WithStyle(blueStyle),
		sparkline.WithMaxValue(float64(maxVal)),
	)
	upSL.Draw()
	upView := invertSparkline(upSL.View())

	var upLabels strings.Builder
	for i := 0; i < halfH; i++ {
		if i == halfH-1 {
			upLabels.WriteString(labelStyle.Render(fmt.Sprintf("%*s", labelWidth, labelMax)))
		} else {
			upLabels.WriteString(strings.Repeat(" ", labelWidth))
		}
		upLabels.WriteString(axisStyle.Render(" ┤"))
		if i < halfH-1 {
			upLabels.WriteString("\n")
		}
	}
	upSection := lipgloss.JoinHorizontal(lipgloss.Top, upLabels.String(), upView)

	return lipgloss.JoinVertical(lipgloss.Left, downSection, center.String(), upSection)
}

func invertSparkline(view string) string {
	lines := strings.Split(view, "\n")
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	mapped := strings.Join(lines, "\n")
	return strings.Map(func(r rune) rune {
		switch r {
		case '\u2581': return '\u2594' //  
		case '\u2582': return '\u2594' // ▂
		case '\u2583': return '\u2580' // ▃
		case '\u2584': return '\u2580' // ▄
		case '\u2585': return '\u2580' // ▅
		case '\u2586': return '\u2588' // ▆
		case '\u2587': return '\u2588' // ▇
		}
		return r
	}, mapped)
}
