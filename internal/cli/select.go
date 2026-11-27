package cli

import (
	"fmt"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/cobra"
)

var selectCmd = newSelectCommand()

func newSelectCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use: "select <group> <proxy>",
		Example: `  mihosh select Proxy "HK 01"
  mihosh select Auto SG-BGP`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(output)
			if err != nil {
				return wrapParameterError(err)
			}
			cfg, err := loadConfigOrDefault()
			if err != nil {
				return wrapConfigError(fmt.Errorf(i18n.T("cli.select.err_load_config")+": %w", err))
			}

			client := loadClient(cfg)
			proxySvc := service.NewProxyService(client, cfg.TestURL, cfg.Timeout)

			if err := proxySvc.SelectProxy(args[0], args[1]); err != nil {
				return wrapNetworkError(fmt.Errorf(i18n.T("cli.select.err_select")+": %w", err))
			}

			return writeFieldResult(cmd.OutOrStdout(), format, i18n.Tf("cli.select.success", args[0], args[1]),
				outputField{"group", args[0]}, outputField{"proxy", args[1]})
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "plain", "plain|table|json")
	return cmd
}
