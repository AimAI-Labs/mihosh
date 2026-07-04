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
	"github.com/mattn/go-runewidth"
)

// renderNodeInfo 渲染节点信息：Mode ● Group · Node · Delay
// 窄屏（<60列）隐藏策略组名
func renderNodeInfo(mode, groupName, nodeName string, delay, width int) string {
	// 无数据时显示占位符
	if mode == "" && groupName == "" && nodeName == "" {
		return lipgloss.NewStyle().Foreground(styles.Gray()).Render("● -- · --")
	}

	// 样式
	modeStyle := lipgloss.NewStyle().Foreground(styles.Gray())
	groupStyle := lipgloss.NewStyle().Foreground(styles.Gray())
	nodeStyle := lipgloss.NewStyle().Foreground(styles.Text())
	dotStyle := lipgloss.NewStyle().Foreground(common.TokyoCyan())
	sepStyle := lipgloss.NewStyle().Foreground(styles.Gray())

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
		delayColor := styles.Gray()
		switch {
		case delay < 100:
			delayColor = styles.Success()
		case delay < 300:
			delayColor = styles.Warning()
		default:
			delayColor = styles.Danger()
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
func RenderStatusBar(width int, err error, testing bool, testingTarget string, notice string, chartData *model.ChartData, uploadTotal int64, downloadTotal int64, mode string, groupName string, nodeName string, delay int, hasUpdate bool) string {
	// ── 左侧：节点信息 + 运行状态 / 错误 ──
	nodeInfo := renderNodeInfo(mode, groupName, nodeName, delay, width)

	// ── 右侧：实时指标 ──
	var metricsStr string
	dimStyle := lipgloss.NewStyle().Foreground(styles.Gray())
	upStyle := lipgloss.NewStyle().Foreground(styles.Success())
	downStyle := lipgloss.NewStyle().Foreground(styles.Primary())
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

	var status string
	metricsWidth := lipgloss.Width(metricsStr)
	nodeInfoWidth := lipgloss.Width(nodeInfo)

	if err != nil {
		errText := err.Error()
		friendlyErr := errText
		if strings.Contains(errText, "context dead") {
			friendlyErr = i18n.T("status.err.timeout_node")
		} else if strings.Contains(errText, "connection refused") {
			friendlyErr = i18n.T("status.err.refused")
		} else if strings.Contains(errText, "timeout") {
			friendlyErr = i18n.T("status.err.timeout")
		}

		// 截断长度需减去节点信息宽度和右侧指标宽度，以及图标等占用的边距
		maxErrLen := width - nodeInfoWidth - metricsWidth - 6
		if maxErrLen < 5 {
			maxErrLen = 5
		}
		status = styles.ErrorStyle().Render(fmt.Sprintf("✗ %s", truncateRunes(friendlyErr, maxErrLen)))
	} else if strings.TrimSpace(notice) != "" {
		maxNoticeLen := width - nodeInfoWidth - metricsWidth - 6
		if maxNoticeLen < 5 {
			maxNoticeLen = 5
		}
		status = styles.StatusStyle().Render("✔ " + truncateRunes(notice, maxNoticeLen))
	} else if testing {
		statusText := i18n.T("status.testing")
		if target := strings.TrimSpace(testingTarget); target != "" {
			statusText = fmt.Sprintf("%s: %s", i18n.T("status.testing"), target)
		}
		maxTestingLen := width - nodeInfoWidth - metricsWidth - 6
		if maxTestingLen < 5 {
			maxTestingLen = 5
		}
		status = styles.TestingStyle().Render(truncateRunes(statusText, maxTestingLen))
	} else {
		status = ""
	}

	// 组合左侧内容（节点信息始终显示）
	leftPart := nodeInfo
	if status != "" {
		leftPart = nodeInfo + " " + status
	}

	// ── 分隔线 ──
	divider := styles.DividerStyle().
		Render(strings.Repeat("─", width))

	var rightIcon string
	if hasUpdate {
		rightIcon = lipgloss.NewStyle().Foreground(common.TokyoCyan()).Blink(true).Render(" ◉")
	}

	// ── 组装状态行 ──
	// 计算右侧空间并右对齐
	gap := width - lipgloss.Width(leftPart) - lipgloss.Width(metricsStr) - lipgloss.Width(rightIcon) - 2
	if gap < 0 {
		gap = 0
	}
	statusLine := leftPart + strings.Repeat(" ", gap) + metricsStr + rightIcon

	return lipgloss.JoinVertical(lipgloss.Left, divider, statusLine)
}

func truncateRunes(s string, max int) string {
	if runewidth.StringWidth(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}

	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if w+rw > max-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	b.WriteString("…")
	return b.String()
}

// lastValue 获取切片最后一个元素，空切片返回 0
func lastValue(data []int64) int64 {
	if len(data) == 0 {
		return 0
	}
	return data[len(data)-1]
}
