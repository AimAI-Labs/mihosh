package cli

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/cobra"
)

var modeCmd = newModeCommand()

func newModeCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:  "mode [rule|global|direct]",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(output)
			if err != nil {
				return wrapParameterError(err)
			}
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
				return writeFieldResult(cmd.OutOrStdout(), format, i18n.Tf("cli.mode.current", configs.Mode),
					outputField{"mode", configs.Mode})
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

			return writeFieldResult(cmd.OutOrStdout(), format, i18n.Tf("cli.mode.switched", mode),
				outputField{"mode", mode})
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "plain", "plain|table|json")
	return cmd
}
