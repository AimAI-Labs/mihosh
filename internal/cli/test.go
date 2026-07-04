package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

type testAction string

const (
	actionCurrent testAction = "current"
	actionNode    testAction = "node"
	actionGroup   testAction = "group"
)

var (
	testOutput      string
	testGroupOutput string
)

var testCmd = &cobra.Command{
	Use:   "test [node <节点名> | group <策略组名>] [--output json|table|plain]",
	Example: `  mihosh test
  mihosh test --output json
  mihosh test node HK --output table
  mihosh test group Auto --output json`,
	Args: func(cmd *cobra.Command, args []string) error {
		_, _, err := resolveTestAction(args)
		if err != nil {
			return wrapParameterError(err)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := parseOutputFormat(testOutput)
		if err != nil {
			return wrapParameterError(err)
		}

		cfg, err := config.Load()
		if err != nil {
			return wrapConfigError(fmt.Errorf(i18n.T("cli.test.err_load_config")+": %w", err))
		}

		client := loadClient(cfg)
		proxySvc := service.NewProxyService(client, cfg.TestURL, cfg.Timeout)

		action, target, err := resolveTestAction(args)
		if err != nil {
			return wrapParameterError(err)
		}

		proxyURL := config.MixedPortToProxyURL(config.ResolveMihomoEndpoint().MixedPort)
		if err := runTestAction(os.Stdout, proxySvc, proxyURL, action, target, format); err != nil {
			return wrapNetworkError(err)
		}
		return nil
	},
}

var testGroupCmd = &cobra.Command{
	Use:    "test-group <group> [--output json|table|plain]",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := parseOutputFormat(testGroupOutput)
		if err != nil {
			return wrapParameterError(err)
		}

		cfg, err := config.Load()
		if err != nil {
			return wrapConfigError(fmt.Errorf(i18n.T("cli.test.err_load_config")+": %w", err))
		}

		client := loadClient(cfg)
		proxySvc := service.NewProxyService(client, cfg.TestURL, cfg.Timeout)

		proxyURL := config.MixedPortToProxyURL(config.ResolveMihomoEndpoint().MixedPort)
		if err := runTestAction(os.Stdout, proxySvc, proxyURL, actionGroup, args[0], format); err != nil {
			return wrapNetworkError(err)
		}
		return nil
	},
}

func init() {
	testCmd.Flags().StringVar(&testOutput, "output", string(outputFormatPlain), "")
	testGroupCmd.Flags().StringVar(&testGroupOutput, "output", string(outputFormatPlain), "")
}

func resolveTestAction(args []string) (testAction, string, error) {
	if len(args) == 0 {
		return actionCurrent, "", nil
	}

	if len(args) == 2 {
		switch args[0] {
		case string(actionNode):
			return actionNode, args[1], nil
		case string(actionGroup):
			return actionGroup, args[1], nil
		}
	}

	return "", "", fmt.Errorf("%s", i18n.T("cli.test.err_format"))
}

func runTestAction(w io.Writer, proxySvc *service.ProxyService, proxyAddress string, action testAction, target string, format outputFormat) error {
	switch action {
	case actionCurrent:
		node, found, err := currentSelectedNode(proxySvc)
		if err != nil {
			return fmt.Errorf(i18n.T("cli.test.err_get_current")+": %w", err)
		}
		if !found {
			return renderNoCurrentTestOutput(w, format)
		}

		// 先验证当前选中节点可测速，再保留原本的链路/IP信息输出格式。
		_, err = proxySvc.TestProxyDelay(node)
		if err != nil {
			return fmt.Errorf(i18n.T("cli.test.err_test_delay")+": %w", err)
		}

		chain, err := proxySvc.GetNodeChain()
		if err != nil {
			return fmt.Errorf(i18n.T("cli.test.err_get_chain")+": %w", err)
		}

		ipInfo, err := proxySvc.GetIPInfo(proxyAddress)
		if err != nil {
			return fmt.Errorf(i18n.T("cli.test.err_get_ip")+": %w", err)
		}

		return renderCurrentTestOutput(w, node, chain, ipInfo, format)

	case actionNode:
		delay, err := proxySvc.TestProxyDelay(target)
		if err != nil {
			return fmt.Errorf(i18n.T("cli.test.err_test_delay")+": %w", err)
		}
		return renderNodeTestOutput(w, target, delay, format)

	case actionGroup:
		if err := proxySvc.TestGroupDelay(target); err != nil {
			return fmt.Errorf(i18n.T("cli.test.err_test_group")+": %w", err)
		}
		return renderGroupTestOutput(w, target, format)
	}

	return fmt.Errorf(i18n.T("cli.test.err_unsupported_action"), action)
}

