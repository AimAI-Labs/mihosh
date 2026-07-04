package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/spf13/cobra"
)

var statusOutput string

var statusCmd = &cobra.Command{
	Use:   "status [--output json|table|plain]",
	Example: `  mihosh status
  mihosh status --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := parseOutputFormat(statusOutput)
		if err != nil {
			return wrapParameterError(err)
		}

		cfg, err := config.Load()
		if err != nil {
			return wrapConfigError(fmt.Errorf(i18n.T("cli.status.err_load_config")+": %w", err))
		}

		client := loadClient(cfg)

		configs, err := client.GetConfigs()
		if err != nil {
			return wrapNetworkError(fmt.Errorf(i18n.T("cli.status.err_get_configs")+": %w", err))
		}

		conns, err := client.GetConnections()
		if err != nil {
			return wrapNetworkError(fmt.Errorf(i18n.T("cli.status.err_get_conns")+": %w", err))
		}

		mem, err := client.GetMemory()
		if err != nil {
			return wrapNetworkError(fmt.Errorf(i18n.T("cli.status.err_get_memory")+": %w", err))
		}

		proxies, err := client.GetProxies()
		if err != nil {
			return wrapNetworkError(fmt.Errorf(i18n.T("cli.status.err_get_proxies")+": %w", err))
		}

		snapshot := buildStatusSnapshot(configs.Mode, *conns, *mem, proxies)
		if err := renderStatus(os.Stdout, snapshot, format); err != nil {
			return fmt.Errorf(i18n.T("cli.status.err_render")+": %w", err)
		}
		return nil
	},
}

func init() {
	statusCmd.Flags().StringVar(&statusOutput, "output", string(outputFormatPlain), "")
}

type statusSelectedGroup struct {
	Name    string `json:"name"`
	Current string `json:"current"`
	Leaf    string `json:"leaf,omitempty"`
}

type statusSnapshot struct {
	Mode              string                `json:"mode"`
	ActiveConnections int                   `json:"active_connections"`
	UploadSpeed       int64                 `json:"upload_speed"`
	DownloadSpeed     int64                 `json:"download_speed"`
	MemoryInuse       int64                 `json:"memory_inuse"`
	MemoryOSLimit     int64                 `json:"memory_oslimit"`
	SelectedGroups    []statusSelectedGroup `json:"selected_groups"`
}

func buildStatusSnapshot(mode string, conns model.ConnectionsResponse, mem model.MemoryResponse, proxies map[string]model.Proxy) statusSnapshot {
	var uploadSpeed int64
	var downloadSpeed int64
	for _, conn := range conns.Connections {
		uploadSpeed += conn.UploadSpeed
		downloadSpeed += conn.DownloadSpeed
	}

	return statusSnapshot{
		Mode:              mode,
		ActiveConnections: len(conns.Connections),
		UploadSpeed:       uploadSpeed,
		DownloadSpeed:     downloadSpeed,
		MemoryInuse:       mem.Inuse,
		MemoryOSLimit:     mem.OSLimit,
		SelectedGroups:    resolveSelectedGroups(proxies),
	}
}

func resolveSelectedGroups(proxies map[string]model.Proxy) []statusSelectedGroup {
	groups := make([]statusSelectedGroup, 0, 2)
	for _, name := range []string{"GLOBAL", "Proxy"} {
		proxy, ok := proxies[name]
		if !ok {
			continue
		}

		group := statusSelectedGroup{
			Name:    name,
			Current: proxy.Now,
		}
		if leaf, found := resolveLeafFromRoot(proxies, name); found {
			group.Leaf = leaf
		}
		groups = append(groups, group)
	}
	return groups
}

func renderStatus(w io.Writer, snapshot statusSnapshot, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(w, snapshot)
	case outputFormatTable:
		return renderStatusTable(w, snapshot)
	case outputFormatPlain:
		renderStatusPlain(w, snapshot)
		return nil
	default:
		return fmt.Errorf(i18n.T("cli.status.unsupported_format"), format)
	}
}

func renderStatusTable(w io.Writer, snapshot statusSnapshot) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "KEY\tVALUE")
	fmt.Fprintf(tw, "MODE\t%s\n", snapshot.Mode)
	fmt.Fprintf(tw, "ACTIVE_CONNECTIONS\t%d\n", snapshot.ActiveConnections)
	fmt.Fprintf(tw, "UPLOAD_SPEED\t%s/s\n", utils.FormatBytes(snapshot.UploadSpeed))
	fmt.Fprintf(tw, "DOWNLOAD_SPEED\t%s/s\n", utils.FormatBytes(snapshot.DownloadSpeed))
	fmt.Fprintf(tw, "MEMORY\t%s / %s\n", utils.FormatBytes(snapshot.MemoryInuse), utils.FormatBytes(snapshot.MemoryOSLimit))
	for _, group := range snapshot.SelectedGroups {
		fmt.Fprintf(tw, "GROUP_%s\t%s\n", group.Name, formatSelectedGroupValue(group))
	}
	return tw.Flush()
}

func renderStatusPlain(w io.Writer, snapshot statusSnapshot) {
	fmt.Fprintf(w, "%s: %s\n", i18n.T("cli.status.label_mode"), snapshot.Mode)
	fmt.Fprintf(w, "%s: %d\n", i18n.T("cli.status.label_active_conns"), snapshot.ActiveConnections)
	fmt.Fprintf(w, "%s: %s/s\n", i18n.T("cli.status.label_upload_speed"), utils.FormatBytes(snapshot.UploadSpeed))
	fmt.Fprintf(w, "%s: %s/s\n", i18n.T("cli.status.label_download_speed"), utils.FormatBytes(snapshot.DownloadSpeed))
	fmt.Fprintf(w, "%s: %s / %s\n", i18n.T("cli.status.label_memory"), utils.FormatBytes(snapshot.MemoryInuse), utils.FormatBytes(snapshot.MemoryOSLimit))
	if len(snapshot.SelectedGroups) == 0 {
		return
	}

	fmt.Fprintln(w, i18n.T("cli.status.label_selected_groups"))
	for _, group := range snapshot.SelectedGroups {
		fmt.Fprintf(w, "  %s: %s\n", group.Name, formatSelectedGroupValue(group))
	}
}

func formatSelectedGroupValue(group statusSelectedGroup) string {
	if group.Leaf == "" || group.Leaf == group.Current {
		return group.Current
	}
	return fmt.Sprintf("%s -> %s", group.Current, group.Leaf)
}
