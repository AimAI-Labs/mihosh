package components

import (
	"fmt"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
	"github.com/charmbracelet/lipgloss"
)

// GetSiteTestLayout 根据页面宽度和卡片数量计算响应式的卡片列数和宽度
func GetSiteTestLayout(width int, numCards int) (cols int, cardWidth int) {
	if numCards <= 0 {
		numCards = 1
	}
	avail := width - 2
	if avail < 15 {
		return 1, 12
	}
	cols = avail / 15
	if cols < 1 {
		cols = 1
	}
	if cols > numCards {
		cols = numCards
	}
	cardWidth = (avail / cols) - 3
	if cardWidth < 12 {
		cardWidth = 12
	}
	if cardWidth > 20 {
		cardWidth = 20
	}
	return cols, cardWidth
}

// RenderSiteTestSection 渲染网站测速区域
func RenderSiteTestSection(siteTests []model.SiteTest, selectedIdx int, width int) string {
	layoutCols, cardWidth := GetSiteTestLayout(width, len(siteTests))

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
	innerWidth := width - 2 // 减去 padding(2)，border 绘制在 width 外部

	// 卡片边框样式 — Tokyo Night
	var borderColor lipgloss.Color
	if selected {
		borderColor = styles.Primary()   // #7AA2F7 蓝
	} else {
		borderColor = styles.Border()    // #414868 暗边框
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
		Foreground(styles.Text())

	// 名称：truncate 防止溢出
	name := site.Name
	if len([]rune(name)) > innerWidth {
		name = string([]rune(name)[:innerWidth-1]) + "…"
	}
	nameStyle := lipgloss.NewStyle().
		Foreground(styles.Gray()).
		Width(innerWidth).
		Align(lipgloss.Center)

	// 延迟 / 状态
	var delayStr string
	var delayColor lipgloss.Color

	switch {
	case site.Testing:
		delayStr = "⟳"
		delayColor = styles.Warning()  // #E0AF68 黄
	case site.Error != "":
		delayStr = "✗"
		delayColor = styles.Danger()   // #F7768E 红
	case site.Delay > 0:
		delayStr = fmt.Sprintf("%dms", site.Delay)
		switch {
		case site.Delay < 300:
			delayColor = styles.Success()  // #9ECE6A 绿
		case site.Delay < 800:
			delayColor = styles.Warning()  // #E0AF68 黄
		default:
			delayColor = styles.Danger()   // #F7768E 红
		}
	default:
		delayStr = "—"
		delayColor = styles.Border()   // #414868 暗灰
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
