package common

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

// Sparkline 配置
type SparklineConfig struct {
	Title      string             // 图表标题
	Width      int                // 图表宽度
	Height     int                // 图表高度（行数）
	Color1     lipgloss.Color     // 第一条线颜色
	Color2     lipgloss.Color     // 第二条线颜色（可选）
	Label1     string             // 第一条线标签
	Label2     string             // 第二条线标签（可选）
	FormatFunc func(int64) string // Y轴格式化函数
	MinValue   int64              // Y轴最小刻度值（0表示自动）
	ShowXAxis  bool               // 是否显示X轴时间标签
	MaxSeconds int                // X轴最大秒数（默认60秒）
}

// DefaultSparklineConfig 默认配置
func DefaultSparklineConfig() SparklineConfig {
	return SparklineConfig{
		Width:    40,
		Height:   4,
		Color1:   styles.ColorPrimary,
		Color2:   lipgloss.Color("#9370DB"),
		MinValue: 0,
		FormatFunc: func(v int64) string {
			return utils.FormatBytes(v)
		},
	}
}

// RenderSparkline 渲染单数据系列的 sparkline
func RenderSparkline(data []int64, config SparklineConfig) string {
	return RenderDualSparkline(data, nil, config)
}

// RenderDualSparkline 渲染双数据系列的 sparkline（区域图样式）
func RenderDualSparkline(data1, data2 []int64, config SparklineConfig) string {
	if config.Width <= 0 {
		config.Width = 40
	}
	if config.Height <= 0 {
		config.Height = 4
	}
	if config.FormatFunc == nil {
		config.FormatFunc = func(v int64) string {
			return utils.FormatBytes(v)
		}
	}

	// 找出最大值用于缩放
	maxVal := FindMax(data1)
	if data2 != nil {
		maxVal2 := FindMax(data2)
		if maxVal2 > maxVal {
			maxVal = maxVal2
		}
	}
	// 使用配置的最小值（如果数据太小）
	if config.MinValue > 0 && maxVal < config.MinValue {
		maxVal = config.MinValue
	}
	// 确保最小有1，避免除零
	if maxVal < 1 {
		maxVal = 1
	}

	// 计算Y轴刻度
	yLabels := calculateYLabels(maxVal, config.Height, config.FormatFunc)
	labelWidth := maxLabelWidth(yLabels)
	if labelWidth < 8 {
		labelWidth = 8
	}

	// 渲染画布
	chartWidth := config.Width - labelWidth - 3
	if chartWidth < 10 {
		chartWidth = 10
	}

	// 采样数据以适应宽度
	sampled1 := SampleData(data1, chartWidth)
	var sampled2 []int64
	if data2 != nil {
		sampled2 = SampleData(data2, chartWidth)
	}

	// 样式
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.ColorPrimary)
	labelStyle := lipgloss.NewStyle().Foreground(styles.ColorSecondary)
	color1Style := lipgloss.NewStyle().Foreground(config.Color1)
	color2Style := lipgloss.NewStyle().Foreground(config.Color2)
	pauseStyle := lipgloss.NewStyle().Foreground(styles.ColorSecondary)
	axisStyle := lipgloss.NewStyle().Foreground(styles.ColorGray)

	var lines []string

	// 标题行
	if config.Title != "" {
		lines = append(lines, titleStyle.Render(config.Title)+" "+pauseStyle.Render("⏸"))
	}

	// 渲染图表行（从上到下）
	for row := 0; row < config.Height; row++ {
		// Y轴标签
		yLabel := ""
		if row < len(yLabels) {
			yLabel = fmt.Sprintf("%*s", labelWidth, yLabels[row])
		} else {
			yLabel = strings.Repeat(" ", labelWidth)
		}

		// 分隔符
		separator := axisStyle.Render(" ┤")

		// 渲染该行的数据
		rowContent := renderAreaRow(row, config.Height, sampled1, sampled2, maxVal, color1Style, color2Style)

		lines = append(lines, labelStyle.Render(yLabel)+separator+rowContent)
	}

	// X轴时间标签
	if config.ShowXAxis {
		xAxisLine := renderXAxisLabels(labelWidth, chartWidth, config.MaxSeconds, axisStyle)
		lines = append(lines, xAxisLine)
	}

	// 图例
	var legendParts []string
	if config.Label1 != "" {
		legendParts = append(legendParts, color1Style.Render("● "+config.Label1))
	}
	if config.Label2 != "" && data2 != nil {
		legendParts = append(legendParts, color2Style.Render("● "+config.Label2))
	}
	if len(legendParts) > 0 {
		padding := strings.Repeat(" ", labelWidth+3)
		lines = append(lines, padding+strings.Join(legendParts, "  "))
	}

	return strings.Join(lines, "\n")
}

