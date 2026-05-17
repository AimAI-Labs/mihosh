package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/spf13/cobra"
)

var connectionsOutput string

var connectionsCmd = &cobra.Command{
	Use:   "connections [--output json|table|plain]",
	Short: "查看当前连接（支持多种输出格式）",
	Long: `查看当前活跃连接和流量统计。

可通过 --output 选择输出格式：
  plain  人类可读文本（默认）
  table  表格输出
  json   结构化 JSON 输出`,
	Example: `  mihosh connections
  mihosh connections --output table
  mihosh connections --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := parseOutputFormat(connectionsOutput)
		if err != nil {
			return wrapParameterError(err)
		}

		cfg, err := config.Load()
		if err != nil {
			return wrapConfigError(fmt.Errorf("加载配置失败: %w", err))
		}

		client := api.NewClient(cfg)
		connSvc := service.NewConnectionService(client)

		conns, err := connSvc.GetConnections()
		if err != nil {
			return wrapNetworkError(fmt.Errorf("获取连接失败: %w", err))
		}

		if err := renderConnections(os.Stdout, *conns, format); err != nil {
			return fmt.Errorf("渲染输出失败: %w", err)
		}
		return nil
	},
}

func init() {
	connectionsCmd.Flags().StringVar(&connectionsOutput, "output", string(outputFormatPlain), "输出格式: json|table|plain")
}

func renderConnections(w io.Writer, conns model.ConnectionsResponse, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return renderConnectionsJSON(w, conns)
	case outputFormatTable:
		return renderConnectionsTable(w, conns)
	case outputFormatPlain:
		renderConnectionsPlain(w, conns)
		return nil
	default:
		return fmt.Errorf("不支持的输出格式: %s", format)
	}
}

type connectionOutputItem struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	SourceIP    string `json:"source_ip"`
	SourcePort  string `json:"source_port"`
	DestIP      string `json:"destination_ip"`
	DestPort    string `json:"destination_port"`
	Chain       string `json:"chain"`
}

func renderConnectionsJSON(w io.Writer, conns model.ConnectionsResponse) error {
	items := make([]connectionOutputItem, 0, len(conns.Connections))
	for _, conn := range conns.Connections {
		chain := "DIRECT"
		if len(conn.Chains) > 0 {
			chain = conn.Chains[len(conn.Chains)-1]
		}
		items = append(items, connectionOutputItem{
			Source:      fmt.Sprintf("%s:%s", conn.Metadata.SourceIP, conn.Metadata.SourcePort),
			Destination: fmt.Sprintf("%s:%s", conn.Metadata.DestinationIP, conn.Metadata.DestinationPort),
			SourceIP:    conn.Metadata.SourceIP,
			SourcePort:  conn.Metadata.SourcePort,
			DestIP:      conn.Metadata.DestinationIP,
			DestPort:    conn.Metadata.DestinationPort,
			Chain:       chain,
		})
	}

	payload := struct {
		ActiveConnections int                    `json:"active_connections"`
		UploadTotal       int64                  `json:"upload_total"`
		DownloadTotal     int64                  `json:"download_total"`
		Connections       []connectionOutputItem `json:"connections"`
	}{
		ActiveConnections: len(conns.Connections),
		UploadTotal:       conns.UploadTotal,
		DownloadTotal:     conns.DownloadTotal,
		Connections:       items,
	}

	return writeJSON(w, payload)
}

func renderConnectionsTable(w io.Writer, conns model.ConnectionsResponse) error {
	fmt.Fprintf(w, "ACTIVE_CONNECTIONS: %d\n", len(conns.Connections))
	fmt.Fprintf(w, "UPLOAD_TOTAL: %s\n", utils.FormatBytes(conns.UploadTotal))
	fmt.Fprintf(w, "DOWNLOAD_TOTAL: %s\n\n", utils.FormatBytes(conns.DownloadTotal))

	tw := newTabWriter(w)
	fmt.Fprintln(tw, "SOURCE\tDESTINATION\tCHAIN")
	for _, conn := range conns.Connections {
		chain := "DIRECT"
		if len(conn.Chains) > 0 {
			chain = conn.Chains[len(conn.Chains)-1]
		}

		fmt.Fprintf(tw, "%s:%s\t%s:%s\t%s\n",
			conn.Metadata.SourceIP,
			conn.Metadata.SourcePort,
			conn.Metadata.DestinationIP,
			conn.Metadata.DestinationPort,
			chain,
		)
	}
	return tw.Flush()
}

func renderConnectionsPlain(w io.Writer, conns model.ConnectionsResponse) {
	fmt.Fprintf(w, "活跃连接数: %d\n", len(conns.Connections))
	fmt.Fprintf(w, "上传总量: %s\n", utils.FormatBytes(conns.UploadTotal))
	fmt.Fprintf(w, "下载总量: %s\n", utils.FormatBytes(conns.DownloadTotal))
	fmt.Fprintln(w, "\n连接列表:")

	for _, conn := range conns.Connections {
		chain := "DIRECT"
		if len(conn.Chains) > 0 {
			chain = conn.Chains[len(conn.Chains)-1]
		}
		fmt.Fprintf(w, "  %s:%s -> %s:%s [%s]\n",
			conn.Metadata.SourceIP,
			conn.Metadata.SourcePort,
			conn.Metadata.DestinationIP,
			conn.Metadata.DestinationPort,
			chain,
		)
	}
}
