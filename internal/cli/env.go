package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/cobra"
)

func newEnvCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:     "env [on|off]",
		Hidden:  true,
		Example: "  eval \"$(mihosh env on)\"\n  eval \"$(mihosh env off)\"\n  mihosh env --output json",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 || len(args) == 1 && args[0] != "on" && args[0] != "off" {
				return wrapParameterError(errors.New(i18n.T("cli.env.invalid_action")))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(output)
			if err != nil {
				return wrapParameterError(err)
			}
			cfg, err := config.Load()
			if errors.Is(err, config.ErrConfigNotFound) {
				defaults := config.DefaultConfig
				cfg, err = &defaults, nil
			}
			if err != nil {
				return wrapConfigError(err)
			}
			if len(args) == 1 {
				cfg.ShellProxy = args[0] == "on"
				startup, err := service.SaveShellProxy(cfg.ShellProxy)
				if err != nil {
					return wrapConfigError(err)
				}
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), i18n.Tf("core.shell.source_hint", service.ShellProxySourceCommand(startup))); err != nil {
					return err
				}
			}
			root, err := config.GetConfigDir()
			if err != nil {
				return wrapConfigError(err)
			}
			executable, err := os.Executable()
			if err != nil {
				return err
			}
			path := filepath.Join(root, "proxy.sh")
			script := service.ShellProxyScript(cfg.ShellProxy, executable, root)
			switch format {
			case outputFormatJSON:
				return writeJSON(cmd.OutOrStdout(), struct {
					Enabled    bool   `json:"enabled"`
					ScriptPath string `json:"script_path"`
					Script     string `json:"script"`
				}{cfg.ShellProxy, path, script})
			case outputFormatTable:
				w := newTabWriter(cmd.OutOrStdout())
				if _, err := fmt.Fprintf(w, "%s\t%t\n%s\t%s\n", i18n.T("cli.env.enabled"), cfg.ShellProxy, i18n.T("cli.env.script"), path); err != nil {
					return err
				}
				return w.Flush()
			default:
				_, err := fmt.Fprint(cmd.OutOrStdout(), script)
				return err
			}
		},
	}
	cmd.Flags().StringVar(&output, "output", "plain", "plain|table|json")
	return cmd
}
