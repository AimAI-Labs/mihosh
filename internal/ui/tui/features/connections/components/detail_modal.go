package components

import (
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/lipgloss"
)

// ResolveConnectionDetailModalBounds 返回详情弹窗在页面坐标系中的边界（右下为开区间）。
func ResolveConnectionDetailModalBounds(
	conn *model.Connection,
	ipInfo *model.IPInfo,
	width, height, leftScroll, rightScroll, focusPanel int,
) (left, top, right, bottom int) {
	if conn == nil || width <= 0 || height <= 0 {
		return 0, 0, 0, 0
	}

	modal := buildConnectionDetailModal(conn, ipInfo, width, height, leftScroll, rightScroll, focusPanel)
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)

	containerHeight := height - 2
	if containerHeight < 1 {
		containerHeight = 1
	}

	leftGap := width - modalWidth
	if leftGap < 0 {
		leftGap = 0
	}
	topGap := containerHeight - modalHeight
	if topGap < 0 {
		topGap = 0
	}

	left = leftGap / 2
	top = topGap / 2
	right = left + modalWidth
	bottom = top + modalHeight
	return left, top, right, bottom
}

// RenderConnectionDetailModal 渲染连接详情模态弹窗
func RenderConnectionDetailModal(
	conn *model.Connection,
	ipInfo *model.IPInfo,
	width, height, leftScroll, rightScroll, focusPanel int,
) string {
	modal := buildConnectionDetailModal(conn, ipInfo, width, height, leftScroll, rightScroll, focusPanel)

	helpText := common.DimStyle.Render("[←/→/h/l] 切换焦点  [↑/↓/k/j] 滚动  [q/Esc/Enter] 关闭")

	// 在整个屏幕中居中显示
	centeredModal := lipgloss.Place(
		width,
		height-2, // 为底部帮助留出空间
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)

	return lipgloss.JoinVertical(lipgloss.Left, centeredModal, helpText)
}

func buildConnectionDetailModal(
	conn *model.Connection,
	ipInfo *model.IPInfo,
	width, height, leftScroll, rightScroll, focusPanel int,
) string {
	// 模态框边框和标题样式
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.CSecondary).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.CWarning)

	host := firstNonEmpty(conn.Metadata.Host, conn.Metadata.SniffHost, conn.Metadata.DestinationIP, "-")
	title := titleStyle.Render("🔗 连接详情 - " + host)

	// 计算内部可用尺寸，保留一定外边距以支持“点击外部关闭”
	innerW := width - 12
	innerH := height - 10

	maxInnerW := width - 6  // 模态框边框(2) + 内边距(4)
	maxInnerH := height - 6 // 模态框边框(2) + 内边距(2) + 标题和间距
	if maxInnerW < 1 {
		maxInnerW = 1
	}
	if maxInnerH < 1 {
		maxInnerH = 1
	}

	if innerW < 40 {
		innerW = 40
	}
	if innerH < 15 {
		innerH = 15
	}
	if innerW > maxInnerW {
		innerW = maxInnerW
	}
	if innerH > maxInnerH {
		innerH = maxInnerH
	}

	// 布局计算：三列 vs 两列 vs 单列
	var content string

	if innerW >= 100 {
		// 宽屏：左右并排
		leftW := innerW * 4 / 10
		rightW := innerW - leftW - 4 // 减去列间距

		leftPanel := lipgloss.NewStyle().
			Width(leftW).
			Render(RenderDetailModalLeft(conn, ipInfo, leftW, innerH, leftScroll, focusPanel == 0))

		rightPanel := lipgloss.NewStyle().
			Width(rightW).
			Render(RenderDetailModalRight(conn, rightW, innerH, rightScroll, focusPanel == 1))

		content = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "    ", rightPanel)

	} else {
		// 窄屏：上下堆叠，焦点所在的面板占据主要空间
		// 动态分配高度
		var topH, bottomH int
		if focusPanel == 0 {
			topH = innerH * 2 / 3
			bottomH = innerH - topH - 2
		} else {
			bottomH = innerH * 2 / 3
			topH = innerH - bottomH - 2
		}

		leftPanel := lipgloss.NewStyle().
			Width(innerW).
			Render(RenderDetailModalLeft(conn, ipInfo, innerW, topH, leftScroll, focusPanel == 0))

		rightPanel := lipgloss.NewStyle().
			Width(innerW).
			Render(RenderDetailModalRight(conn, innerW, bottomH, rightScroll, focusPanel == 1))

		content = lipgloss.JoinVertical(lipgloss.Left, leftPanel, "", rightPanel)
	}

	// 组装模态框
	modalContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		content,
	)

	return modalStyle.Render(modalContent)
}
