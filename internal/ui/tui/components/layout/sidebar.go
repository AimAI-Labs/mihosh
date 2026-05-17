package layout

import (
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

// getSidebarItems 获取侧边栏项目
func getSidebarItems() []struct{ Label string } {
	return []struct{ Label string }{
		{i18n.T("menu.nodes")},
		{i18n.T("menu.connections")},
		{i18n.T("menu.logs")},
		{i18n.T("menu.rules")},
		{i18n.T("menu.settings")},
	}
}

// SidebarWidth 侧边栏渲染宽度（含右边框）
const SidebarWidth = 6

// RenderSidebar 渲染侧边栏
func RenderSidebar(currentPage PageType, height int) string {
	activeStyle := lipgloss.NewStyle().
		Foreground(styles.ColorPrimary).
		Bold(true).
		Width(SidebarWidth).
		Align(lipgloss.Center)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(styles.ColorGray).
		Width(SidebarWidth).
		Align(lipgloss.Center)

	var items []string
	sidebarItems := getSidebarItems()

	for i, item := range sidebarItems {
		var label string
		if PageType(i) == currentPage {
			label = activeStyle.Render(item.Label)
		} else {
			label = inactiveStyle.Render(item.Label)
		}
		items = append(items, label)
		if i < len(sidebarItems)-1 {
			items = append(items, "")
		}
	}

	content := strings.Join(items, "\n")

	// 截断超出的内容，防止侧边栏溢出
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	content = strings.Join(lines, "\n")

	// 用右侧边框分隔，内容垂直居中
	barStyle := lipgloss.NewStyle().
		Width(SidebarWidth).
		Height(height).
		AlignVertical(lipgloss.Center).
		BorderStyle(lipgloss.Border{Right: "│"}).
		BorderRight(true).
		BorderForeground(styles.ColorBorder)

	return barStyle.Render(content)
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

// SidebarMenuHeight 获取侧边栏菜单区域的高度（不含边框）
func SidebarMenuHeight(height int) int {
	// 侧边栏有5个菜单项，每个菜单项占1行，项之间有4个空行
	// 菜单项: 节点(0), 连接(1), 日志(2), 规则(3), 设置(4)
	// 总共占据: 5 + 4 = 9 行
	return 9
}

// GetClickedPage 获取点击位置对应的页面类型
// x, y 是点击的绝对坐标，height 是侧边栏的实际可用高度（contentHeight）
// 返回对应的页面类型，如果点击不在菜单区域返回 -1
func GetClickedPage(x, y, height int) PageType {
	// 侧边栏宽度为6（不含右边框），点击的X坐标应该 < 6
	if x < 0 || x >= SidebarWidth {
		return -1
	}

	// 菜单内容共 9 行（5项 + 4空行），lipgloss AlignVertical(Center)
	// 会在顶部插入 (height - 9) / 2 行空白，需要减去该偏移
	const menuContentHeight = 9
	topPadding := (height - menuContentHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}

	// 将绝对 y 转换为菜单内的相对 y
	menuY := y - topPadding
	if menuY < 0 || menuY >= menuContentHeight {
		return -1
	}

	// 计算点击的是哪个菜单项
	// 节点: 0, 连接: 1, 日志: 2, 规则: 3, 设置: 4
	// 空行位置: 1, 3, 5, 7
	clickedPage := menuY / 2

	// 如果点击的是空行位置，则无效
	if menuY%2 == 1 {
		return -1
	}

	// 确保不超过页面数量
	if clickedPage >= int(PageCount) {
		return -1
	}

	return PageType(clickedPage)
}
