package layout

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

// RenderStatusBar 渲染底部状态栏（含实时指标和累计流量）
func RenderStatusBar(width int, err error, testing bool, testingTarget string, chartData *model.ChartData, uploadTotal int64, downloadTotal int64) string {
	// ── 左侧：运行状态 / 错误 ──
	var status string
	if err != nil {
		errText := err.Error()

		maxErrLen := width - 20
		if maxErrLen < 10 {
			maxErrLen = 10
		}
		if len(errText) > maxErrLen {
			errText = errText[:maxErrLen] + "..."
		}

		friendlyErr := errText
		if strings.Contains(errText, "context dead") {
			friendlyErr = "测速超时，节点可能不可用"
		} else if strings.Contains(errText, "connection refused") {
			friendlyErr = "无法连接mihomo API，请检查mihomo是否运行"
		} else if strings.Contains(errText, "timeout") {
			friendlyErr = "请求超时，请检查网络或增加超时时间"
		}

		status = styles.ErrorStyle.Render(fmt.Sprintf("✗ %s", friendlyErr))
	} else if testing {
		statusText := "⏳ 正在测速..."
		if target := strings.TrimSpace(testingTarget); target != "" {
			// 避免节点名过长挤压状态栏布局
			maxTargetLen := width / 3
			if maxTargetLen < 8 {
				maxTargetLen = 8
			}
			target = truncateRunes(target, maxTargetLen)
			statusText = fmt.Sprintf("⏳ 正在测速: %s", target)
		}
		status = styles.TestingStyle.Render(statusText)
	} else {
		status = styles.StatusStyle.Render("● 正常")
	}

	// ── 中部：快捷键提示 ──
	helpHint := lipgloss.NewStyle().
		Foreground(styles.ColorDim).
		Render("1-5 切页 │ / 搜索 │ ? 帮助 │ q 退出")

	// ── 右侧：实时指标 ──
	var metricsStr string
	dimStyle := lipgloss.NewStyle().Foreground(styles.ColorGray)
	upStyle := lipgloss.NewStyle().Foreground(styles.ColorSuccess)
	downStyle := lipgloss.NewStyle().Foreground(styles.ColorPrimary)

	if chartData != nil {
		mem := lastValue(chartData.MemoryHistory)
		upSpeed := lastValue(chartData.SpeedUpHistory)
		downSpeed := lastValue(chartData.SpeedDownHistory)

		metricsStr = dimStyle.Render(fmt.Sprintf("MEM %s", utils.FormatBytes(mem))) +
			"  " + upStyle.Render(fmt.Sprintf("↑%s/s", utils.FormatBytes(upSpeed))) +
			"  " + downStyle.Render(fmt.Sprintf("↓%s/s", utils.FormatBytes(downSpeed)))
	}

	// ── 总流量 ──
	if uploadTotal > 0 || downloadTotal > 0 {
		sep := dimStyle.Render("  │  ")
		totalStr := dimStyle.Render("总流量:") +
			"  " + upStyle.Render(fmt.Sprintf("↑%s", utils.FormatBytes(uploadTotal))) +
			"  " + downStyle.Render(fmt.Sprintf("↓%s", utils.FormatBytes(downloadTotal)))
		if metricsStr != "" {
			metricsStr = metricsStr + sep + totalStr
		} else {
			metricsStr = totalStr
		}
	}

	// ── 分隔线 ──
	divider := styles.DividerStyle.
		Render(strings.Repeat("─", width))

	// ── 组装状态行 ──
	leftPart := status + "  " + helpHint
	// 计算右侧空间并右对齐
	gap := width - lipgloss.Width(leftPart) - lipgloss.Width(metricsStr) - 2
	if gap < 0 {
		gap = 0
	}
	statusLine := leftPart + strings.Repeat(" ", gap) + metricsStr

	return lipgloss.JoinVertical(lipgloss.Left, divider, statusLine)
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}

// lastValue 获取切片最后一个元素，空切片返回 0
func lastValue(data []int64) int64 {
	if len(data) == 0 {
		return 0
	}
	return data[len(data)-1]
}
