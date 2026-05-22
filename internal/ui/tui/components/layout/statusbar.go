package layout

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

// RenderStatusBar 渲染底部状态栏（含实时指标和累计流量）
func RenderStatusBar(width int, err error, testing bool, testingTarget string, notice string, chartData *model.ChartData, uploadTotal int64, downloadTotal int64) string {
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
			friendlyErr = i18n.T("status.err.timeout_node")
		} else if strings.Contains(errText, "connection refused") {
			friendlyErr = i18n.T("status.err.refused")
		} else if strings.Contains(errText, "timeout") {
			friendlyErr = i18n.T("status.err.timeout")
		}

		status = styles.ErrorStyle.Render(fmt.Sprintf("✗ %s", friendlyErr))
	} else if strings.TrimSpace(notice) != "" {
		status = styles.StatusStyle.Render("✔ " + truncateRunes(notice, width/2))
	} else if testing {
		statusText := i18n.T("status.testing")
		if target := strings.TrimSpace(testingTarget); target != "" {
			// 避免节点名过长挤压状态栏布局
			maxTargetLen := width / 3
			if maxTargetLen < 8 {
				maxTargetLen = 8
			}
			target = truncateRunes(target, maxTargetLen)
			statusText = fmt.Sprintf("%s: %s", i18n.T("status.testing"), target)
		}
		status = styles.TestingStyle.Render(statusText)
	} else {
		status = styles.StatusStyle.Render("● " + i18n.T("status.normal"))
	}

	// ── 中部：快捷键提示 ──
	helpHint := lipgloss.NewStyle().
		Foreground(styles.ColorDim).
		Render(i18n.T("status.help_hint"))

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
		totalStr := dimStyle.Render(i18n.T("status.total")+":") +
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
