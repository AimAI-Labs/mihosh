package components

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
)

func renderConnectionInfoSection(conn *model.Connection, s detailStyles) []string {
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

	lines := []string{
		s.SectionTitle.Render("─── 连接详情 ───"),
		"",
		renderKVLine("主机", host, s),
		renderKVLine("源地址", source, s),
		renderKVLine("目标地址", target, s),
		renderKVLine("协议", protocol, s),
		renderKVLine("规则链路", fmt.Sprintf("%s → %s", rule, chain), s),
		renderKVLine("连接时长", utils.FormatDuration(conn.Start), s),
		renderKVLine("流量", fmt.Sprintf("↓%s  ↑%s", utils.FormatBytes(conn.Download), utils.FormatBytes(conn.Upload)), s),
	}

	if conn.UploadSpeed > 0 || conn.DownloadSpeed > 0 {
		lines = append(lines, renderKVLine(
			"实时速率",
			fmt.Sprintf("↓%s/s  ↑%s/s", utils.FormatBytes(conn.DownloadSpeed), utils.FormatBytes(conn.UploadSpeed)),
			s,
		))
	}

	if conn.RulePayload != "" {
		lines = append(lines, renderKVLine("规则负载", conn.RulePayload, s))
	}

	process := firstNonEmpty(conn.Metadata.Process, conn.Metadata.ProcessPath)
	if process != "" {
		lines = append(lines, renderKVLine("进程", process, s))
	}

	return lines
}
