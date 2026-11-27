package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/cobra"
)

func newUninstallCommand() *cobra.Command {
	return newUninstallCommandWithService(func() *service.UninstallService {
		return service.NewUninstallService(nil)
	})
}

func newUninstallCommandWithService(svcFactory func() *service.UninstallService) *cobra.Command {
	var (
		keepConfig bool
		yes        bool
		dryRun     bool
		output     string
	)
	cmd := &cobra.Command{
		Use:           "uninstall",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args: func(cmd *cobra.Command, args []string) error {
			return wrapParameterError(cobra.NoArgs(cmd, args))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := parseOutputFormat(output)
			if err != nil {
				return wrapParameterError(err)
			}
			svc := svcFactory()
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			plan, err := svc.Plan(ctx, keepConfig)
			if err != nil {
				return wrapGeneralError(err)
			}

			if dryRun {
				res, err := svc.Uninstall(ctx, service.UninstallOptions{
					KeepConfig: keepConfig,
					DryRun:     true,
				})
				if err != nil {
					return wrapGeneralError(err)
				}
				return writeUninstallResult(cmd.OutOrStdout(), format, res)
			}

			if !yes {
				if format == outputFormatJSON {
					return wrapParameterError(errors.New(i18n.T("cli.uninstall.err_yes_required_json")))
				}
				if isNonInteractiveFile(cmd.InOrStdin()) {
					return wrapParameterError(errors.New(i18n.T("cli.uninstall.err_non_interactive")))
				}
				confirmed, nextKeep, err := promptUninstall(cmd, plan, keepConfig, cmd.Flags().Changed("keep-config"))
				if err != nil {
					return wrapGeneralError(err)
				}
				if !confirmed {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cli.uninstall.cancelled"))
					return nil
				}
				keepConfig = nextKeep
			}

			allowInteractiveSudo := format != outputFormatJSON && isTerminalInput(cmd.InOrStdin())
			res, err := svc.Uninstall(ctx, service.UninstallOptions{
				KeepConfig:           keepConfig,
				AllowInteractiveSudo: allowInteractiveSudo,
			})
			if err != nil {
				return wrapGeneralError(err)
			}
			return writeUninstallResult(cmd.OutOrStdout(), format, res)
		},
	}

	cmd.Flags().BoolVar(&keepConfig, "keep-config", false, "keep configuration directory")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview uninstall actions without deleting files")
	cmd.Flags().StringVarP(&output, "output", "o", "plain", "plain|table|json")
	return cmd
}

func promptUninstall(cmd *cobra.Command, plan service.UninstallPlan, keepConfig, keepConfigExplicit bool) (confirmed bool, nextKeepConfig bool, err error) {
	out := cmd.ErrOrStderr()
	reader := bufio.NewReader(cmd.InOrStdin())

	_, _ = fmt.Fprintln(out, i18n.T("cli.uninstall.preview_title"))
	if plan.ExecutablePath != "" {
		_, _ = fmt.Fprintf(out, "  - %s: %s\n", i18n.T("settings.uninstall.label_binary"), plan.ExecutablePath)
	}
	if plan.ServiceInstalled {
		_, _ = fmt.Fprintf(out, "  - %s: %s\n", i18n.T("settings.uninstall.label_service"), plan.ServiceUnit)
	}
	if len(plan.ShellFiles) > 0 {
		_, _ = fmt.Fprintf(out, "  - %s: %s\n", i18n.T("settings.uninstall.label_shell"), strings.Join(plan.ShellFiles, ", "))
	}
	if plan.ConfigExists {
		_, _ = fmt.Fprintf(out, "  - %s: %s\n", i18n.T("settings.uninstall.label_config"), plan.ConfigDir)
	}

	_, _ = fmt.Fprint(out, i18n.T("cli.uninstall.prompt_confirm")+" ")
	line, readErr := reader.ReadString('\n')
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return false, keepConfig, readErr
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	if ans != "y" && ans != "yes" {
		return false, keepConfig, nil
	}

	nextKeepConfig = keepConfig
	if plan.ConfigExists && !keepConfigExplicit {
		_, _ = fmt.Fprint(out, i18n.Tf("cli.uninstall.prompt_keep_config", plan.ConfigDir)+" ")
		line, readErr = reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return false, keepConfig, readErr
		}
		ans = strings.ToLower(strings.TrimSpace(line))
		nextKeepConfig = ans == "" || ans == "y" || ans == "yes"
	}
	return true, nextKeepConfig, nil
}

