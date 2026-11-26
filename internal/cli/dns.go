package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

type dnsAnswerOutput struct {
	Name     string `json:"name"`
	Type     uint16 `json:"type"`
	TypeName string `json:"type_name"`
	TTL      uint32 `json:"ttl"`
	Data     string `json:"data"`
	FakeIP   bool   `json:"fake_ip"`
	Tag      string `json:"tag,omitempty"`
}

type dnsQueryOutput struct {
	Domain     string            `json:"domain"`
	RecordType string            `json:"record_type"`
	Status     int               `json:"status"`
	StatusName string            `json:"status_name"`
	HasFakeIP  bool              `json:"has_fake_ip"`
	Answers    []dnsAnswerOutput `json:"answers"`
}

func newDNSCommand() *cobra.Command {
	dnsCmd := &cobra.Command{
		Use: "dns",
	}
	var recordType string
	var outputRaw string
	queryCmd := &cobra.Command{
		Use:     "query <domain>",
		Args:    cobra.ExactArgs(1),
		Example: "  mihosh dns query example.com\n  mihosh dns query example.com --type AAAA --output table\n  mihosh dns query example.com --type A --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(outputRaw)
			if err != nil {
				return wrapParameterError(err)
			}
			domain := strings.TrimSpace(args[0])
			if domain == "" {
				return wrapParameterError(fmt.Errorf("domain name is required"))
			}
			qtype := strings.ToUpper(strings.TrimSpace(recordType))
			if qtype == "" {
				qtype = "A"
			}

			cfg, err := config.Load()
			if err != nil {
				c := config.DefaultConfig
				cfg = &c
			}
			client := loadClient(cfg)
			resp, err := client.QueryDNS(domain, qtype)
			if err != nil {
				return wrapNetworkError(fmt.Errorf("%s: %w", i18n.T("cli.dns.query.err_query"), err))
			}

			out := cmd.OutOrStdout()
			if out == nil {
				out = os.Stdout
			}
			return renderDNSQueryResult(out, domain, qtype, resp, format)
		},
	}
	queryCmd.Flags().StringVarP(&recordType, "type", "t", "A", "")
	queryCmd.Flags().StringVarP(&outputRaw, "output", "o", string(outputFormatPlain), "")
	dnsCmd.AddCommand(queryCmd)
	return dnsCmd
}

func buildDNSQueryOutput(domain, recordType string, resp *model.DNSQueryResponse) dnsQueryOutput {
	out := dnsQueryOutput{
		Domain:     domain,
		RecordType: recordType,
		Answers:    []dnsAnswerOutput{},
	}
	if resp == nil {
		return out
	}
	out.Status = resp.Status
	out.StatusName = model.DNSStatusName(resp.Status)
	out.HasFakeIP = resp.HasFakeIP()
	for _, ans := range resp.Answer {
		tag := model.ClassifyDNSIP(ans.Data)
		out.Answers = append(out.Answers, dnsAnswerOutput{
			Name:     ans.Name,
			Type:     ans.Type,
			TypeName: model.DNSRecordTypeName(ans.Type),
			TTL:      ans.TTL,
			Data:     ans.Data,
			FakeIP:   tag == "fake-ip",
			Tag:      tag,
		})
	}
	return out
}

func renderDNSQueryResult(w io.Writer, domain, recordType string, resp *model.DNSQueryResponse, format outputFormat) error {
	payload := buildDNSQueryOutput(domain, recordType, resp)
	switch format {
	case outputFormatJSON:
		return writeJSON(w, payload)
	case outputFormatTable:
		_, _ = fmt.Fprintf(w, "%s (%s)  %s: %s", payload.Domain, payload.RecordType, i18n.T("conns.dns.status"), payload.StatusName)
		if payload.HasFakeIP {
			_, _ = fmt.Fprintf(w, "  [Fake-IP]")
		}
		_, _ = fmt.Fprintln(w)
		if len(payload.Answers) == 0 {
			_, _ = fmt.Fprintln(w, i18n.T("conns.dns.no_answers"))
			return nil
		}
		rows := [][5]string{{
			i18n.T("conns.dns.col_name"),
			i18n.T("conns.dns.col_type"),
			i18n.T("conns.dns.col_ttl"),
			i18n.T("conns.dns.col_data"),
			i18n.T("conns.dns.col_tag"),
		}}
		for _, a := range payload.Answers {
			tag := a.Tag
			if tag == "" {
				tag = "-"
			}
			rows = append(rows, [5]string{a.Name, a.TypeName, fmt.Sprintf("%ds", a.TTL), a.Data, tag})
		}
		var widths [4]int
		for _, row := range rows {
			for i := range widths {
				widths[i] = max(widths[i], runewidth.StringWidth(row[i]))
			}
		}
		for _, row := range rows {
			for i := range widths {
				row[i] = padDisplayRight(row[i], widths[i])
			}
			if _, err := fmt.Fprintln(w, strings.Join(row[:], "  ")); err != nil {
				return err
			}
		}
		return nil
	case outputFormatPlain:
		header := fmt.Sprintf("%s (%s) %s", payload.Domain, payload.RecordType, payload.StatusName)
		if payload.HasFakeIP {
			header += " [Fake-IP]"
		}
		_, _ = fmt.Fprintln(w, header)
		if len(payload.Answers) == 0 {
			_, _ = fmt.Fprintln(w, i18n.T("conns.dns.no_answers"))
			return nil
		}
		for _, a := range payload.Answers {
			line := fmt.Sprintf("  %s %s %ds %s", a.Name, a.TypeName, a.TTL, a.Data)
			if a.Tag != "" {
				line += " [" + a.Tag + "]"
			}
			_, _ = fmt.Fprintln(w, line)
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %q", format)
	}
}
