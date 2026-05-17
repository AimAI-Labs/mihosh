package components

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

// 表格列宽配置（固定列）
const (
	ColWidthClose = 4
	ColWidthDL    = 10
	ColWidthUL    = 10
	ColWidthTime  = 8
	// 弹性列最小宽度
	ColMinWidthHost  = 12
	ColMinWidthType  = 8
	ColMinWidthRule  = 8
	ColMinWidthChain = 8
)

// columnWidths 根据可用宽度动态计算各列宽度
// 优先保证固定列（关闭/下载/上传/时长），剩余空间按比例分配给弹性列
type columnWidths struct {
	Close int
	Host  int
	Type  int
	Rule  int
	Chain int
	DL    int
	UL    int
	Time  int
}

// calcColumnWidths 根据页面宽度动态计算列宽
// prefix 宽度（"► " 或 "  "）为 2 字符
func calcColumnWidths(pageWidth int) columnWidths {
	const prefixWidth = 2
	const colGap = 7 // 7 个列间空格

	// 固定列总宽度
	fixedTotal := ColWidthClose + ColWidthDL + ColWidthUL + ColWidthTime
	// 弹性列最小总宽度
	elasticMinTotal := ColMinWidthHost + ColMinWidthType + ColMinWidthRule + ColMinWidthChain

	available := pageWidth - prefixWidth - fixedTotal - colGap
	if available < elasticMinTotal {
		available = elasticMinTotal
	}

	// 按比例分配弹性列（主机:类型:规则:代理链 = 4:1.5:2:2.5）
	totalRatio := 10.0
	hostW := max(ColMinWidthHost, int(float64(available)*4.0/totalRatio))
	typeW := max(ColMinWidthType, int(float64(available)*1.5/totalRatio))
	ruleW := max(ColMinWidthRule, int(float64(available)*2.0/totalRatio))
	chainW := available - hostW - typeW - ruleW
	if chainW < ColMinWidthChain {
		chainW = ColMinWidthChain
	}

	return columnWidths{
		Close: ColWidthClose,
		Host:  hostW,
		Type:  typeW,
		Rule:  ruleW,
		Chain: chainW,
		DL:    ColWidthDL,
		UL:    ColWidthUL,
		Time:  ColWidthTime,
	}
}

// RenderTableHeader 渲染表头
func RenderTableHeader(style lipgloss.Style, pageWidth int) string {
	cols := calcColumnWidths(pageWidth)
	header := strings.Join([]string{
		alignCenter("", cols.Close),
		alignLeft("主机", cols.Host),
		alignLeft("类型", cols.Type),
		alignLeft("规则", cols.Rule),
		alignLeft("代理链", cols.Chain),
		alignRight("↓下载", cols.DL),
		alignRight("↑上传", cols.UL),
		alignRight("时长", cols.Time),
	}, " ")
	return style.Render(header)
}

// RenderConnectionRow 渲染单行连接
func RenderConnectionRow(conn model.Connection, style lipgloss.Style, prefix string, pageWidth int) string {
	cols := calcColumnWidths(pageWidth)

	// 主机
	host := conn.Metadata.Host
	if host == "" {
		host = conn.Metadata.DestinationIP
	}
	host = utils.TruncateString(host, cols.Host)

	// 类型
	connType := fmt.Sprintf("%s/%s", conn.Metadata.Network, conn.Metadata.Type)
	connType = utils.TruncateString(connType, cols.Type)

	// 规则
	rule := conn.Rule
	if conn.RulePayload != "" && utils.DisplayWidth(rule)+utils.DisplayWidth(conn.RulePayload)+1 <= cols.Rule {
		rule = fmt.Sprintf("%s:%s", rule, conn.RulePayload)
	}
	rule = utils.TruncateString(rule, cols.Rule)

	// 代理链（chains 为倒序，反转后用 → 拼接显示完整链路）
	chain := "DIRECT"
	if len(conn.Chains) > 0 {
		parts := make([]string, len(conn.Chains))
		for i, c := range conn.Chains {
			parts[len(conn.Chains)-1-i] = c
		}
		chain = strings.Join(parts, " → ")
	}
	chain = utils.TruncateString(chain, cols.Chain)

	// 流量
	download := utils.FormatBytes(conn.Download)
	upload := utils.FormatBytes(conn.Upload)

	// 连接时长
	duration := utils.FormatDuration(conn.Start)

	row := prefix + strings.Join([]string{
		alignCenter("×", cols.Close),
		alignLeft(host, cols.Host),
		alignLeft(connType, cols.Type),
		alignLeft(rule, cols.Rule),
		alignLeft(chain, cols.Chain),
		alignRight(download, cols.DL),
		alignRight(upload, cols.UL),
		alignRight(duration, cols.Time),
	}, " ")

	return style.Render(row)
}

func alignLeft(s string, width int) string {
	return utils.PadString(utils.TruncateString(s, width), width)
}

func alignRight(s string, width int) string {
	s = utils.TruncateString(s, width)
	padding := width - utils.DisplayWidth(s)
	if padding <= 0 {
		return s
	}
	return strings.Repeat(" ", padding) + s
}

func alignCenter(s string, width int) string {
	s = utils.TruncateString(s, width)
	padding := width - utils.DisplayWidth(s)
	if padding <= 0 {
		return s
	}

	left := padding / 2
	right := padding - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}