func renderNoCurrentTestOutput(w io.Writer, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(w, map[string]interface{}{
			"action": "current",
			"found":  false,
		})
	case outputFormatTable:
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "KEY\tVALUE")
		fmt.Fprintln(tw, "ACTION\tcurrent")
		fmt.Fprintln(tw, "FOUND\tfalse")
		return tw.Flush()
	case outputFormatPlain:
		fmt.Fprintln(w, i18n.T("cli.test.no_current_node"))
		return nil
	default:
		return fmt.Errorf(i18n.T("cli.test.unsupported_format"), format)
	}
}

func renderCurrentTestOutput(w io.Writer, node string, chain []string, ipInfo *model.IPInfo, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(w, map[string]interface{}{
			"action":  "current",
			"found":   true,
			"node":    node,
			"chain":   chain,
			"ip_info": ipInfo,
		})
	case outputFormatTable:
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "KEY\tVALUE")
		fmt.Fprintf(tw, "ACTION\t%s\n", actionCurrent)
		fmt.Fprintf(tw, "NODE\t%s\n", node)
		fmt.Fprintf(tw, "CHAIN\t%s\n", strings.Join(chain, " -> "))
		fmt.Fprintf(tw, "IP\t%s\n", ipInfo.IP)
		fmt.Fprintf(tw, "COUNTRY\t%s (%s)\n", ipInfo.Country, ipInfo.CountryCode)
		fmt.Fprintf(tw, "CITY\t%s\n", ipInfo.City)
		fmt.Fprintf(tw, "ASN\t%s\n", ipInfo.AS)
		fmt.Fprintf(tw, "ORG\t%s\n", ipInfo.Org)
		return tw.Flush()
	case outputFormatPlain:
		const contentWidth = 50
		labels := []string{i18n.T("cli.test.label_node"), i18n.T("cli.test.label_chain"), i18n.T("cli.test.label_ip"), i18n.T("cli.test.label_country"), i18n.T("cli.test.label_city"), i18n.T("cli.test.label_asn"), i18n.T("cli.test.label_org")}
		labelWidth := maxDisplayWidth(labels)
		if labelWidth < 8 {
			labelWidth = 8
		}

		fmt.Fprintln(w, "┌"+strings.Repeat("─", contentWidth+2)+"┐")
		fmt.Fprintln(w, boxLine(i18n.T("cli.test.label_node"), node, labelWidth, contentWidth))
		fmt.Fprintln(w, boxLine(i18n.T("cli.test.label_chain"), strings.Join(chain, " -> "), labelWidth, contentWidth))
		fmt.Fprintln(w, boxLine(i18n.T("cli.test.label_ip"), ipInfo.IP, labelWidth, contentWidth))
		fmt.Fprintln(w, boxLine(i18n.T("cli.test.label_country"), fmt.Sprintf("%s (%s)", ipInfo.Country, ipInfo.CountryCode), labelWidth, contentWidth))
		fmt.Fprintln(w, boxLine(i18n.T("cli.test.label_city"), ipInfo.City, labelWidth, contentWidth))
		fmt.Fprintln(w, boxLine(i18n.T("cli.test.label_asn"), ipInfo.AS, labelWidth, contentWidth))
		fmt.Fprintln(w, boxLine(i18n.T("cli.test.label_org"), ipInfo.Org, labelWidth, contentWidth))
		fmt.Fprintln(w, "└"+strings.Repeat("─", contentWidth+2)+"┘")
		return nil
	default:
		return fmt.Errorf(i18n.T("cli.test.unsupported_format"), format)
	}
}

