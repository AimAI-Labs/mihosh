package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
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
	Version:       Version,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 默认行为：启动TUI界面
		cfg, err := config.Load()
		if err == nil {
			i18n.SetLanguageOverride(cfg.Language)
		}
		if err != nil {
			if !errors.Is(err, config.ErrConfigNotFound) {
				return wrapConfigError(fmt.Errorf(i18n.T("cli.root.err_load_config")+": %w", err))
			}

			// 友好的首次使用引导
			configSvc := service.NewConfigService()
			if err := configSvc.InitConfig(); err != nil {
				return wrapConfigError(fmt.Errorf(i18n.T("cli.root.err_init_config")+": %w", err))
			}

			// 重新加载配置
			cfg, err = config.Load()
			if err != nil {
				return wrapConfigError(fmt.Errorf(i18n.T("cli.root.err_load_config")+": %w", err))
			}
		}

		client := api.NewClient(cfg)
		model := tui.NewModel(client, cfg.TestURL, cfg.Timeout)

		p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
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
