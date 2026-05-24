package layout

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

// PageType 页面类型
type PageType int

const (
	PageNodes PageType = iota
	PageConnections
	PageLogs
	PageRules
	PageSettings
	PageCount // 页面总数，必须放在最后
)

// getTopNavItems 获取顶部导航项目
func getTopNavItems() []struct{ Label string } {
	return []struct{ Label string }{
		{i18n.T("menu.nodes")},
		{i18n.T("menu.connections")},
		{i18n.T("menu.logs")},
		{i18n.T("menu.rules")},
		{i18n.T("menu.settings")},
	}
}

const TopNavHeight = 3

type TopNavRefreshStatus struct {
	Enabled          bool
	SecondsRemaining int
	Synced           bool
}

type topNavItemBounds struct {
	Page   PageType
	StartX int
	EndX   int
}

// RenderTopNav 渲染顶部横向导航栏。导航栏无标题，使用与节点管理面板一致的方框语言。
func RenderTopNav(currentPage PageType, width int, refreshStatus ...TopNavRefreshStatus) string {
	if width < commonMinTopNavWidth() {
		width = commonMinTopNavWidth()
	}
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	content := renderTopNavContent(currentPage, innerWidth, refreshStatus...)
	borderStyle := topNavBorderStyle()
	topLine := borderStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")
	middleLine := borderStyle.Render("│") +
		content +
		borderStyle.Render("│")
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")
	return topLine + "\n" + middleLine + "\n" + bottomLine
}

func commonMinTopNavWidth() int {
	return 24
}

func topNavBorderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(styles.ColorBorder)
}

func renderTopNavContent(currentPage PageType, width int, refreshStatus ...TopNavRefreshStatus) string {
	items := getTopNavItems()
	activeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#292E42")).
		Foreground(styles.ColorPrimary).
		Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(styles.ColorGray)
	separatorStyle := lipgloss.NewStyle().Foreground(styles.ColorGray)

	parts := make([]string, 0, len(items)*2)
	for i, item := range items {
		label := " " + item.Label + " "
		if PageType(i) == currentPage {
			parts = append(parts, activeStyle.Render(label))
		} else {
			parts = append(parts, inactiveStyle.Render(label))
		}
		if i < len(items)-1 {
			parts = append(parts, separatorStyle.Render("│"))
		}
	}

	left := lipgloss.JoinHorizontal(lipgloss.Left, parts...)
	right := ""
	if len(refreshStatus) > 0 {
		right = renderTopNavRefreshStatus(refreshStatus[0], true)
		if right != "" && lipgloss.Width(left)+lipgloss.Width(right)+1 > width {
			right = renderTopNavRefreshStatus(refreshStatus[0], false)
		}
	}
	if right == "" {
		return fitTopNavLine(left, width)
	}

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		leftWidth := width - lipgloss.Width(right) - 1
		if leftWidth <= 0 {
			return fitTopNavLine(right, width)
		}
		return truncateTopNavLine(left, leftWidth) + " " + right
	}
	return fitTopNavLine(left+strings.Repeat(" ", gap)+right, width)
}

func fitTopNavLine(line string, width int) string {
	lineWidth := lipgloss.Width(line)
	if lineWidth > width {
		return truncateTopNavLine(line, width)
	}
	return line + strings.Repeat(" ", width-lineWidth)
}

func truncateTopNavLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) <= width {
		return line
	}
	var b strings.Builder
	current := 0
	inEscape := false
	for _, r := range line {
		if inEscape {
			b.WriteRune(r)
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			b.WriteRune(r)
			continue
		}
		rw := lipgloss.Width(string(r))
		if current+rw > width {
			break
		}
		b.WriteRune(r)
		current += rw
	}
	if current < width {
		b.WriteString(strings.Repeat(" ", width-current))
	}
	return b.String()
}

// GetClickedTopNavPage 获取点击位置对应的顶部导航页面。
func GetClickedTopNavPage(x, y, width int) PageType {
	if y != 1 || x <= 0 || x >= width-1 {
		return -1
	}
	contentX := x - 1
	for _, bounds := range calcTopNavItemBounds() {
		if contentX >= bounds.StartX && contentX < bounds.EndX {
			return bounds.Page
		}
	}
	return -1
}

func calcTopNavItemBounds() []topNavItemBounds {
	items := getTopNavItems()
	bounds := make([]topNavItemBounds, 0, len(items))
	x := 0
	for i, item := range items {
		itemWidth := lipgloss.Width(" " + item.Label + " ")
		bounds = append(bounds, topNavItemBounds{
			Page:   PageType(i),
			StartX: x,
			EndX:   x + itemWidth,
		})
		x += itemWidth
		if i < len(items)-1 {
			x++
		}
	}
	return bounds
}

func renderTopNavRefreshStatus(status TopNavRefreshStatus, includeLabel bool) string {
	if !status.Enabled {
		return ""
	}
	text := fmt.Sprintf("%ds", status.SecondsRemaining)
	if status.Synced {
		text = "✔"
	}
	if includeLabel {
		text = i18n.T("status.auto_refresh") + " " + text
	}
	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Foreground(styles.ColorSuccess).
		Render(text)
}

// GetPageTitle 获取页面标题
func GetPageTitle(page PageType) string {
	titles := []string{
		i18n.T("title.nodes"),
		i18n.T("title.connections"),
		i18n.T("title.logs"),
		i18n.T("title.rules"),
		i18n.T("title.settings"),
	}
	if int(page) < len(titles) {
		return titles[page]
	}
	return ""
}
