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
	localizeCommands()
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

func localizeCommands() {
	rootCmd.Short = i18n.T("cli.root.short")
	rootCmd.Long = i18n.T("cli.root.long")

	statusCmd.Short = i18n.T("cli.status.short")
	statusCmd.Long = i18n.T("cli.status.long")
	
	configCmd.Short = i18n.T("cli.config.short")
	configShowCmd.Short = i18n.T("cli.config.show.short")
	configShowCmd.Long = i18n.T("cli.config.show.long")
	configSetCmd.Short = i18n.T("cli.config.set.short")
	configSetCmd.Long = i18n.T("cli.config.set.long")
	configEditCmd.Short = i18n.T("cli.config.edit.short")
	configEditCmd.Long = i18n.T("cli.config.edit.long")

	connectionsCmd.Short = i18n.T("cli.connections.short")
	connectionsCmd.Long = i18n.T("cli.connections.long")

	listCmd.Short = i18n.T("cli.list.short")
	listCmd.Long = i18n.T("cli.list.long")

	modeCmd.Short = i18n.T("cli.mode.short")
	modeCmd.Long = i18n.T("cli.mode.long")

	selectCmd.Short = i18n.T("cli.select.short")
	selectCmd.Long = i18n.T("cli.select.long")

	testCmd.Short = i18n.T("cli.test.short")
	testCmd.Long = i18n.T("cli.test.long")
	testGroupCmd.Short = i18n.T("cli.test_group.short")

	versionCmd.Short = i18n.T("cli.version.short")
	versionCmd.Long = i18n.T("cli.version.long")

	// cobra generated completion command
	if completionCmd, _, err := rootCmd.Find([]string{"completion"}); err == nil && completionCmd != nil {
		completionCmd.Short = i18n.T("cli.completion.short")
		completionCmd.Long = i18n.T("cli.completion.long")
	}

	serviceCmd.Short = i18n.T("cli.service.short")
	serviceCmd.Long = i18n.T("cli.service.long")
	if c, _, err := serviceCmd.Find([]string{"logs"}); err == nil && c != nil {
		c.Short = i18n.T("cli.service.logs.short")
		if f := c.Flags().Lookup("lines"); f != nil { f.Usage = i18n.T("cli.service.logs.flag_lines") }
		if f := c.Flags().Lookup("follow"); f != nil { f.Usage = i18n.T("cli.service.logs.flag_follow") }
	}
	for _, action := range []string{"start", "stop", "restart", "enable", "disable", "status"} {
		if c, _, err := serviceCmd.Find([]string{action}); err == nil && c != nil {
			if action == "status" {
				c.Short = i18n.T("cli.service.action_status")
			} else {
				c.Short = i18n.T("cli.service.action_" + action)
			}
		}
	}

	doctorCmd.Short = i18n.T("cli.doctor.short")
	doctorCmd.Long = i18n.T("cli.doctor.long")

	proxyOnCmd.Short = i18n.T("cli.proxy.on.short")
	proxyOnCmd.Long = i18n.T("cli.proxy.on.long")
	proxyOffCmd.Short = i18n.T("cli.proxy.off.short")
	proxyOffCmd.Long = i18n.T("cli.proxy.off.long")

	// Set flag descriptions
	if f := statusCmd.Flags().Lookup("output"); f != nil { f.Usage = i18n.T("cli.status.flag_output") }
	if f := configShowCmd.Flags().Lookup("output"); f != nil { f.Usage = i18n.T("cli.config.show.flag_output") }
	if f := configEditCmd.Flags().Lookup("editor"); f != nil { f.Usage = i18n.T("cli.config.edit.flag_editor") }
	if f := configEditCmd.Flags().Lookup("path"); f != nil { f.Usage = i18n.T("cli.config.edit.flag_path") }
	if f := connectionsCmd.Flags().Lookup("output"); f != nil { f.Usage = i18n.T("cli.connections.flag_output") }
	if f := listCmd.Flags().Lookup("output"); f != nil { f.Usage = i18n.T("cli.list.flag_output") }
	if f := testCmd.Flags().Lookup("output"); f != nil { f.Usage = i18n.T("cli.test.flag_output") }
	if f := versionCmd.Flags().Lookup("output"); f != nil { f.Usage = i18n.T("cli.version.flag_output") }
	if f := serviceCmd.PersistentFlags().Lookup("unit"); f != nil { f.Usage = i18n.T("cli.service.flag_unit") }
	if f := doctorCmd.Flags().Lookup("output"); f != nil { f.Usage = i18n.T("cli.doctor.flag_output") }
}
