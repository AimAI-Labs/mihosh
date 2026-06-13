package components

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// RenderDetailModalLeft 渲染详情模态框的左侧（基础信息和地理信息）
func RenderDetailModalLeft(conn *model.Connection, ipInfo *model.IPInfo, width, height, scrollTop int, isFocused bool) string {
	// 左侧面板本身不需要再包一个大边框，它是由两个小的带边框面板组成的
	// 上下各有一行滚动提示，所以可见内容高度为 height - 2
	maxHeight := height - 2
	if maxHeight < 5 {
		maxHeight = 5
	}

	// 准备连接信息表格
	connRows := getConnInfoRows(conn)
	connTable := renderInfoPanel(i18n.T("conns.detail.title"), connRows, width, isFocused)

	// 准备IP地理信息表格
	ipRows := getIPGeoInfoRows(ipInfo)
	ipTable := renderInfoPanel(i18n.T("conns.detail.title_geo"), ipRows, width, isFocused)

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
		dimStyle = dimStyle.Foreground(common.TokyoCyan)
	}

	if scrollTop > 0 {
		output = append(output, dimStyle.Render(i18n.Tf("conns.detail.more_up", scrollTop)))
	} else {
		output = append(output, "") // 占位
	}

	output = append(output, visibleLines...)

	// 补齐高度
	for len(output) < maxHeight+1 {
		output = append(output, "")
	}

	if endIdx < totalLines {
		output = append(output, dimStyle.Render(i18n.Tf("conns.detail.more_down", totalLines-endIdx)))
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
		{i18n.T("conns.detail.label.host"), host},
		{i18n.T("conns.detail.label.source"), source},
		{i18n.T("conns.detail.label.target"), target},
		{i18n.T("conns.detail.label.protocol"), protocol},
		{i18n.T("conns.detail.label.rule_chain"), fmt.Sprintf("%s → %s", rule, chain)},
		{i18n.T("conns.detail.label.duration"), utils.FormatDuration(conn.Start)},
		{i18n.T("conns.detail.label.traffic"), fmt.Sprintf("↓%s  ↑%s", utils.FormatBytes(conn.Download), utils.FormatBytes(conn.Upload))},
	}

	if conn.UploadSpeed > 0 || conn.DownloadSpeed > 0 {
		rows = append(rows, []string{
			i18n.T("conns.detail.label.speed"),
			fmt.Sprintf("↓%s/s  ↑%s/s", utils.FormatBytes(conn.DownloadSpeed), utils.FormatBytes(conn.UploadSpeed)),
		})
	}

	if conn.RulePayload != "" {
		rows = append(rows, []string{i18n.T("conns.detail.label.rule_payload"), conn.RulePayload})
	}

	process := firstNonEmpty(conn.Metadata.Process, conn.Metadata.ProcessPath)
	if process != "" {
		rows = append(rows, []string{i18n.T("conns.detail.label.process"), process})
	}

	return rows
}

func getIPGeoInfoRows(ipInfo *model.IPInfo) [][]string {
	if ipInfo == nil {
		return [][]string{{i18n.T("conns.detail.label.status"), i18n.T("conns.detail.loading_geo")}}
	}

	ip := firstNonEmpty(ipInfo.IP, ipInfo.Query, "-")
	location := strings.Join(nonEmptyStrings(ipInfo.Country, ipInfo.RegionName, ipInfo.City), ", ")
	if location == "" {
		location = i18n.T("conns.detail.unknown")
	}

	asn := firstNonEmpty(formatASNInt(ipInfo.ASN), ipInfo.AS)
	if asn == "" {
		asn = "-"
	}

	network := firstNonEmpty(ipInfo.ISP, ipInfo.Org, ipInfo.Organization, ipInfo.ASNOrganization, "-")

	rows := [][]string{
		{i18n.T("conns.detail.label.ip"), ip},
		{i18n.T("conns.detail.label.location"), location},
		{i18n.T("conns.detail.label.asn"), asn},
		{i18n.T("conns.detail.label.network"), network},
	}

	if timezone := firstNonEmpty(ipInfo.Timezone); timezone != "" {
		rows = append(rows, []string{i18n.T("conns.detail.label.timezone"), timezone})
	}

	lat, lon, hasCoord := coordinates(ipInfo)
	if hasCoord {
		rows = append(rows, []string{i18n.T("conns.detail.label.coordinates"), fmt.Sprintf("%.3f, %.3f", lat, lon)})
	}

	return rows
}

func renderInfoPanel(title string, rows [][]string, width int, isFocused bool) string {
	borderColor := common.TokyoMuted
	titleColor := common.TokyoBlue
	if isFocused {
		borderColor = common.TokyoPurple
		titleColor = common.TokyoCyan
	}

	// 基础样式
	baseStyle := lipgloss.NewStyle().Padding(0, 1)

	// 设置列宽
	// RenderBorderedPanel: innerWidth=width-2, contentWidth=innerWidth-2=width-4
	// table with HiddenBorder: tableWidth = keyWidth + valWidth + 3 (overhead)
	// so valWidth = contentWidth - keyWidth - 3 = (width-4) - keyWidth - 3
	contentWidth := width - 4
	keyWidth := 10
	valWidth := contentWidth - keyWidth - 3
	if valWidth < 10 {
		valWidth = 10
	}

	t := table.New().
		Border(lipgloss.HiddenBorder()).
		StyleFunc(func(row, col int) lipgloss.Style {
			if col == 0 {
				return baseStyle.
					Foreground(common.TokyoBlue).
					Width(keyWidth)
			}
			return baseStyle.
				Foreground(common.TokyoForeground).
				Width(valWidth)
		}).
		Rows(rows...)

	return common.RenderBorderedPanel(title, t.Render(), width, borderColor, titleColor)
}
