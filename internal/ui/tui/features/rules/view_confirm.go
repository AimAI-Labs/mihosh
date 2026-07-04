package rules

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ============================================================
//  删除规则确认弹窗（renderDeleteConfirmOverlay / buildDeleteConfirmModal）
// ============================================================
//
// 复用暗化+居中+嵌入三步叠加模式。弹窗展示待删除规则摘要，
// 提示用户按 Enter/y 确认或 Esc/n 取消。
//
// 删除为破坏性操作，点击弹窗外视为取消（安全默认）。

// deleteConfirmModalWidth 弹窗整体宽度（含边框）。
const deleteConfirmModalWidth = 50

// renderDeleteConfirmOverlay 渲染删除确认弹窗叠加层（同 renderTypeFilterOverlay 结构）。
func renderDeleteConfirmOverlay(background string, state PageState, width, height int) string {
	baseLines := strings.Split(background, "\n")
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

	modal := buildDeleteConfirmModal(state, width, height)
	modalLines := strings.Split(modal, "\n")
	modalHeight := len(modalLines)
	if modalHeight == 0 {
		return strings.Join(dimmed, "\n")
	}

	modalWidth := lipgloss.Width(modalLines[0])
	leftOffset := (width - modalWidth) / 2
	if leftOffset < 0 {
		leftOffset = 0
	}
	topOffset := rulesHeaderHeight + 1 // 从规则头部下方开始，避免视觉疲劳
	if topOffset < 0 {
		topOffset = 0
	}
	// 确保弹窗底部不会超出屏幕
	if topOffset+modalHeight > height {
		topOffset = height - modalHeight
		if topOffset < 0 {
			topOffset = 0
		}
	}

	for i, pl := range modalLines {
		y := topOffset + i
		if y >= height {
			break
		}

		leftPart := ansi.Cut(dimmed[y], 0, leftOffset)
		leftW := lipgloss.Width(leftPart)
		if leftW < leftOffset {
			leftPart += strings.Repeat(" ", leftOffset-leftW)
		}

		rightPart := ansi.Cut(dimmed[y], leftOffset+modalWidth, width)
		dimmed[y] = leftPart + pl + rightPart
	}

	return strings.Join(dimmed, "\n")
}

// buildDeleteConfirmModal 渲染删除确认弹窗本体（无暗化背景）。
// 内容：标题 + 分隔 + 规则摘要 + 确认提示。
func buildDeleteConfirmModal(state PageState, width, height int) string {
	modalWidth := deleteConfirmModalWidth
	if modalWidth > width-4 {
		modalWidth = width - 4
	}
	if modalWidth < 28 {
		modalWidth = 28
	}
	innerWidth := modalWidth - 4
	if innerWidth < 10 {
		innerWidth = 10
	}

	target := state.DeleteTarget
	indexStr := fmt.Sprintf("%d.", state.DeleteTargetIndex+1)

	// 序号行
	indexStyle := lipgloss.NewStyle().Foreground(common.Success())
	indexLine := indexStyle.Render(indexStr)

	// 类型标签（带规则类型颜色）
	typeColor := getAdjustedRuleTypeColor(target.Type, nil)
	typeStyle := lipgloss.NewStyle().Foreground(typeColor).Bold(true)
	typeLine := typeStyle.Render(target.Type)

	// no-resolve 标签
	var noResolveLine string
	if target.NoResolve {
		noResolveLine = lipgloss.NewStyle().Foreground(common.Warning()).Render(" [no-resolve]")
	}

	// Payload
	payloadStyle := lipgloss.NewStyle().Foreground(common.Secondary())
	payload := target.Payload
	maxPayload := innerWidth - 2
	if len(payload) > maxPayload {
		payload = payload[:maxPayload-3] + "..."
	}
	payloadLine := payloadStyle.Render(payload)

	// 代理
	proxyStyle := lipgloss.NewStyle().Foreground(common.Bright())
	proxyLine := proxyStyle.Render("→ " + target.Proxy)

	// 摘要行（单行截断）
	summary := indexLine + " " + typeLine + noResolveLine + " " + payloadLine + " " + proxyLine
	if lipgloss.Width(summary) > innerWidth {
		summary = lipgloss.NewStyle().Width(innerWidth).Render(summary[:innerWidth-3] + "...")
	}

	// 分隔
	divider := lipgloss.NewStyle().Foreground(common.TokyoBlue()).Render(strings.Repeat("─", innerWidth))

	// 确认提示
	prompt := common.TokyoMutedStyle().Render(i18n.T("rules.delete_confirm_hint"))

	modalContent := lipgloss.JoinVertical(lipgloss.Left,
		"",      // 上边框后空行
		summary, // 规则摘要
		"",      // 间距
		divider, // 分隔线
		prompt,  // 提示
	)

	return common.RenderBorderedPanel(
		i18n.T("rules.delete_title"),
		modalContent,
		modalWidth,
		common.Danger(),
		common.TokyoForeground(),
	)
}

// ResolveDeleteConfirmBounds 返回删除确认弹窗在页面坐标系中的边界（右下为开区间）。
func ResolveDeleteConfirmBounds(state PageState, width, height int) (left, top, right, bottom int) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0
	}
	modal := buildDeleteConfirmModal(state, width, height)
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)

	leftGap := width - modalWidth
	if leftGap < 0 {
		leftGap = 0
	}
	left = leftGap / 2
	top = rulesHeaderHeight + 1
	if top < 0 {
		top = 0
	}
	// 确保弹窗底部不会超出屏幕
	if top+modalHeight > height {
		top = height - modalHeight
		if top < 0 {
			top = 0
		}
	}
	right = left + modalWidth
	bottom = top + modalHeight
	return left, top, right, bottom
}
