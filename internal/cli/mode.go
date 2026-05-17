package cli

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/spf13/cobra"
)

var modeCmd = &cobra.Command{
	Use:   "mode [rule|global|direct]",
	Short: "获取或设置代理模式",
	Long:  "显示当前运行的代理模式，或通过提供 'rule', 'global', 或 'direct' 来更新它。",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return wrapConfigError(fmt.Errorf("加载配置失败: %w", err))
		}

		client := api.NewClient(cfg)

		if len(args) == 0 {
			// Get configs
			configs, err := client.GetConfigs()
			if err != nil {
				return wrapNetworkError(fmt.Errorf("获取配置模式失败: %w", err))
			}
			fmt.Printf("当前模式: %s\n", configs.Mode)
			return nil
		}

		// Set mode
		mode := strings.ToLower(args[0])
		if mode != "rule" && mode != "global" && mode != "direct" {
			return wrapParameterError(fmt.Errorf("未知的模式: %s, 仅支持 rule, global, direct", mode))
		}

		err = client.UpdateConfig(model.UpdateConfigRequest{Mode: mode})
		if err != nil {
			return wrapNetworkError(fmt.Errorf("更新配置模式失败: %w", err))
		}

		fmt.Printf("成功切换模式为: %s\n", mode)
		return nil
	},
}