// renderAreaRow 渲染一行区域图数据
func renderAreaRow(row, height int, data1, data2 []int64, maxVal int64, style1, style2 lipgloss.Style) string {
	var result strings.Builder

	// 该行对应的值区间（从上到下）
	// row 0 是最高行，对应 maxVal
	// row height-1 是最低行，对应 0
	threshold := float64(maxVal) * float64(height-row) / float64(height)

	for i := 0; i < len(data1); i++ {
		val1 := float64(0)
		val2 := float64(0)
		if i < len(data1) {
			val1 = float64(data1[i])
		}
		if data2 != nil && i < len(data2) {
			val2 = float64(data2[i])
		}

		// 决定显示哪个字符
		char1 := getBlockChar(val1, threshold, float64(maxVal)/float64(height))
		char2 := getBlockChar(val2, threshold, float64(maxVal)/float64(height))

		// 如果两个数据都有值，选择较大的那个显示
		if char1 != ' ' && char2 != ' ' {
			// 两个都有值时，优先显示较大的
			if val1 >= val2 {
				result.WriteString(style1.Render(string(char1)))
			} else {
				result.WriteString(style2.Render(string(char2)))
			}
		} else if char1 != ' ' {
			result.WriteString(style1.Render(string(char1)))
		} else if char2 != ' ' {
			result.WriteString(style2.Render(string(char2)))
		} else {
			result.WriteRune(' ')
		}
	}

	return result.String()
}

// getBlockChar 获取该位置应该显示的方块字符
func getBlockChar(value, threshold, step float64) rune {
	if value < threshold-step {
		return ' '
	}
	if value >= threshold {
		return '█'
	}
	// 在 threshold-step 到 threshold 之间，显示部分填充
	fraction := (value - (threshold - step)) / step
	if fraction >= 0.875 {
		return '█'
	} else if fraction >= 0.75 {
		return '▇'
	} else if fraction >= 0.625 {
		return '▆'
	} else if fraction >= 0.5 {
		return '▅'
	} else if fraction >= 0.375 {
		return '▄'
	} else if fraction >= 0.25 {
		return '▃'
	} else if fraction >= 0.125 {
		return '▂'
	}
	return '▁'
}

// RenderIntSparkline 渲染整数数据的 sparkline（用于连接数）
func RenderIntSparkline(data []int, config SparklineConfig) string {
	// 转换为 int64
	data64 := make([]int64, len(data))
	for i, v := range data {
		data64[i] = int64(v)
	}
	return RenderSparkline(data64, config)
}

// FindMax 找出切片中的最大值
func FindMax(data []int64) int64 {
	if len(data) == 0 {
		return 0
	}
	max := data[0]
	for _, v := range data[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

// calculateYLabels 计算Y轴标签
func calculateYLabels(maxVal int64, height int, format func(int64) string) []string {
	labels := make([]string, height)
	for i := 0; i < height; i++ {
		// 从上到下：最大值 -> 0
		val := maxVal * int64(height-i) / int64(height)
		labels[i] = format(val)
	}
	return labels
}

// maxLabelWidth 获取标签最大宽度
func maxLabelWidth(labels []string) int {
	max := 0
	for _, l := range labels {
		w := len([]rune(l))
		if w > max {
			max = w
		}
	}
	return max
}

// SampleData 采样数据以适应宽度
func SampleData(data []int64, width int) []int64 {
	if len(data) == 0 {
		return make([]int64, width)
	}
	if len(data) <= width {
		// 填充前面的空白
		result := make([]int64, width)
		offset := width - len(data)
		copy(result[offset:], data)
		return result
	}
	// 采样
	result := make([]int64, width)
	for i := 0; i < width; i++ {
		idx := i * len(data) / width
		result[i] = data[idx]
	}
	return result
}

// renderXAxisLabels 渲染X轴时间标签
func renderXAxisLabels(labelWidth, chartWidth, maxSeconds int, axisStyle lipgloss.Style) string {
	if maxSeconds <= 0 {
		maxSeconds = 60 // 默认60秒
	}

	// 左侧填充（与Y轴标签对齐）
	padding := strings.Repeat(" ", labelWidth+3)

	// 计算标签位置
	// 显示3-4个标签：起始、中间、结束
	var labels []string

	// 根据宽度决定显示几个标签
	if chartWidth >= 30 {
		// 显示4个标签：-60s, -40s, -20s, 0s
		step := chartWidth / 3
		positions := []int{0, step, step * 2, chartWidth - 1}
		times := []int{-maxSeconds, -maxSeconds * 2 / 3, -maxSeconds / 3, 0}

		var result strings.Builder
		result.WriteString(padding)

		lastPos := 0
		for i, pos := range positions {
			// 填充空格到当前位置
			if pos > lastPos {
				result.WriteString(strings.Repeat(" ", pos-lastPos))
			}
			label := fmt.Sprintf("%ds", times[i])
			result.WriteString(axisStyle.Render(label))
			lastPos = pos + len(label)
		}
		return result.String()
	}

	// 较窄时只显示2个标签
	labels = append(labels, fmt.Sprintf("-%ds", maxSeconds))
	labels = append(labels, "0s")

	// 计算间距
	leftLabel := axisStyle.Render(labels[0])
	rightLabel := axisStyle.Render(labels[1])
	gap := chartWidth - len(labels[0]) - len(labels[1])
	if gap < 0 {
		gap = 0
	}

	return padding + leftLabel + strings.Repeat(" ", gap) + rightLabel
}
