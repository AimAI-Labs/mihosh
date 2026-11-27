package logs

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
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
	popup := renderLogDetailPopup(log, parsed, resolved, sourcePrivate, width, height, scroll)
	return dialog.New(popup, width, height).Overlay(base, true)
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
		output = append(output, common.DimStyle().Render(i18n.Tf("logs.detail.more_above", scroll)))
	}
	output = append(output, lines[scroll:endIdx]...)
	if endIdx < totalLines {
		output = append(output, common.DimStyle().Render(i18n.Tf("logs.detail.more_below", totalLines-endIdx)))
	}

	body := strings.Join(output, "\n")
	return common.RenderTokyoPanel(i18n.T("logs.detail.title"), body, popupWidth)
}
