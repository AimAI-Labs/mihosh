package components

import (
	"fmt"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
	"github.com/charmbracelet/lipgloss"
)

// RenderSiteTestSection 渲染网站测速区域
func RenderSiteTestSection(siteTests []model.SiteTest, selectedIdx int, width int) string {
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
	return lipgloss.JoinVertical(lipgloss.Left, "", cardsContent)
}

// RenderSiteCard 渲染单个网站测速卡片
func RenderSiteCard(site model.SiteTest, selected bool, width int) string {
	innerWidth := width - 4 // 减去 border(2) + padding(2)

	// 卡片边框样式
	var borderColor lipgloss.Color
	if selected {
		borderColor = styles.ColorPrimary
	} else {
		borderColor = lipgloss.Color("#444")
	}

	cardStyle := lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		MarginRight(1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	// 图标
	iconStyle := lipgloss.NewStyle().
		Bold(true).
		Width(innerWidth).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color("#FFF"))

	// 名称：truncate 防止溢出
	name := site.Name
	if len([]rune(name)) > innerWidth {
		name = string([]rune(name)[:innerWidth-1]) + "…"
	}
	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#AAA")).
		Width(innerWidth).
		Align(lipgloss.Center)

	// 延迟 / 状态
	var delayStr string
	var delayColor lipgloss.Color

	switch {
	case site.Testing:
		delayStr = "⟳"
		delayColor = lipgloss.Color("#FFD700")
	case site.Error != "":
		delayStr = "✗"
		delayColor = lipgloss.Color("#FF6B6B")
	case site.Delay > 0:
		delayStr = fmt.Sprintf("%dms", site.Delay)
		switch {
		case site.Delay < 300:
			delayColor = lipgloss.Color("#00E676") // 绿
		case site.Delay < 800:
			delayColor = lipgloss.Color("#FFD700") // 黄
		default:
			delayColor = lipgloss.Color("#FF6B6B") // 红
		}
	default:
		delayStr = "—"
		delayColor = lipgloss.Color("#555")
	}

	delayStyle := lipgloss.NewStyle().
		Bold(site.Delay > 0 || site.Testing).
		Foreground(delayColor).
		Width(innerWidth).
		Align(lipgloss.Center)

	content := lipgloss.JoinVertical(lipgloss.Center,
		iconStyle.Render(site.Icon),
		nameStyle.Render(name),
		delayStyle.Render(delayStr),
	)
	return cardStyle.Render(content)
}