func renderNodeTestOutput(w io.Writer, node string, delay int, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(w, map[string]interface{}{
			"action":   "node",
			"node":     node,
			"delay_ms": delay,
		})
	case outputFormatTable:
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "KEY\tVALUE")
		fmt.Fprintf(tw, "ACTION\t%s\n", actionNode)
		fmt.Fprintf(tw, "NODE\t%s\n", node)
		fmt.Fprintf(tw, "DELAY_MS\t%d\n", delay)
		return tw.Flush()
	case outputFormatPlain:
		fmt.Fprintln(w, i18n.Tf("cli.test.success_node", node, delay))
		return nil
	default:
		return fmt.Errorf(i18n.T("cli.test.unsupported_format"), format)
	}
}

func renderGroupTestOutput(w io.Writer, group string, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(w, map[string]interface{}{
			"action": "group",
			"group":  group,
			"status": "completed",
		})
	case outputFormatTable:
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "KEY\tVALUE")
		fmt.Fprintf(tw, "ACTION\t%s\n", actionGroup)
		fmt.Fprintf(tw, "GROUP\t%s\n", group)
		fmt.Fprintln(tw, "STATUS\tcompleted")
		return tw.Flush()
	case outputFormatPlain:
		fmt.Fprintln(w, i18n.Tf("cli.test.success_group", group))
		return nil
	default:
		return fmt.Errorf(i18n.T("cli.test.unsupported_format"), format)
	}
}

func currentSelectedNode(proxySvc *service.ProxyService) (string, bool, error) {
	proxies, err := proxySvc.GetProxies()
	if err != nil {
		return "", false, err
	}
	node, found := resolveCurrentSelectedNode(proxies)
	return node, found, nil
}

func resolveCurrentSelectedNode(proxies map[string]model.Proxy) (string, bool) {
	for _, root := range []string{"GLOBAL", "Proxy"} {
		if _, ok := proxies[root]; !ok {
			continue
		}

		if node, found := resolveLeafFromRoot(proxies, root); found {
			return node, true
		}
	}

	return "", false
}

func resolveLeafFromRoot(proxies map[string]model.Proxy, root string) (string, bool) {
	current := root
	visited := make(map[string]struct{}, len(proxies))

	for {
		if _, seen := visited[current]; seen {
			return "", false
		}
		visited[current] = struct{}{}

		proxy, ok := proxies[current]
		if !ok {
			return "", false
		}

		if proxy.Now == "" {
			// 策略组通常带有 all 字段。没有 Now 且仍是策略组时，视为未选择具体节点。
			if len(proxy.All) > 0 {
				return "", false
			}
			return current, true
		}

		current = proxy.Now
	}
}

func boxLine(label, value string, labelWidth, width int) string {
	left := padDisplayRight(label, labelWidth) + ": "
	leftWidth := runewidth.StringWidth(left)
	valueWidth := width - leftWidth
	if valueWidth < 0 {
		valueWidth = 0
	}

	value = fitText(value, valueWidth)
	padding := width - leftWidth - runewidth.StringWidth(value)
	if padding < 0 {
		padding = 0
	}

	return fmt.Sprintf("│ %s%s%s │", left, value, strings.Repeat(" ", padding))
}

func padDisplayRight(s string, width int) string {
	current := runewidth.StringWidth(s)
	if current >= width {
		return s
	}
	return s + strings.Repeat(" ", width-current)
}

func maxDisplayWidth(items []string) int {
	max := 0
	for _, item := range items {
		w := runewidth.StringWidth(item)
		if w > max {
			max = w
		}
	}
	return max
}

func fitText(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth == 1 {
		return "…"
	}

	rs := []rune(s)
	current := 0
	var out []rune
	for _, r := range rs {
		w := runewidth.RuneWidth(r)
		if current+w > maxWidth-1 {
			break
		}
		out = append(out, r)
		current += w
	}
	return string(out) + "…"
}
