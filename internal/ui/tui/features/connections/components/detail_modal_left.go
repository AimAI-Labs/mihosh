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

// RenderDetailModalLeft 渲染详情模态框的左侧（基础信息和地理信息）。
//
// 为与右侧 JSON 面板保持等高，左侧不再使用外部的滚动提示占位行，
// 而是把左侧整体视为一个与右侧同高的可滚动视图：
//   - 当内容不溢出时：两个子面板按自然高度渲染，底部用空行补齐到 height；
//   - 当内容溢出时：按 scrollTop 切片后渲染，并保留顶/底的「更多」提示行。
func RenderDetailModalLeft(conn *model.Connection, ipInfo *model.IPInfo, width, height, scrollTop int, isFocused bool) string {
	if height < 5 {
		height = 5
	}

	// 准备连接信息与 IP 地理信息行
	connRows := getConnInfoRows(conn)
	ipRows := getIPGeoInfoRows(ipInfo)

	connTable := renderInfoPanel(i18n.T("conns.detail.title"), connRows, width, len(connRows), isFocused)
	ipTable := renderInfoPanel(i18n.T("conns.detail.title_geo"), ipRows, width, len(ipRows), isFocused)

	// 合并两个面板（中间留 1 行间距）
	content := lipgloss.JoinVertical(lipgloss.Left, connTable, "", ipTable)
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// 情况一：内容不溢出，直接补齐到目标高度
	if totalLines <= height {
		for len(lines) < height {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}

	// 情况二：内容溢出，按滚动窗口渲染
	// 视口高度 = height - 2（顶部、底部各保留 1 行滚动提示）
	viewport := height - 2
	if viewport < 3 {
		viewport = 3
	}

	if scrollTop > totalLines-viewport {
		scrollTop = totalLines - viewport
	}
	if scrollTop < 0 {
		scrollTop = 0
	}

	endIdx := scrollTop + viewport
	if endIdx > totalLines {
		endIdx = totalLines
	}

	visibleLines := lines[scrollTop:endIdx]

	dimStyle := common.DimStyle()
	if isFocused {
		dimStyle = dimStyle.Foreground(common.TokyoCyan())
	}

	var output []string
	if scrollTop > 0 {
		output = append(output, dimStyle.Render(i18n.Tf("conns.detail.more_up", scrollTop)))
	} else {
		output = append(output, "")
	}

	output = append(output, visibleLines...)

	// 补齐到 height（视口 + 上下两行提示）
	for len(output) < height {
		output = append(output, "")
	}

	if endIdx < totalLines {
		output = append(output, dimStyle.Render(i18n.Tf("conns.detail.more_down", totalLines-endIdx)))
	} else {
		output = append(output, "")
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

// renderInfoPanel 渲染一个带边框的信息面板。
// targetRows 为目标正文行数：当大于实际行数时，用空行补齐，
// 使面板整体高度固定，便于与右侧 JSON 面板对齐。
func renderInfoPanel(title string, rows [][]string, width, targetRows int, isFocused bool) string {
	borderColor := common.TokyoMuted()
	titleColor := common.TokyoBlue()
	if isFocused {
		borderColor = common.TokyoPurple()
		titleColor = common.TokyoCyan()
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
					Foreground(common.TokyoBlue()).
					Width(keyWidth)
			}
			return baseStyle.
				Foreground(common.TokyoForeground()).
				Width(valWidth)
		}).
		Rows(rows...)

	body := t.Render()

	// 用空行将正文补齐到 targetRows，保证面板高度固定
	bodyLines := strings.Split(body, "\n")
	for len(bodyLines) < targetRows {
		bodyLines = append(bodyLines, "")
	}

	return common.RenderBorderedPanel(title, strings.Join(bodyLines, "\n"), width, borderColor, titleColor)
}
