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

// RenderConnectionDetailInline 渲染全新的内联连接详情页面，作为原页面信息的补充，
// 不显示IP地理位置信息，并支持响应式布局（自动将内容按列分布在一个统一的信息框中）
func RenderConnectionDetailInline(conn *model.Connection, width, height int, focused bool, scrollOffset int) string {
	if conn == nil || width <= 0 || height <= 0 {
		return ""
	}

	innerH := height
	if innerH < 3 {
		innerH = 3
	}
	targetRows := innerH - 2

	chain := "DIRECT"
	if len(conn.Chains) > 0 {
		chain = strings.Join(conn.Chains, " → ")
	}

	outbound := "DIRECT"
	if len(conn.Chains) > 0 {
		outbound = conn.Chains[0]
	}

	speedText := fmt.Sprintf("↓%s/s  ↑%s/s", utils.FormatBytes(conn.DownloadSpeed), utils.FormatBytes(conn.UploadSpeed))

	baseStyle := lipgloss.NewStyle().Padding(0, 1)

	// 1. 抽离长字段独占一行显示
	var topRows [][]string
	topRows = append(topRows, []string{"ID", conn.ID})
	if conn.Metadata.Host != "" {
		topRows = append(topRows, []string{i18n.T("conns.detail.label.host"), conn.Metadata.Host})
	}
	topRows = append(topRows, []string{i18n.T("conns.detail.label.chain"), chain})

	topTable := table.New().
		Border(lipgloss.HiddenBorder()).
		StyleFunc(func(row, col int) lipgloss.Style {
			if col == 0 {
				return baseStyle.Foreground(common.TokyoBlue()).Width(12)
			}
			return baseStyle.Foreground(common.TokyoForeground()).Width(width - 4 - 12 - 3)
		}).
		Rows(topRows...)

	topContent := topTable.Render()

	// 2. 抽离所有连接都有的公共字段（按照逻辑分组排序）
	commonRows := [][]string{
		{i18n.T("conns.detail.label.network"), strings.ToUpper(firstNonEmpty(conn.Metadata.Network, "-"))},
		{i18n.T("conns.detail.label.type"), strings.ToUpper(firstNonEmpty(conn.Metadata.Type, "-"))},
		{i18n.T("conns.detail.label.rule"), firstNonEmpty(conn.Rule, "-")},
		{i18n.T("conns.detail.label.outbound"), outbound},
		{i18n.T("conns.detail.label.duration"), utils.FormatDuration(conn.Start)},
		{i18n.T("conns.detail.label.traffic"), fmt.Sprintf("↓%s  ↑%s", utils.FormatBytes(conn.Download), utils.FormatBytes(conn.Upload))},
		{i18n.T("conns.detail.label.speed"), speedText},
		{i18n.T("conns.detail.label.source_ip"), firstNonEmpty(conn.Metadata.SourceIP, "-")},
		{i18n.T("conns.detail.label.source_port"), firstNonEmpty(conn.Metadata.SourcePort, "-")},
		{i18n.T("conns.detail.label.target_ip"), firstNonEmpty(conn.Metadata.DestinationIP, "-")},
		{i18n.T("conns.detail.label.target_port"), firstNonEmpty(conn.Metadata.DestinationPort, "-")},
	}

	// 3. 收集可有可无的补充字段
	var optionalRows [][]string
	if conn.RulePayload != "" {
		optionalRows = append(optionalRows, []string{i18n.T("conns.detail.label.rule_payload"), conn.RulePayload})
	}
	if conn.Metadata.SniffHost != "" {
		optionalRows = append(optionalRows, []string{i18n.T("conns.detail.label.sniff_host"), conn.Metadata.SniffHost})
	}
	if conn.Metadata.DNSMode != "" {
		optionalRows = append(optionalRows, []string{i18n.T("conns.detail.label.dns_mode"), conn.Metadata.DNSMode})
	}
	process := firstNonEmpty(conn.Metadata.Process, conn.Metadata.ProcessPath)
	if process != "" {
		optionalRows = append(optionalRows, []string{i18n.T("conns.detail.label.process"), process})
	}
	if conn.Metadata.InboundUser != "" {
		optionalRows = append(optionalRows, []string{i18n.T("conns.detail.label.inbound_user"), conn.Metadata.InboundUser})
	}

	// 计算响应式列数
	cols := 1
	if width >= 120 {
		cols = 3
	} else if width >= 80 {
		cols = 2
	}

	var colRows [][][]string
	for i := 0; i < cols; i++ {
		colRows = append(colRows, [][]string{})
	}

	// 分配公共字段（手动指定语义化的分组，确保在三列时地址信息独占第三列）
	var commonRowsPerCol int
	if cols == 3 {
		colRows[0] = append(colRows[0], commonRows[0:4]...)  // 基础信息：Network, Type, Rule, Outbound
		colRows[1] = append(colRows[1], commonRows[4:7]...)  // 流量速度：Duration, Traffic, Speed
		colRows[2] = append(colRows[2], commonRows[7:11]...) // 地址信息：SrcIP, SrcPort, DstIP, DstPort
		commonRowsPerCol = 4                                 // 三列时的最大行数
	} else if cols == 2 {
		colRows[0] = append(colRows[0], commonRows[0:6]...)
		colRows[1] = append(colRows[1], commonRows[6:11]...)
		commonRowsPerCol = 6
	} else {
		colRows[0] = append(colRows[0], commonRows...)
		commonRowsPerCol = 11
	}

	// 分配可选字段
	if len(optionalRows) > 0 {
		// 先补齐各列的高度，确保可选字段都在同一水平线开始，避免错位
		for i := 0; i < cols; i++ {
			for len(colRows[i]) < commonRowsPerCol {
				colRows[i] = append(colRows[i], []string{"", ""})
			}
		}

		optRowsPerCol := (len(optionalRows) + cols - 1) / cols
		for i, row := range optionalRows {
			colIdx := i / optRowsPerCol
			if colIdx >= cols {
				colIdx = cols - 1
			}
			colRows[colIdx] = append(colRows[colIdx], row)
		}
	}

	// 渲染多列表格并水平拼接
	var renderedCols []string
	colW := (width - 4) / cols

	for i := 0; i < cols; i++ {
		// 计算每一列的 key 和 val 宽度
		contentWidth := colW
		if i == cols-1 {
			// 最后一列填满剩余宽度
			contentWidth = width - 4 - colW*(cols-1)
		}
		keyW := 12
		valW := contentWidth - keyW - 3
		if valW < 5 {
			valW = 5
		}

		t := table.New().
			Border(lipgloss.HiddenBorder()).
			StyleFunc(func(row, col int) lipgloss.Style {
				if col == 0 {
					return baseStyle.Foreground(common.TokyoBlue()).Width(keyW)
				}
				return baseStyle.Foreground(common.TokyoForeground()).Width(valW)
			}).
			Rows(colRows[i]...)

		renderedCols = append(renderedCols, t.Render())
	}

	// 水平拼接所有列的内容
	colsContent := lipgloss.JoinHorizontal(lipgloss.Top, renderedCols...)

	// 拼合独占行和多列内容
	fullBody := topContent
	if colsContent != "" {
		fullBody += "\n\n" + colsContent
	}

	// 处理滚动
	fullLines := strings.Split(fullBody, "\n")

	maxScroll := len(fullLines) - targetRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}

	var visibleLines []string
	if len(fullLines) <= targetRows {
		visibleLines = fullLines
		for len(visibleLines) < targetRows {
			visibleLines = append(visibleLines, "")
		}
	} else {
		visibleLines = fullLines[scrollOffset : scrollOffset+targetRows]
	}

	body := strings.Join(visibleLines, "\n")

	borderColor := common.TokyoMuted()
	if focused {
		borderColor = common.TokyoBlue()
	}

	// 最后加上整体边框（标题：Connection Detail）
	content := common.RenderBorderedPanel(i18n.T("conns.detail.title"), body, width, borderColor, common.TokyoBlue())

	return content
}
