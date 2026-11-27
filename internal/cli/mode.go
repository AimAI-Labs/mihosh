package cli

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/cobra"
)

var modeCmd = &cobra.Command{
	Use: "mode [rule|global|direct]",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfigOrDefault()
		if err != nil {
			return wrapConfigError(fmt.Errorf(i18n.T("cli.mode.err_load_config")+": %w", err))
		}

		client := loadClient(cfg)

		if len(args) == 0 {
			// Get configs
			configs, err := client.GetConfigs()
			if err != nil {
				return wrapNetworkError(fmt.Errorf(i18n.T("cli.mode.err_get")+": %w", err))
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cli.mode.current", configs.Mode)); err != nil {
				return err
			}
			return nil
		}

		// Set mode
		mode := strings.ToLower(args[0])
		if mode != "rule" && mode != "global" && mode != "direct" {
			return wrapParameterError(fmt.Errorf(i18n.T("cli.mode.err_unknown"), mode))
		}

		err = client.UpdateConfig(model.UpdateConfigRequest{Mode: mode})
		if err != nil {
			return wrapNetworkError(fmt.Errorf(i18n.T("cli.mode.err_set")+": %w", err))
		}

		if _, err := fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cli.mode.switched", mode)); err != nil {
			return err
		}
		return nil
	},
}
