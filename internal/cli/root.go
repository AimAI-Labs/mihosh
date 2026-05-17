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
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "mihosh",
	Short:         "Mihosh - Mihomo 终端管理工具",
	Long:          `一个功能完整的 mihomo 终端命令行工具，支持节点切换、测速等操作`,
	Version:       Version,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 默认行为：启动TUI界面
		cfg, err := config.Load()
		if err != nil {
			if !errors.Is(err, config.ErrConfigNotFound) {
				return wrapConfigError(fmt.Errorf("加载配置失败: %w", err))
			}

			// 友好的首次使用引导
			configSvc := service.NewConfigService()
			if err := configSvc.InitConfig(); err != nil {
				return wrapConfigError(fmt.Errorf("配置初始化失败: %w", err))
			}

			// 重新加载配置
			cfg, err = config.Load()
			if err != nil {
				return wrapConfigError(fmt.Errorf("加载配置失败: %w", err))
			}
		}

		client := api.NewClient(cfg)
		model := tui.NewModel(client, cfg.TestURL, cfg.Timeout)

		p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("启动失败: %w", err)
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
}

// Execute 执行命令
func Execute() {
	os.Exit(executeRootCommand(rootCmd, os.Stderr))
}

func executeRootCommand(root *cobra.Command, stderr io.Writer) int {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, renderCommandError(err))
		return exitCodeForError(err)
	}
	return exitCodeOK
}
