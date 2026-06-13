package components

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
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
		s.SectionTitle.Render(fmt.Sprintf("─── %s ───", i18n.T("conns.detail.title"))),
		"",
		renderKVLine(i18n.T("conns.detail.label.host"), host, s),
		renderKVLine(i18n.T("conns.detail.label.source"), source, s),
		renderKVLine(i18n.T("conns.detail.label.target"), target, s),
		renderKVLine(i18n.T("conns.detail.label.protocol"), protocol, s),
		renderKVLine(i18n.T("conns.detail.label.rule_chain"), fmt.Sprintf("%s → %s", rule, chain), s),
		renderKVLine(i18n.T("conns.detail.label.duration"), utils.FormatDuration(conn.Start), s),
		renderKVLine(i18n.T("conns.detail.label.traffic"), fmt.Sprintf("↓%s  ↑%s", utils.FormatBytes(conn.Download), utils.FormatBytes(conn.Upload)), s),
	}

	if conn.UploadSpeed > 0 || conn.DownloadSpeed > 0 {
		lines = append(lines, renderKVLine(
			i18n.T("conns.detail.label.speed"),
			fmt.Sprintf("↓%s/s  ↑%s/s", utils.FormatBytes(conn.DownloadSpeed), utils.FormatBytes(conn.UploadSpeed)),
			s,
		))
	}

	if conn.RulePayload != "" {
		lines = append(lines, renderKVLine(i18n.T("conns.detail.label.rule_payload"), conn.RulePayload, s))
	}

	process := firstNonEmpty(conn.Metadata.Process, conn.Metadata.ProcessPath)
	if process != "" {
		lines = append(lines, renderKVLine(i18n.T("conns.detail.label.process"), process, s))
	}

	return lines
}
