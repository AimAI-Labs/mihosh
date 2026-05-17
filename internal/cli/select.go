package cli

import (
	"fmt"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/spf13/cobra"
)

var selectCmd = &cobra.Command{
	Use:   "select <group> <proxy>",
	Short: "切换节点",
	Long:  `切换指定策略组到目标节点。`,
	Example: `  mihosh select Proxy "HK 01"
  mihosh select Auto SG-BGP`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return wrapConfigError(fmt.Errorf("加载配置失败: %w", err))
		}

		client := api.NewClient(cfg)
		proxySvc := service.NewProxyService(client, cfg.TestURL, cfg.Timeout)

		if err := proxySvc.SelectProxy(args[0], args[1]); err != nil {
			return wrapNetworkError(fmt.Errorf("切换节点失败: %w", err))
		}

		fmt.Printf("✓ 已将策略组 '%s' 切换到节点 '%s'\n", args[0], args[1])
		return nil
	},
}
