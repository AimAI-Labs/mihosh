package components

import (
	"fmt"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
	"github.com/charmbracelet/lipgloss"
)

// RenderSiteTestSection 渲染网站测速区域
func RenderSiteTestSection(siteTests []model.SiteTest, selectedIdx int, width int) string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.ColorPrimary)

	dimStyle := lipgloss.NewStyle().
		Foreground(styles.ColorSecondary)

	// 标题行
	title := headerStyle.Render("⚡ 网站测试")
	testAllBtn := dimStyle.Render("[S]测试全部")
	titleLine := title + "  " + testAllBtn

	// 根据宽度动态计算每行卡片数和卡片宽度
	layoutCols := 4
	if width < 60 {
		layoutCols = 2
	} else if width < 90 {
		layoutCols = 3
	}

	cardWidth := (width - 10) / layoutCols
	if cardWidth < 12 {
		cardWidth = 12
	}
	if cardWidth > 20 {
		cardWidth = 20
	}

	// 渲染网站卡片，按行分组
	var rowGroups [][]string
	for i, site := range siteTests {
		card := RenderSiteCard(site, i == selectedIdx, cardWidth)
		rowIdx := i / layoutCols
		if rowIdx >= len(rowGroups) {
			rowGroups = append(rowGroups, []string{})
		}
		rowGroups[rowIdx] = append(rowGroups[rowIdx], card)
	}

	var cardRows []string
	for _, group := range rowGroups {
		cardRows = append(cardRows, lipgloss.JoinHorizontal(lipgloss.Top, group...))
	}

	cardsContent := lipgloss.JoinVertical(lipgloss.Left, cardRows...)
	return lipgloss.JoinVertical(lipgloss.Left, titleLine, "", cardsContent)
}

// RenderSiteCard 渲染单个网站测速卡片
func RenderSiteCard(site model.SiteTest, selected bool, width int) string {
	// 卡片样式
	cardStyle := lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		MarginRight(1)

	if selected {
		cardStyle = cardStyle.
			Background(lipgloss.Color("#333")).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(styles.ColorPrimary)
	} else {
		cardStyle = cardStyle.
			Background(lipgloss.Color("#1a1a2e")).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444"))
	}

	// 图标样式
	iconStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFF"))

	// 名称样式
	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CCC")).
		Width(width - 4).
		Align(lipgloss.Center)

	// 延迟样式
	var delayStr string
	var delayStyle lipgloss.Style

	if site.Testing {
		delayStr = "测试中..."
		delayStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	} else if site.Error != "" {
		delayStr = site.Error
		delayStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))
	} else if site.Delay > 0 {
		delayStr = fmt.Sprintf("%dms", site.Delay)
		// 根据延迟着色
		if site.Delay < 500 {
			delayStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF7F")) // 绿色
		} else if site.Delay < 1000 {
			delayStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")) // 黄色
		} else {
			delayStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")) // 红色
		}
	} else {
		delayStr = "-"
		delayStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666"))
	}
	delayStyle = delayStyle.Width(width - 4).Align(lipgloss.Center)

	// 组装卡片内容
	icon := iconStyle.Render(site.Icon)
	name := nameStyle.Render(site.Name)
	delay := delayStyle.Render(delayStr)

	content := lipgloss.JoinVertical(lipgloss.Center, icon, name, delay)
	return cardStyle.Render(content)
}
