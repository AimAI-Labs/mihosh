package logs

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// renderLogDetailModal 渲染日志详情模态弹窗（单列布局）
func renderLogDetailModal(
	log *model.LogEntry,
	parsed *ParsedLog,
	resolved *model.ResolvedIP,
	sourcePrivate bool,
	width, height, scroll int,
) string {
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.CSecondary).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.CWarning)

	title := titleStyle.Render("📋 日志详情")

	// 计算内部可用尺寸
	innerW := width - 12
	maxInnerW := width - 6
	if maxInnerW < 1 {
		maxInnerW = 1
	}
	if innerW < 40 {
		innerW = 40
	}
	if innerW > maxInnerW {
		innerW = maxInnerW
	}

	innerH := height - 8
	maxInnerH := height - 6
	if maxInnerH < 1 {
		maxInnerH = 1
	}
	if innerH < 15 {
		innerH = 15
	}
	if innerH > maxInnerH {
		innerH = maxInnerH
	}

	// ── 构建内容区域 ──
	var sections []string

	// 基础信息表格
	sections = append(sections, renderLogInfoTable(log, parsed, innerW))

	// 请求来源（仅内网IP时显示）
	if sourcePrivate {
		sections = append(sections, "")
		sections = append(sections, renderSourceSection(resolved, innerW))
	}

	// 原始日志
	sections = append(sections, "")
	rawTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.CSecondary).
		MarginBottom(1).
		Render("原始日志")
	rawContent := lipgloss.NewStyle().
		Foreground(common.CMuted).
		Width(innerW).
		Render(log.Payload)
	sections = append(sections, rawTitle)
	sections = append(sections, rawContent)

	content := strings.Join(sections, "\n")
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// ── 滚动计算 ──
	displayH := innerH - 2
	if displayH < 5 {
		displayH = 5
	}
	if scroll > totalLines-displayH {
		scroll = totalLines - displayH
	}
	if scroll < 0 {
		scroll = 0
	}
	endIdx := scroll + displayH
	if endIdx > totalLines {
		endIdx = totalLines
	}

	// ── 可见内容 + 滚动提示 ──
	var output []string
	if scroll > 0 {
		output = append(output, common.DimStyle.Render(fmt.Sprintf("↑ 还有 %d 行", scroll)))
	}
	output = append(output, lines[scroll:endIdx]...)
	if endIdx < totalLines {
		output = append(output, common.DimStyle.Render(fmt.Sprintf("↓ 还有 %d 行", totalLines-endIdx)))
	}

	modalContent := lipgloss.JoinVertical(lipgloss.Left, title, "", strings.Join(output, "\n"))
	modal := modalStyle.Render(modalContent)

	helpText := common.DimStyle.Render("[↑/↓] 滚动  [Esc/Enter/q] 关闭")

	centered := lipgloss.Place(width, height-2, lipgloss.Center, lipgloss.Center, modal)
	return lipgloss.JoinVertical(lipgloss.Left, centered, helpText)
}

// renderLogInfoTable 渲染基础信息表格
func renderLogInfoTable(log *model.LogEntry, parsed *ParsedLog, width int) string {
	sectionTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.CSecondary).
		MarginBottom(1)

	rows := [][]string{
		{"时间", log.Timestamp.Format("15:04:05")},
		{"级别", strings.ToUpper(log.Type)},
	}

	if parsed != nil && parsed.Protocol != "" {
		rows = append(rows, []string{"协议", parsed.Protocol})
		rows = append(rows, []string{"源地址", parsed.SourceIP + ":" + parsed.SourcePort})
		rows = append(rows, []string{"目标", parsed.DestHost + ":" + parsed.DestPort})
		rows = append(rows, []string{"规则", parsed.MatchRule})
		rows = append(rows, []string{"代理链", parsed.ProxyChain})
	}

	t := buildDetailTable(rows, width)
	return lipgloss.JoinVertical(lipgloss.Left, sectionTitle.Render("基础信息"), t)
}

// renderSourceSection 渲染请求来源区域
func renderSourceSection(resolved *model.ResolvedIP, width int) string {
	sectionTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.CSecondary).
		MarginBottom(1)

	if resolved == nil {
		loadingStyle := lipgloss.NewStyle().Foreground(common.CWarning)
		return lipgloss.JoinVertical(lipgloss.Left,
			sectionTitle.Render("🔍 请求来源"),
			loadingStyle.Render("  正在查询来源应用..."),
		)
	}

	typeDisplay := resolved.NetworkType
	switch resolved.NetworkType {
	case "docker":
		typeDisplay = "🐳 Docker"
	case "tailscale":
		typeDisplay = "🔗 Tailscale"
	case "local":
		typeDisplay = "💻 本机回环"
	case "lan":
		typeDisplay = "🏠 局域网"
	default:
		typeDisplay = "❓ 未知"
	}

	rows := [][]string{
		{"IP", resolved.IP},
		{"类型", typeDisplay},
	}

	if resolved.AppName != "" {
		label := "应用"
		switch resolved.NetworkType {
		case "docker":
			label = "容器"
		case "tailscale":
			label = "设备"
		}
		rows = append(rows, []string{label, resolved.AppName})
	}

	if resolved.AppDetail != "" {
		label := "详情"
		switch resolved.NetworkType {
		case "docker":
			label = "镜像"
		case "tailscale":
			label = "系统"
		}
		rows = append(rows, []string{label, resolved.AppDetail})
	}

	t := buildDetailTable(rows, width)
	return lipgloss.JoinVertical(lipgloss.Left, sectionTitle.Render("🔍 请求来源"), t)
}

// buildDetailTable 构建详情表格
func buildDetailTable(rows [][]string, width int) string {
	baseStyle := lipgloss.NewStyle().Padding(0, 1)
	keyWidth := 8
	valWidth := width - keyWidth - 6
	if valWidth < 10 {
		valWidth = 10
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if col == 0 {
				return baseStyle.Foreground(common.CPrimary).Width(keyWidth)
			}
			return baseStyle.Foreground(lipgloss.Color("#E5E7EB")).Width(valWidth)
		}).
		Rows(rows...)

	return t.Render()
}
