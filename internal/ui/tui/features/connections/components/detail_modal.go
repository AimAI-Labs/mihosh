package components

import (
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/charmbracelet/lipgloss"
)

type detailPanelBounds struct {
	x, y, width, height int
}

// ConnectionDetailLayout keeps the rendered panels and their actual bounds together.
type ConnectionDetailLayout struct {
	Content string
	panels  [2]detailPanelBounds
}

// PanelAt returns the panel under the pointer, or -1 for gaps and empty space.
func (l ConnectionDetailLayout) PanelAt(x, y int) int {
	for i, panel := range l.panels {
		if x >= panel.x && x < panel.x+panel.width && y >= panel.y && y < panel.y+panel.height {
			return i
		}
	}
	return -1
}

// RenderConnectionDetailImmersive 渲染沉浸式连接详情（嵌在导航栏下方）
func RenderConnectionDetailImmersive(
	conn *model.Connection,
	ipInfo *model.IPInfo,
	width, height, leftScroll, rightScroll, focusPanel int,
) string {
	return LayoutConnectionDetailImmersive(conn, ipInfo, width, height, leftScroll, rightScroll, focusPanel).Content
}

func LayoutConnectionDetailImmersive(
	conn *model.Connection,
	ipInfo *model.IPInfo,
	width, height, leftScroll, rightScroll, focusPanel int,
) ConnectionDetailLayout {
	if conn == nil || width <= 0 || height <= 0 {
		return ConnectionDetailLayout{}
	}

	innerH := height
	if innerH < 5 {
		innerH = 5
	}

	var content string
	var layout ConnectionDetailLayout

	if width >= 100 {
		// 宽屏：左 1/3，右 2/3
		leftW := width / 3
		rightW := width - leftW - 2 // 减去列间距

		leftPanel := RenderDetailModalLeft(conn, ipInfo, leftW, innerH, leftScroll, focusPanel == 0)
		rightPanel := RenderDetailModalRight(conn, rightW, innerH, rightScroll, focusPanel == 1)

		content = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)
		layout.panels[0] = detailPanelBounds{width: lipgloss.Width(leftPanel), height: lipgloss.Height(leftPanel)}
		layout.panels[1] = detailPanelBounds{x: lipgloss.Width(leftPanel) + 2, width: lipgloss.Width(rightPanel), height: lipgloss.Height(rightPanel)}

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
		layout.panels[0] = detailPanelBounds{width: lipgloss.Width(leftPanel), height: lipgloss.Height(leftPanel)}
		layout.panels[1] = detailPanelBounds{y: lipgloss.Height(leftPanel) + 1, width: lipgloss.Width(rightPanel), height: lipgloss.Height(rightPanel)}
	}

	placement := dialog.New(content, width, height).Below(0)
	left, top, _, _ := placement.Bounds()
	for i := range layout.panels {
		layout.panels[i].x += left
		layout.panels[i].y += top
	}
	layout.Content = placement.View()
	return layout
}
