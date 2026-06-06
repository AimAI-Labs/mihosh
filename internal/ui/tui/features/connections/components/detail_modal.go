package components

import (
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/charmbracelet/lipgloss"
)

// RenderConnectionDetailImmersive 渲染沉浸式连接详情（嵌在导航栏下方）
func RenderConnectionDetailImmersive(
	conn *model.Connection,
	ipInfo *model.IPInfo,
	width, height, leftScroll, rightScroll, focusPanel int,
) string {
	if conn == nil || width <= 0 || height <= 0 {
		return ""
	}

	innerH := height
	if innerH < 5 {
		innerH = 5
	}

	var content string

	if width >= 100 {
		// 宽屏：左 1/3，右 2/3
		leftW := width / 3
		rightW := width - leftW - 2 // 减去列间距

		leftPanel := RenderDetailModalLeft(conn, ipInfo, leftW, innerH, leftScroll, focusPanel == 0)
		rightPanel := RenderDetailModalRight(conn, rightW, innerH, rightScroll, focusPanel == 1)

		content = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	} else {
		// 窄屏：上下堆叠，焦点所在的面板占据主要空间
		var topH, bottomH int
		if focusPanel == 0 {
			topH = innerH * 2 / 3
			bottomH = innerH - topH - 1
		} else {
			bottomH = innerH * 2 / 3
			topH = innerH - bottomH - 1
		}

		leftPanel := RenderDetailModalLeft(conn, ipInfo, width, topH, leftScroll, focusPanel == 0)
		rightPanel := RenderDetailModalRight(conn, width, bottomH, rightScroll, focusPanel == 1)

		content = lipgloss.JoinVertical(lipgloss.Left, leftPanel, "", rightPanel)
	}

	return content
}
