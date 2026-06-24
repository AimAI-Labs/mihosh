package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "mihosh",
	Short:         i18n.T("cli.root.short"),
	Long:          i18n.T("cli.root.long"),
	Version:       model.Version,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 默认行为：启动TUI界面。
		// 配置缺失（含首次启动）直接用默认值继续，不在启动期写文件；
		// 连接信息由 mihomo 配置文件自动发现。
		cfg, err := config.Load()
		if err == nil {
			i18n.SetLanguageOverride(cfg.Language)
		} else {
			cfg = &config.DefaultConfig
		}

		endpoint := config.ResolveMihomoEndpoint()
		client := api.NewClient(endpoint, cfg.Timeout)
		m := tui.NewModel(client, cfg.TestURL, cfg.Timeout)

		p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf(i18n.T("cli.root.err_start")+": %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(selectCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(testGroupCmd)
	rootCmd.AddCommand(connectionsCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(modeCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(proxyOnCmd)
	rootCmd.AddCommand(proxyOffCmd)
}

// Execute 执行命令
func Execute() {
	os.Exit(executeRootCommand(rootCmd, os.Stderr))
}

func executeRootCommand(root *cobra.Command, stderr io.Writer) int {
	i18n.Init()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, renderCommandError(err))
		return exitCodeForError(err)
	}
	return exitCodeOK
}

// loadClient 统一构造 API Client：从 mihomo 配置文件解析连接信息，
// 超时取自 Mihosh 配置（加载失败则回退默认）。供所有 CLI 子命令复用，
// 避免各处重复 api.NewClient + endpoint 解析。
func loadClient(cfg *config.Config) *api.Client {
	return api.NewClient(config.ResolveMihomoEndpoint(), cfg.Timeout)
}
