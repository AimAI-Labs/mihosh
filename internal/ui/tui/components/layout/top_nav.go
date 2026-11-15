package layout

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// PageType 页面类型
type PageType int

const (
	PageNodes PageType = iota
	PageConnections
	PageLogs
	PageRules
	PageSub
	PageSettings
	PageCore
	PageCount // 页面总数，必须放在最后
)

// getTopNavItems 获取顶部导航项目
func getTopNavItems() []struct{ Label string } {
	return []struct{ Label string }{
		{i18n.T("menu.nodes")},
		{i18n.T("menu.connections")},
		{i18n.T("menu.logs")},
		{i18n.T("menu.rules")},
		{i18n.T("menu.sub")},
		{i18n.T("menu.settings")},
		{i18n.T("menu.core")},
	}
}

const TopNavHeight = 3

type TopNavRefreshStatus struct {
	Enabled          bool
	SecondsRemaining int
	Interval         int // configured refresh interval (seconds), used for stable width
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
	return lipgloss.NewStyle().Foreground(styles.Border())
}

func renderTopNavContent(currentPage PageType, width int, refreshStatus ...TopNavRefreshStatus) string {
	items := getTopNavItems()
	activeStyle := lipgloss.NewStyle().
		Background(theme.Resolve(common.Selected())).
		Foreground(theme.Resolve(styles.Primary())).
		Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(theme.Resolve(styles.Gray()))
	separatorStyle := inactiveStyle

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
	offset, visible, right := topNavViewport(currentPage, width, refreshStatus...)
	left = ansi.Cut(left, offset, offset+visible)
	if right == "" {
		return fitTopNavLine(left, width)
	}
	if visible <= 0 {
		return fitTopNavLine(right, width)
	}
	return fitTopNavLine(left, width-lipgloss.Width(right)-1) + " " + right
}

// topNavViewport is shared by rendering and hit testing. Narrow terminals keep
// the selected page visible while retaining the existing refresh indicator.
func topNavViewport(currentPage PageType, width int, status ...TopNavRefreshStatus) (int, int, string) {
	bounds := calcTopNavItemBounds()
	fullWidth := 0
	if len(bounds) > 0 {
		fullWidth = bounds[len(bounds)-1].EndX
	}
	right := ""
	if len(status) > 0 {
		right = renderTopNavRefreshStatus(status[0], true)
		if fullWidth+lipgloss.Width(right)+1 > width {
			right = renderTopNavRefreshStatus(status[0], false)
		}
	}
	visible := width
	if right != "" {
		visible = max(0, width-lipgloss.Width(right)-1)
	}
	offset := 0
	for _, b := range bounds {
		if b.Page == currentPage && b.EndX > visible {
			offset = b.EndX - visible
			break
		}
	}
	return offset, visible, right
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
	return GetClickedTopNavPageForState(x, y, width, PageNodes)
}

func GetClickedTopNavPageForState(x, y, width int, currentPage PageType, status ...TopNavRefreshStatus) PageType {
	if y != 1 || x <= 0 || x >= width-1 {
		return -1
	}
	offset, visible, _ := topNavViewport(currentPage, width-2, status...)
	contentX := x - 1
	if contentX >= visible {
		return -1
	}
	contentX += offset
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
	countdownStr := fmt.Sprintf("%ds", status.SecondsRemaining)

	// Compute a stable value width from the configured interval so that both
	// the countdown digits and the synced checkmark always occupy the same
	// cell count, preventing layout jitter during state transitions.
	ref := status.Interval
	if ref < status.SecondsRemaining {
		ref = status.SecondsRemaining
	}
	valueWidth := lipgloss.Width(fmt.Sprintf("%ds", ref))
	if minW := lipgloss.Width("✔"); minW > valueWidth {
		valueWidth = minW
	}

	var valueText string
	if status.Synced {
		valueText = "✔"
		if pad := valueWidth - lipgloss.Width(valueText); pad > 0 {
			valueText += strings.Repeat(" ", pad)
		}
	} else {
		valueText = countdownStr
		if pad := valueWidth - lipgloss.Width(valueText); pad > 0 {
			valueText = strings.Repeat(" ", pad) + valueText
		}
	}
	text := valueText
	if includeLabel {
		text = i18n.T("status.auto_refresh") + " " + valueText
	}
	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Foreground(theme.Resolve(styles.Success())).
		Render(text)
}

// GetPageTitle 获取页面标题
func GetPageTitle(page PageType) string {
	titles := []string{
		i18n.T("title.nodes"),
		i18n.T("title.connections"),
		i18n.T("title.logs"),
		i18n.T("title.rules"),
		i18n.T("title.sub"),
		i18n.T("title.settings"),
		i18n.T("title.core"),
	}
	if int(page) < len(titles) {
		return titles[page]
	}
	return ""
}
