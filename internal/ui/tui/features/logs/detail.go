package logs

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// OverlayLogDetailPopup 将日志详情弹窗叠加在 base 页面之上
func OverlayLogDetailPopup(
	base string,
	log *model.LogEntry,
	parsed *ParsedLog,
	resolved *model.ResolvedIP,
	sourcePrivate bool,
	width, height, scroll int,
) string {
	// ── 1. 暗化底层 ──
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	if len(baseLines) > height {
		baseLines = baseLines[:height]
	}

	faint := lipgloss.NewStyle().Faint(true)
	dimmed := make([]string, height)
	for i, l := range baseLines {
		dimmed[i] = faint.Render(l)
	}

	// ── 2. 弹窗居中计算 ──
	popup := renderLogDetailPopup(log, parsed, resolved, sourcePrivate, width, height, scroll)
	popupLines := strings.Split(popup, "\n")
	popupHeight := len(popupLines)
	if popupHeight == 0 {
		return strings.Join(dimmed, "\n")
	}

	popupWidth := lipgloss.Width(popupLines[0])
	leftOffset := (width - popupWidth) / 2
	if leftOffset < 0 {
		leftOffset = 0
	}
	topOffset := (height - popupHeight) / 2
	if topOffset < 0 {
		topOffset = 0
	}

	// ── 3. 弹窗行嵌入暗化底层 ──
	for i, pl := range popupLines {
		y := topOffset + i
		if y >= height {
			break
		}

		leftPart := ansi.Cut(dimmed[y], 0, leftOffset)
		leftW := lipgloss.Width(leftPart)
		if leftW < leftOffset {
			leftPart += strings.Repeat(" ", leftOffset-leftW)
		}

		rightPart := ansi.Cut(dimmed[y], leftOffset+popupWidth, width)
		dimmed[y] = leftPart + pl + rightPart
	}

	return strings.Join(dimmed, "\n")
}

// renderLogDetailPopup 渲染日志详情模态弹窗（单列布局）
func renderLogDetailPopup(
	log *model.LogEntry,
	parsed *ParsedLog,
	resolved *model.ResolvedIP,
	sourcePrivate bool,
	width, height, scroll int,
) string {
	popupWidth := width * 80 / 100
	if popupWidth < 60 {
		popupWidth = 60
	}
	if popupWidth > 120 {
		popupWidth = 120
	}
	if popupWidth > width-4 {
		popupWidth = width - 4
	}

	innerW := popupWidth - 4 // RenderTokyoPanel 内部会减2（边框）再减2（padding）

	innerH := height - 8
	maxInnerH := height - 6
	if maxInnerH < 1 {
		maxInnerH = 1
	}
	if innerH < 15 {
		innerH = 15
	}
	if innerH > maxInnerH {
		innerH = maxInnerH
	}

	// ── 构建内容区域 ──
	content := common.TokyoMutedStyle().
		Width(innerW).
		Render(log.Payload)
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// ── 滚动计算 ──
	displayH := innerH - 2
	if displayH < 5 {
		displayH = 5
	}
	if scroll > totalLines-displayH {
		scroll = totalLines - displayH
	}
	if scroll < 0 {
		scroll = 0
	}
	endIdx := scroll + displayH
	if endIdx > totalLines {
		endIdx = totalLines
	}

	// ── 可见内容 + 滚动提示 ──
	var output []string
	if scroll > 0 {
		output = append(output, common.DimStyle().Render(fmt.Sprintf("↑ 还有 %d 行", scroll)))
	}
	output = append(output, lines[scroll:endIdx]...)
	if endIdx < totalLines {
		output = append(output, common.DimStyle().Render(fmt.Sprintf("↓ 还有 %d 行", totalLines-endIdx)))
	}

	body := strings.Join(output, "\n")
	return common.RenderTokyoPanel(i18n.T("logs.detail.title"), body, popupWidth)
}

