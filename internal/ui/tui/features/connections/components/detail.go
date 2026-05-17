package components

import (
	"fmt"

	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/styles"
	"github.com/charmbracelet/lipgloss"
)

type detailStyles struct {
	Header       lipgloss.Style
	SectionTitle lipgloss.Style
	Label        lipgloss.Style
	Value        lipgloss.Style
	JSON         lipgloss.Style
	Dim          lipgloss.Style
}

func newDetailStyles() detailStyles {
	return detailStyles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(styles.ColorPrimary),
		SectionTitle: lipgloss.NewStyle().
			Foreground(styles.ColorSecondary),
		Label: lipgloss.NewStyle().
			Foreground(styles.ColorPrimary),
		Value: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")),
		JSON: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")),
		Dim: lipgloss.NewStyle().
			Foreground(styles.ColorSecondary),
	}
}

// RenderConnectionDetail 渲染连接详情（JSON格式，支持滚动）
func RenderConnectionDetail(conn *model.Connection, ipInfo *model.IPInfo, height, scrollTop int) string {
	s := newDetailStyles()

	// 构建完整内容
	var allLines []string
	allLines = append(allLines, s.Header.Render("连接详情"))
	allLines = append(allLines, "")
	allLines = append(allLines, renderConnectionInfoSection(conn, s)...)
	allLines = append(allLines, "")

	jsonLines, err := renderJSONDetailSection(conn, s)
	if err != nil {
		return fmt.Sprintf("无法解析连接信息: %v", err)
	}
	allLines = append(allLines, jsonLines...)
	allLines = append(allLines, "")

	// 添加IP地理信息
	allLines = append(allLines, renderTargetIPGeoSection(ipInfo, s)...)
	allLines = append(allLines, "")

	// 计算可显示的行数
	maxDisplay := height - 4 // 留出空间给帮助提示和边框
	if maxDisplay < 10 {
		maxDisplay = 10
	}

	totalLines := len(allLines)

	// 限制滚动范围
	if scrollTop > totalLines-maxDisplay {
		scrollTop = totalLines - maxDisplay
	}
	if scrollTop < 0 {
		scrollTop = 0
	}

	// 截取显示内容
	endIdx := scrollTop + maxDisplay
	if endIdx > totalLines {
		endIdx = totalLines
	}

	visibleLines := allLines[scrollTop:endIdx]

	// 构建输出
	var output []string

	// 滚动提示（上方）
	if scrollTop > 0 {
		output = append(output, s.Dim.Render(fmt.Sprintf("↑ 还有 %d 行", scrollTop)))
	}

	output = append(output, visibleLines...)

	// 滚动提示（下方）
	if endIdx < totalLines {
		output = append(output, s.Dim.Render(fmt.Sprintf("↓ 还有 %d 行", totalLines-endIdx)))
	}

	// 帮助提示
	output = append(output, "")
	output = append(output, s.Dim.Render("[↑↓] 滚动 [Esc/Enter] 返回列表"))

	return strings.Join(output, "\n")
}