func writeUninstallResult(out io.Writer, format outputFormat, res service.UninstallResult) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(out, res)
	case outputFormatTable:
		return writeUninstallTable(out, res)
	default:
		_, err := fmt.Fprintln(out, service.FormatUninstallSummary(res))
		return err
	}
}

func writeUninstallTable(out io.Writer, res service.UninstallResult) error {
	w := newTabWriter(out)
	if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n",
		i18n.T("cli.uninstall.col_item"),
		i18n.T("cli.uninstall.col_status"),
		i18n.T("cli.uninstall.col_detail"),
	); err != nil {
		return err
	}

	statusPlannedOr := func(active bool, activeKey, inactiveKey string) string {
		if res.DryRun && active {
			return i18n.T("cli.uninstall.status_planned")
		}
		if active {
			return i18n.T(activeKey)
		}
		return i18n.T(inactiveKey)
	}

	if res.CoreStopped {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n",
			i18n.T("core.group.running"),
			statusPlannedOr(true, "cli.uninstall.status_stopped", "cli.uninstall.status_skipped"),
			"",
		)
	}
	if res.ServiceRemoved || res.ServiceUnit != "" {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n",
			i18n.T("settings.uninstall.label_service"),
			statusPlannedOr(res.ServiceRemoved, "cli.uninstall.status_removed", "cli.uninstall.status_skipped"),
			res.ServiceUnit,
		)
	}
	if res.NetworkRecovered {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n",
			i18n.T("settings.uninstall.label_network"),
			statusPlannedOr(true, "cli.uninstall.status_recovered", "cli.uninstall.status_skipped"),
			"",
		)
	}
	if len(res.ShellFilesCleaned) > 0 || res.ProxyScriptRemoved {
		detail := strings.Join(res.ShellFilesCleaned, ", ")
		if detail == "" && res.ProxyScriptRemoved {
			detail = "proxy.sh"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n",
			i18n.T("settings.uninstall.label_shell"),
			statusPlannedOr(true, "cli.uninstall.status_removed", "cli.uninstall.status_skipped"),
			detail,
		)
	}

	cfgStatus := i18n.T("cli.uninstall.status_skipped")
	if res.ConfigRemoved {
		cfgStatus = statusPlannedOr(true, "cli.uninstall.status_removed", "cli.uninstall.status_skipped")
	} else if res.ConfigPreserved {
		cfgStatus = i18n.T("cli.uninstall.status_preserved")
	}
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", i18n.T("settings.uninstall.label_config"), cfgStatus, res.ConfigDir)

	binStatus := i18n.T("cli.uninstall.status_skipped")
	binDetail := res.ExecutablePath
	switch {
	case res.BinaryRemoved:
		binStatus = statusPlannedOr(true, "cli.uninstall.status_removed", "cli.uninstall.status_skipped")
	case res.BinaryDeferred:
		binStatus = i18n.T("cli.uninstall.status_deferred")
	case res.BinaryHomebrew || res.BinaryNeedsManualRemove:
		binStatus = i18n.T("cli.uninstall.status_manual")
		if res.ManualCommand != "" {
			binDetail = res.ManualCommand
		}
	}
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", i18n.T("settings.uninstall.label_binary"), binStatus, binDetail)

	for _, warning := range res.Warnings {
		_, _ = fmt.Fprintf(w, "warning\t!\t%s\n", warning)
	}
	return w.Flush()
}

func isNonInteractiveFile(r io.Reader) bool {
	if _, ok := r.(*os.File); !ok {
		return false
	}
	return !isTerminalInput(r)
}

func isTerminalInput(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if nullInfo, err := os.Stat(os.DevNull); err == nil && os.SameFile(info, nullInfo) {
		return false
	}
	return true
}
