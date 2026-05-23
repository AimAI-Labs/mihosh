package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

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
		Short: "管理 mihomo systemd 服务",
		Long:  "管理 mihomo systemd 服务，并通过 journalctl 查看日志。仅支持 Linux systemd 系统。",
	}
	cmd.PersistentFlags().StringVar(&opts.unit, "unit", defaultSystemdUnit, "systemd unit 名称")

	cmd.AddCommand(newServiceActionCommand("status", "查看服务状态", false, &opts))
	for _, action := range []string{"start", "stop", "restart", "enable", "disable"} {
		cmd.AddCommand(newServiceActionCommand(action, serviceActionShort(action), true, &opts))
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
					return wrapGeneralError(fmt.Errorf("执行 sudo systemctl %s 失败: %w", action, err))
				}
				return nil
			}

			if err := runSystemCommandFn("systemctl", action, unit, "--no-pager"); err != nil {
				return wrapGeneralError(fmt.Errorf("执行 systemctl 失败: %w", err))
			}
			return nil
		},
	}
}

func newServiceLogsCommand(opts *serviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "查看服务日志",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureLinuxSystemd(); err != nil {
				return err
			}
			if opts.lines <= 0 {
				return wrapParameterError(fmt.Errorf("--lines 必须大于 0"))
			}

			unit := normalizeSystemdUnit(opts.unit)
			journalArgs := []string{"-u", unit, "-n", strconv.Itoa(opts.lines)}
			if opts.follow {
				journalArgs = append(journalArgs, "-f")
			} else {
				journalArgs = append(journalArgs, "--no-pager")
			}

			if err := runSystemCommandFn("journalctl", journalArgs...); err != nil {
				return wrapGeneralError(fmt.Errorf("执行 journalctl 失败: %w", err))
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&opts.lines, "lines", "n", 200, "显示最近 N 行日志")
	cmd.Flags().BoolVarP(&opts.follow, "follow", "f", false, "持续跟随日志输出")
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
		return wrapGeneralError(fmt.Errorf("service 命令仅支持 Linux systemd 系统"))
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

func serviceActionShort(action string) string {
	switch action {
	case "start":
		return "启动服务"
	case "stop":
		return "停止服务"
	case "restart":
		return "重启服务"
	case "enable":
		return "设置服务开机自启"
	case "disable":
		return "取消服务开机自启"
	default:
		return action
	}
}
