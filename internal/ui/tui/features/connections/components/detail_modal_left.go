package components

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// RenderDetailModalLeft 渲染详情模态框的左侧（基础信息和地理信息）
func RenderDetailModalLeft(conn *model.Connection, ipInfo *model.IPInfo, width, height, scrollTop int, isFocused bool) string {
	// 调整高度，为边框和标题留出空间
	maxHeight := height - 4
	if maxHeight < 5 {
		maxHeight = 5
	}

	// 准备连接信息表格
	connRows := getConnInfoRows(conn)
	connTable := renderInfoTable("连接详情", connRows, width, isFocused)

	// 准备IP地理信息表格
	ipRows := getIPGeoInfoRows(ipInfo)
	ipTable := renderInfoTable("目标 IP 地理信息", ipRows, width, isFocused)

	// 合并表格
	content := lipgloss.JoinVertical(lipgloss.Left, connTable, "", ipTable)
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// 处理滚动
	if scrollTop > totalLines-maxHeight {
		scrollTop = totalLines - maxHeight
	}
	if scrollTop < 0 {
		scrollTop = 0
	}

	endIdx := scrollTop + maxHeight
	if endIdx > totalLines {
		endIdx = totalLines
	}

	visibleLines := lines[scrollTop:endIdx]

	// 滚动提示
	var output []string
	dimStyle := common.DimStyle
	if isFocused {
		dimStyle = dimStyle.Foreground(lipgloss.Color("#00FF00"))
	}

	if scrollTop > 0 {
		output = append(output, dimStyle.Render(fmt.Sprintf("↑ 还有 %d 行", scrollTop)))
	} else {
		output = append(output, "") // 占位
	}

	output = append(output, visibleLines...)

	// 补齐高度
	for len(output) < maxHeight+1 {
		output = append(output, "")
	}

	if endIdx < totalLines {
		output = append(output, dimStyle.Render(fmt.Sprintf("↓ 还有 %d 行", totalLines-endIdx)))
	} else {
		output = append(output, "") // 占位
	}

	return strings.Join(output, "\n")
}

func getConnInfoRows(conn *model.Connection) [][]string {
	host := firstNonEmpty(conn.Metadata.Host, conn.Metadata.SniffHost, conn.Metadata.DestinationIP, "-")
	source := formatEndpoint(conn.Metadata.SourceIP, conn.Metadata.SourcePort)
	target := formatEndpoint(conn.Metadata.DestinationIP, conn.Metadata.DestinationPort)

	network := strings.ToUpper(firstNonEmpty(conn.Metadata.Network, "-"))
	connType := strings.ToUpper(firstNonEmpty(conn.Metadata.Type, "-"))
	protocol := fmt.Sprintf("%s/%s", network, connType)

	rule := firstNonEmpty(conn.Rule, "-")
	chain := "DIRECT"
	if len(conn.Chains) > 0 {
		chain = strings.Join(conn.Chains, " → ")
	}

	rows := [][]string{
		{"主机", host},
		{"源地址", source},
		{"目标地址", target},
		{"协议", protocol},
		{"规则链路", fmt.Sprintf("%s → %s", rule, chain)},
		{"连接时长", utils.FormatDuration(conn.Start)},
		{"流量", fmt.Sprintf("↓%s  ↑%s", utils.FormatBytes(conn.Download), utils.FormatBytes(conn.Upload))},
	}

	if conn.UploadSpeed > 0 || conn.DownloadSpeed > 0 {
		rows = append(rows, []string{
			"实时速率",
			fmt.Sprintf("↓%s/s  ↑%s/s", utils.FormatBytes(conn.DownloadSpeed), utils.FormatBytes(conn.UploadSpeed)),
		})
	}

	if conn.RulePayload != "" {
		rows = append(rows, []string{"规则负载", conn.RulePayload})
	}

	process := firstNonEmpty(conn.Metadata.Process, conn.Metadata.ProcessPath)
	if process != "" {
		rows = append(rows, []string{"进程", process})
	}

	return rows
}

func getIPGeoInfoRows(ipInfo *model.IPInfo) [][]string {
	if ipInfo == nil {
		return [][]string{{"状态", "正在加载 IP 信息..."}}
	}

	ip := firstNonEmpty(ipInfo.IP, ipInfo.Query, "-")
	location := strings.Join(nonEmptyStrings(ipInfo.Country, ipInfo.RegionName, ipInfo.City), ", ")
	if location == "" {
		location = "未知"
	}

	asn := firstNonEmpty(formatASNInt(ipInfo.ASN), ipInfo.AS)
	if asn == "" {
		asn = "-"
	}

	network := firstNonEmpty(ipInfo.ISP, ipInfo.Org, ipInfo.Organization, ipInfo.ASNOrganization, "-")

	rows := [][]string{
		{"IP", ip},
		{"位置", location},
		{"ASN", asn},
		{"网络", network},
	}

	if timezone := firstNonEmpty(ipInfo.Timezone); timezone != "" {
		rows = append(rows, []string{"时区", timezone})
	}

	lat, lon, hasCoord := coordinates(ipInfo)
	if hasCoord {
		rows = append(rows, []string{"坐标", fmt.Sprintf("%.3f, %.3f", lat, lon)})
	}

	return rows
}

func renderInfoTable(title string, rows [][]string, width int, isFocused bool) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.CSecondary).
		MarginBottom(1)

	if isFocused {
		titleStyle = titleStyle.Foreground(lipgloss.Color("#00FF00"))
	}

	// 基础样式
	baseStyle := lipgloss.NewStyle().Padding(0, 1)

	// 设置列宽
	keyWidth := 10
	valWidth := width - keyWidth - 6 // 考虑 padding 和边框
	if valWidth < 10 {
		valWidth = 10
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if col == 0 {
				return baseStyle.
					Foreground(common.CPrimary).
					Width(keyWidth)
			}
			return baseStyle.
				Foreground(lipgloss.Color("#E5E7EB")).
				Width(valWidth)
		}).
		Rows(rows...)

	return lipgloss.JoinVertical(lipgloss.Left, titleStyle.Render(title), t.Render())
}
