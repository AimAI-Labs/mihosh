package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/cobra"
)

func newActivateCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:       "activate <bash|zsh>",
		ValidArgs: []string{"bash", "zsh"},
		Args:      cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(output)
			if err != nil {
				return wrapParameterError(err)
			}
			if args[0] != "bash" && args[0] != "zsh" {
				return wrapParameterError(errors.New(i18n.T("cli.activate.invalid_shell")))
			}
			cfg, err := config.Load()
			if errors.Is(err, config.ErrConfigNotFound) {
				cfg, err = &config.DefaultConfig, nil
			}
			if err != nil {
				return wrapConfigError(err)
			}
			root, err := config.GetConfigDir()
			if err != nil {
				return wrapConfigError(err)
			}
			executable, err := os.Executable()
			if err != nil {
				return err
			}
			script, err := service.ShellActivationScript(args[0], cfg.ShellProxy, executable, root)
			if err != nil {
				return wrapParameterError(err)
			}
			switch format {
			case outputFormatJSON:
				return writeJSON(cmd.OutOrStdout(), struct {
					Shell   string `json:"shell"`
					Enabled bool   `json:"enabled"`
					Script  string `json:"script"`
				}{args[0], cfg.ShellProxy, script})
			case outputFormatTable:
				return writeShellTable(cmd, i18n.T("cli.activate.shell"), args[0], cfg.ShellProxy)
			default:
				_, err = fmt.Fprint(cmd.OutOrStdout(), script)
				return err
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "plain", "plain|table|json")
	return cmd
}

func newProxyCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{Use: "proxy"}
	cmd.PersistentFlags().StringVarP(&output, "output", "o", "plain", "plain|table|json")
	for _, action := range []string{"on", "off"} {
		cmd.AddCommand(&cobra.Command{
			Use:  action,
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				format, err := parseOutputFormat(output)
				if err != nil {
					return wrapParameterError(err)
				}
				return runProxyToggle(cmd, action == "on", format)
			},
		})
	}
	cmd.AddCommand(&cobra.Command{
		Use:  "test",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := parseOutputFormat(output)
			if err != nil {
				return wrapParameterError(err)
			}
			return runProxyProbe(cmd, format)
		},
	})
	return cmd
}

func writeShellTable(cmd *cobra.Command, label, value string, enabled bool) error {
	w := newTabWriter(cmd.OutOrStdout())
	if _, err := fmt.Fprintf(w, "%s\t%s\n%s\t%t\n", label, value, i18n.T("cli.env.enabled"), enabled); err != nil {
		return err
	}
	return w.Flush()
}

func runProxyToggle(cmd *cobra.Command, enabled bool, format outputFormat) error {
	if format == outputFormatPlain {
		if enabled {
			return runProxyOn(cmd, nil)
		}
		return runProxyOff(cmd, nil)
	}
	var addr string
	var script strings.Builder
	if enabled {
		var err error
		addr, err = configuredProxyAddress()
		if err != nil {
			return err
		}
		renderProxyOnStatements(&script, addr, noProxyBypass)
	} else {
		renderProxyOffStatements(&script)
	}
	if format == outputFormatJSON {
		return writeJSON(cmd.OutOrStdout(), struct {
			Enabled bool   `json:"enabled"`
			Address string `json:"address,omitempty"`
			Script  string `json:"script"`
		}{enabled, addr, script.String()})
	}
	return writeShellTable(cmd, i18n.T("cli.proxy.address"), addr, enabled)
}

func runProxyProbe(cmd *cobra.Command, format outputFormat) error {
	addr, err := configuredProxyAddress()
	if err != nil {
		return err
	}
	timeout := config.DefaultConfig.Timeout
	if cfg, err := config.Load(); err == nil && cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	results, err := service.TestProxySources(cmd.Context(), addr, time.Duration(timeout)*time.Millisecond)
	if err != nil {
		return wrapNetworkError(err)
	}
	if err := writeProxyProbeResults(cmd, results, format); err != nil {
		return err
	}
	for _, result := range results {
		if result.Error == "" {
			return nil
		}
	}
	return wrapNetworkError(errors.New(i18n.T("cli.proxy.test_failed")))
}

func writeProxyProbeResults(cmd *cobra.Command, results []service.ProxyProbeResult, format outputFormat) error {
	if format == outputFormatJSON {
		return writeJSON(cmd.OutOrStdout(), results)
	}
	w := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintf(w, "%s\tIP\t%s\t%s\tASN\t%s\t%s\t%s\n",
		i18n.T("cli.proxy.source"), i18n.T("cli.proxy.country"), i18n.T("cli.proxy.city"),
		i18n.T("cli.proxy.org"), i18n.T("cli.proxy.latency"), i18n.T("cli.proxy.error"))
	for _, result := range results {
		asn := ""
		if result.ASN != 0 {
			asn = fmt.Sprintf("AS%d", result.ASN)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d ms\t%s\n",
			result.Source, result.IP, result.Country, result.City, asn, result.Org, result.LatencyMS, result.Error)
	}
	return w.Flush()
}
