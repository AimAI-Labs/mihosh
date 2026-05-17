package components

import (
	"fmt"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/lipgloss"
)

// RenderChartsSection 渲染监控图表区域
func RenderChartsSection(chartData *model.ChartData, width int) string {
	if chartData == nil {
		return ""
	}

	// 窄屏阈值：小于此宽度时切换为竖向堆叠布局
	const narrowThreshold = 90

	if width < narrowThreshold {
		return renderChartsNarrow(chartData, width)
	}
	return renderChartsWide(chartData, width)
}

// renderChartsWide 宽屏：三图表并排
func renderChartsWide(chartData *model.ChartData, width int) string {
	chartWidth := (width - 8) / 3
	if chartWidth < 20 {
		chartWidth = 20
	}
	if chartWidth > 45 {
		chartWidth = 45
	}

	// 速度图表配置
	speedConfig := common.SparklineConfig{
		Title:      "上传/下载速度",
		Width:      chartWidth,
		Height:     4,
		Color1:     lipgloss.Color("#00BFFF"), // 蓝色 - 上传
		Color2:     lipgloss.Color("#9370DB"), // 紫色 - 下载
		Label1:     "上传速度",
		Label2:     "下载速度",
		MinValue:   0, // Y轴完全自适应
		ShowXAxis:  true,
		MaxSeconds: 60,
		FormatFunc: func(v int64) string {
			return FormatSpeed(v)
		},
	}

	// 内存图表配置
	memoryConfig := common.SparklineConfig{
		Title:      "内存使用",
		Width:      chartWidth,
		Height:     4,
		Color1:     lipgloss.Color("#00FF7F"), // 绿色
		Label1:     "内存使用",
		MinValue:   0, // Y轴完全自适应
		ShowXAxis:  true,
		MaxSeconds: 60,
		FormatFunc: func(v int64) string {
			return FormatMemory(v)
		},
	}

	// 连接数图表配置
	connConfig := common.SparklineConfig{
		Title:      "连接",
		Width:      chartWidth,
		Height:     4,
		Color1:     lipgloss.Color("#FFD700"), // 金色
		Label1:     "连接",
		MinValue:   0, // 连接数Y轴自适应
		ShowXAxis:  true,
		MaxSeconds: 60,
		FormatFunc: func(v int64) string {
			return fmt.Sprintf("%d", v)
		},
	}

	// 渲染三个图表
	speedChart := common.RenderDualSparkline(
		chartData.SpeedUpHistory,
		chartData.SpeedDownHistory,
		speedConfig,
	)
	memoryChart := common.RenderSparkline(chartData.MemoryHistory, memoryConfig)
	connChart := common.RenderIntSparkline(chartData.ConnCountHistory, connConfig)

	// 横向拼接三个图表
	return lipgloss.JoinHorizontal(lipgloss.Top, speedChart, "  ", memoryChart, "  ", connChart)
}

// renderChartsNarrow 窄屏：图表竖向堆叠，每行一个图表
func renderChartsNarrow(chartData *model.ChartData, width int) string {
	chartWidth := width - 4
	if chartWidth < 20 {
		chartWidth = 20
	}
	if chartWidth > 45 {
		chartWidth = 45
	}

	speedConfig := common.SparklineConfig{
		Title:      "上传/下载速度",
		Width:      chartWidth,
		Height:     3,
		Color1:     lipgloss.Color("#00BFFF"),
		Color2:     lipgloss.Color("#9370DB"),
		Label1:     "上传速度",
		Label2:     "下载速度",
		MinValue:   0,
		ShowXAxis:  false,
		MaxSeconds: 60,
		FormatFunc: func(v int64) string { return FormatSpeed(v) },
	}

	memoryConfig := common.SparklineConfig{
		Title:      "内存使用",
		Width:      chartWidth,
		Height:     3,
		Color1:     lipgloss.Color("#00FF7F"),
		Label1:     "内存使用",
		MinValue:   0,
		ShowXAxis:  false,
		MaxSeconds: 60,
		FormatFunc: func(v int64) string { return FormatMemory(v) },
	}

	connConfig := common.SparklineConfig{
		Title:      "连接",
		Width:      chartWidth,
		Height:     3,
		Color1:     lipgloss.Color("#FFD700"),
		Label1:     "连接",
		MinValue:   0,
		ShowXAxis:  false,
		MaxSeconds: 60,
		FormatFunc: func(v int64) string { return fmt.Sprintf("%d", v) },
	}

	speedChart := common.RenderDualSparkline(
		chartData.SpeedUpHistory,
		chartData.SpeedDownHistory,
		speedConfig,
	)
	memoryChart := common.RenderSparkline(chartData.MemoryHistory, memoryConfig)
	connChart := common.RenderIntSparkline(chartData.ConnCountHistory, connConfig)

	return lipgloss.JoinVertical(lipgloss.Left, speedChart, "", memoryChart, "", connChart)
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
