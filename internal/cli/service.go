package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/cobra"
)

const defaultSystemdUnit = "mihomo.service"

var serviceCmd = newServiceCommand()

var runSystemCommandFn = runSystemCommand
var isLinuxSystemdFn = func() bool {
	return runtime.GOOS == "linux"
}

func newServiceCommand() *cobra.Command {
	opts := serviceOptions{
		unit:  defaultSystemdUnit,
		lines: 200,
	}

	cmd := &cobra.Command{
		Use:   "service",
	}
	cmd.PersistentFlags().StringVar(&opts.unit, "unit", defaultSystemdUnit, "")

	cmd.AddCommand(newServiceActionCommand("status", "", false, &opts))
	for _, action := range []string{"start", "stop", "restart", "enable", "disable"} {
		cmd.AddCommand(newServiceActionCommand(action, "", true, &opts))
	}
	cmd.AddCommand(newServiceLogsCommand(&opts))

	return cmd
}

type serviceOptions struct {
	unit   string
	lines  int
	follow bool
}

func newServiceActionCommand(action, short string, sudo bool, opts *serviceOptions) *cobra.Command {
	return &cobra.Command{
		Use:   action,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureLinuxSystemd(); err != nil {
				return err
			}

			unit := normalizeSystemdUnit(opts.unit)
			if sudo {
				if err := runSystemCommandFn("sudo", "systemctl", action, unit); err != nil {
					return wrapGeneralError(fmt.Errorf(i18n.Tf("cli.service.err_systemctl", action)+": %w", err))
				}
				return nil
			}

			if err := runSystemCommandFn("systemctl", action, unit, "--no-pager"); err != nil {
				return wrapGeneralError(fmt.Errorf(i18n.T("cli.service.err_systemctl_status")+": %w", err))
			}
			return nil
		},
	}
}

func newServiceLogsCommand(opts *serviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureLinuxSystemd(); err != nil {
				return err
			}
			if opts.lines <= 0 {
				return wrapParameterError(fmt.Errorf("%s", i18n.T("cli.service.logs.err_lines")))
			}

			unit := normalizeSystemdUnit(opts.unit)
			journalArgs := []string{"-u", unit, "-n", strconv.Itoa(opts.lines)}
			if opts.follow {
				journalArgs = append(journalArgs, "-f")
			} else {
				journalArgs = append(journalArgs, "--no-pager")
			}

			if err := runSystemCommandFn("journalctl", journalArgs...); err != nil {
				return wrapGeneralError(fmt.Errorf(i18n.T("cli.service.err_journalctl")+": %w", err))
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&opts.lines, "lines", "n", 200, "")
	cmd.Flags().BoolVarP(&opts.follow, "follow", "f", false, "")
	return cmd
}

func normalizeSystemdUnit(unit string) string {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		unit = defaultSystemdUnit
	}
	if !strings.HasSuffix(unit, ".service") {
		unit += ".service"
	}
	return unit
}

func ensureLinuxSystemd() error {
	if !isLinuxSystemdFn() {
		return wrapGeneralError(fmt.Errorf("%s", i18n.T("cli.service.err_linux_only")))
	}
	return nil
}

func runSystemCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// serviceActionShort is no longer used since strings are localized directly in root.go
