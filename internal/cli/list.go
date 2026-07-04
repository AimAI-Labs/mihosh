package cli

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/cobra"
)

var listOutput string

var listCmd = &cobra.Command{
	Use:   "list [--output json|table|plain]",
	Example: `  mihosh list
  mihosh list --output table
  mihosh list --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := parseOutputFormat(listOutput)
		if err != nil {
			return wrapParameterError(err)
		}

		cfg, err := config.Load()
		if err != nil {
			return wrapConfigError(fmt.Errorf(i18n.T("cli.list.err_load_config")+": %w", err))
		}

		client := loadClient(cfg)
		proxySvc := service.NewProxyService(client, cfg.TestURL, cfg.Timeout)

		groups, orderedNames, err := proxySvc.GetGroups()
		if err != nil {
			return wrapNetworkError(fmt.Errorf(i18n.T("cli.list.err_get_groups")+": %w", err))
		}

		proxiesMap, err := proxySvc.GetProxies()
		if err != nil {
			return wrapNetworkError(fmt.Errorf(i18n.T("cli.list.err_get_proxies")+": %w", err))
		}

		if err := renderGroupList(os.Stdout, groups, orderedNames, proxiesMap, format); err != nil {
			return fmt.Errorf(i18n.T("cli.list.err_render")+": %w", err)
		}
		return nil
	},
}

func init() {
	listCmd.Flags().StringVar(&listOutput, "output", string(outputFormatPlain), "")
}

func renderGroupList(w io.Writer, groups map[string]model.Group, orderedNames []string, proxiesMap map[string]model.Proxy, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return renderGroupListJSON(w, groups, orderedNames, proxiesMap)
	case outputFormatTable:
		return renderGroupListTable(w, groups, orderedNames, proxiesMap)
	case outputFormatPlain:
		renderGroupListPlain(w, groups, orderedNames, proxiesMap)
		return nil
	default:
		return fmt.Errorf(i18n.T("cli.list.unsupported_format"), format)
	}
}

type listProxyOutput struct {
	Name     string `json:"name"`
	DelayMS  *int   `json:"delay_ms,omitempty"`
	Selected bool   `json:"selected"`
}

type listGroupOutput struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Current string            `json:"current"`
	Proxies []listProxyOutput `json:"proxies"`
}

func renderGroupListJSON(w io.Writer, groups map[string]model.Group, orderedNames []string, proxiesMap map[string]model.Proxy) error {
	ordered := resolveGroupOrder(groups, orderedNames)
	payload := struct {
		Groups []listGroupOutput `json:"groups"`
	}{
		Groups: make([]listGroupOutput, 0, len(ordered)),
	}

	for _, groupName := range ordered {
		group := groups[groupName]
		groupOut := listGroupOutput{
			Name:    group.Name,
			Type:    group.Type,
			Current: group.Now,
			Proxies: make([]listProxyOutput, 0, len(group.All)),
		}

		for _, proxyName := range group.All {
			proxyOut := listProxyOutput{
				Name:     proxyName,
				Selected: proxyName == group.Now,
			}
			if proxy, ok := proxiesMap[proxyName]; ok && len(proxy.History) > 0 {
				lastDelay := proxy.History[len(proxy.History)-1].Delay
				if lastDelay > 0 {
					delay := lastDelay
					proxyOut.DelayMS = &delay
				}
			}
			groupOut.Proxies = append(groupOut.Proxies, proxyOut)
		}

		payload.Groups = append(payload.Groups, groupOut)
	}

	return writeJSON(w, payload)
}

func renderGroupListTable(w io.Writer, groups map[string]model.Group, orderedNames []string, proxiesMap map[string]model.Proxy) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "GROUP\tTYPE\tCURRENT\tPROXY\tDELAY\tSELECTED")

	for _, groupName := range resolveGroupOrder(groups, orderedNames) {
		group := groups[groupName]
		if len(group.All) == 0 {
			fmt.Fprintf(tw, "%s\t%s\t%s\t-\t-\t-\n", group.Name, group.Type, group.Now)
			continue
		}

		for _, proxyName := range group.All {
			delay := "-"
			if proxy, ok := proxiesMap[proxyName]; ok && len(proxy.History) > 0 {
				lastDelay := proxy.History[len(proxy.History)-1].Delay
				if lastDelay > 0 {
					delay = fmt.Sprintf("%dms", lastDelay)
				}
			}
			selected := ""
			if proxyName == group.Now {
				selected = "yes"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", group.Name, group.Type, group.Now, proxyName, delay, selected)
		}
	}

	return tw.Flush()
}

func renderGroupListPlain(w io.Writer, groups map[string]model.Group, orderedNames []string, proxiesMap map[string]model.Proxy) {
	fmt.Fprintln(w, i18n.T("cli.list.label_groups"))
	for _, groupName := range resolveGroupOrder(groups, orderedNames) {
		group := groups[groupName]
		fmt.Fprintf(w, "\n[%s] %s (%s: %s)\n", group.Type, group.Name, i18n.T("cli.list.label_current"), group.Now)
		for _, proxyName := range group.All {
			proxy, ok := proxiesMap[proxyName]
			delay := ""
			if ok && len(proxy.History) > 0 {
				lastDelay := proxy.History[len(proxy.History)-1].Delay
				if lastDelay > 0 {
					delay = fmt.Sprintf(" (%dms)", lastDelay)
				}
			}
			marker := ""
			if proxyName == group.Now {
				marker = " ✓"
			}
			fmt.Fprintf(w, "  - %s%s%s\n", proxyName, delay, marker)
		}
	}
}

func resolveGroupOrder(groups map[string]model.Group, orderedNames []string) []string {
	if len(groups) == 0 {
		return nil
	}

	if len(orderedNames) == 0 {
		names := make([]string, 0, len(groups))
		for name := range groups {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}

	names := make([]string, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))

	for _, name := range orderedNames {
		if _, ok := groups[name]; ok {
			names = append(names, name)
			seen[name] = struct{}{}
		}
	}

	if len(names) == len(groups) {
		return names
	}

	extras := make([]string, 0, len(groups)-len(names))
	for name := range groups {
		if _, ok := seen[name]; !ok {
			extras = append(extras, name)
		}
	}
	sort.Strings(extras)

	return append(names, extras...)
}
