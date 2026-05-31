package layout

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

// renderNodeInfo 渲染节点信息：Mode ● Group · Node · Delay
// 窄屏（<60列）隐藏策略组名
func renderNodeInfo(mode, groupName, nodeName string, delay, width int) string {
	// 无数据时显示占位符
	if mode == "" && groupName == "" && nodeName == "" {
		return lipgloss.NewStyle().Foreground(styles.ColorGray).Render("● -- · --")
	}

	// 样式
	modeStyle := lipgloss.NewStyle().Foreground(styles.ColorGray)
	groupStyle := lipgloss.NewStyle().Foreground(styles.ColorGray)
	nodeStyle := lipgloss.NewStyle().Foreground(styles.ColorText)
	dotStyle := lipgloss.NewStyle().Foreground(common.TokyoCyan)
	sepStyle := lipgloss.NewStyle().Foreground(styles.ColorGray)

	// 构建各部分
	modeDisplay := mode
	if modeDisplay == "" {
		modeDisplay = "--"
	}
	modeStr := modeStyle.Render(modeDisplay)
	dot := dotStyle.Render(" ● ")
	sep := sepStyle.Render(" · ")

	// 节点名截断（最大 20 字符）
	nodeDisplay := truncateRunes(nodeName, 20)
	if nodeDisplay == "" {
		nodeDisplay = "--"
	}
	nodeStr := nodeStyle.Render(nodeDisplay)

	// 延迟
	var delayStr string
	if delay > 0 {
		delayColor := styles.ColorGray
		switch {
		case delay < 100:
			delayColor = styles.ColorSuccess
		case delay < 300:
			delayColor = styles.ColorWarning
		default:
			delayColor = styles.ColorDanger
		}
		delayStr = lipgloss.NewStyle().Foreground(delayColor).Render(fmt.Sprintf("%dms", delay))
	} else {
		delayStr = modeStyle.Render("--")
	}

	// 组装（宽屏含策略组名，窄屏隐藏）
	if width >= 60 && groupName != "" {
		groupStr := groupStyle.Render(groupName)
		return modeStr + dot + groupStr + sep + nodeStr + sep + delayStr
	}
	return modeStr + dot + nodeStr + sep + delayStr
}

// RenderStatusBar 渲染底部状态栏（含实时指标和累计流量）
func RenderStatusBar(width int, err error, testing bool, testingTarget string, notice string, chartData *model.ChartData, uploadTotal int64, downloadTotal int64, mode string, groupName string, nodeName string, delay int) string {
	// ── 左侧：节点信息 + 运行状态 / 错误 ──
	nodeInfo := renderNodeInfo(mode, groupName, nodeName, delay, width)

	var status string
	if err != nil {
		errText := err.Error()
		// 截断长度需减去节点信息宽度
		maxErrLen := width - lipgloss.Width(nodeInfo) - 20
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
			maxTargetLen := width / 3
			if maxTargetLen < 8 {
				maxTargetLen = 8
			}
			target = truncateRunes(target, maxTargetLen)
			statusText = fmt.Sprintf("%s: %s", i18n.T("status.testing"), target)
		}
		status = styles.TestingStyle.Render(statusText)
	} else {
		status = ""
	}

	// 组合左侧内容（节点信息始终显示）
	leftPart := nodeInfo
	if status != "" {
		leftPart = nodeInfo + " " + status
	}

	// ── 右侧：实时指标 ──
	var metricsStr string
	dimStyle := lipgloss.NewStyle().Foreground(styles.ColorGray)
	upStyle := lipgloss.NewStyle().Foreground(styles.ColorSuccess)
	downStyle := lipgloss.NewStyle().Foreground(styles.ColorPrimary)
	sep := dimStyle.Render(" │ ")

	// 当前实时流量
	if chartData != nil {
		upSpeed := lastValue(chartData.SpeedUpHistory)
		downSpeed := lastValue(chartData.SpeedDownHistory)

		metricsStr = upStyle.Render(fmt.Sprintf("↑%s/s", utils.FormatBytes(upSpeed))) +
			"  " + downStyle.Render(fmt.Sprintf("↓%s/s", utils.FormatBytes(downSpeed)))
	}

	// 总流量
	if uploadTotal > 0 || downloadTotal > 0 {
		totalStr := upStyle.Render(fmt.Sprintf("↑%s", utils.FormatBytes(uploadTotal))) +
			"  " + downStyle.Render(fmt.Sprintf("↓%s", utils.FormatBytes(downloadTotal)))
		if metricsStr != "" {
			metricsStr = metricsStr + sep + totalStr
		} else {
			metricsStr = totalStr
		}
	}

	// MEM 放在最右边
	if chartData != nil {
		mem := lastValue(chartData.MemoryHistory)
		memStr := dimStyle.Render(fmt.Sprintf("MEM %s", utils.FormatBytes(mem)))
		if metricsStr != "" {
			metricsStr = metricsStr + sep + memStr
		} else {
			metricsStr = memStr
		}
	}

	// ── 分隔线 ──
	divider := styles.DividerStyle.
		Render(strings.Repeat("─", width))

	// ── 组装状态行 ──
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
