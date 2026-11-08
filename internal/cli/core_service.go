package cli

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/host"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/cobra"
)

func coreActionResult(cmd *cobra.Command, output, action string, run func(context.Context) (any, error)) error {
	format, err := parseOutputFormat(output)
	if err != nil {
		return wrapParameterError(err)
	}
	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	result, err := run(ctx)
	if err != nil {
		if format == outputFormatJSON {
			_ = writeJSON(cmd.OutOrStdout(), map[string]string{"action": action, "stage": "failed", "result": "error", "error": err.Error()})
		}
		return wrapGeneralError(err)
	}
	if result == nil {
		result = map[string]string{"action": action, "stage": "complete", "result": "ok"}
	}
	return writeCoreResult(cmd.OutOrStdout(), format, result)
}
func newCoreServiceCommand(output *string) *cobra.Command {
	parent := &cobra.Command{Use: "service", Short: i18n.T("core.action.service")}
	for _, action := range []string{"install", "uninstall", "start", "stop", "restart", "enable", "disable", "boot", "status"} {
		child := &cobra.Command{Use: action, Short: i18n.T("core.service." + action), Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			return coreActionResult(cmd, *output, "service-"+action, func(ctx context.Context) (any, error) {
				svc := service.NewCoreService()
				defer svc.Close()
				if action == "status" {
					return svc.Status()
				}
				return nil, svc.ServiceAction(ctx, action)
			})
		}}

		parent.AddCommand(child)
	}
	var root string
	run := &cobra.Command{Use: "run", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cleanup, err := setCoreHome(root)
		if err != nil {
			return err
		}
		defer cleanup()
		home, err := config.GetConfigDir()
		if err != nil {
			return err
		}
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		svc := service.NewCoreService()
		defer svc.Close()
		return host.RunService(ctx, host.ServiceName(home), svc.Serve)
	}}
	run.Flags().StringVar(&root, "home", "", "")
	parent.AddCommand(run)
	return parent
}
func setCoreHome(root string) (func(), error) {
	if root == "" {
		return func() {}, nil
	}
	if !filepath.IsAbs(root) {
		return nil, errors.New("service home must be an absolute path")
	}
	old, exists := os.LookupEnv("MIHOSH_CONFIG_DIR")
	if err := os.Setenv("MIHOSH_CONFIG_DIR", filepath.Clean(root)); err != nil {
		return nil, err
	}
	return func() {
		if exists {
			_ = os.Setenv("MIHOSH_CONFIG_DIR", old)
		} else {
			_ = os.Unsetenv("MIHOSH_CONFIG_DIR")
		}
	}, nil
}
func newCoreNetworkCommand(kind string, output *string) *cobra.Command {
	parent := &cobra.Command{Use: kind, Short: i18n.T("core.action." + kind)}
	for _, action := range []string{"on", "off", "status"} {
		parent.AddCommand(&cobra.Command{Use: action, Args: cobra.NoArgs, Short: i18n.T("core.network." + action), RunE: func(cmd *cobra.Command, _ []string) error {
			return coreActionResult(cmd, *output, kind+"-"+action, func(ctx context.Context) (any, error) {
				svc := service.NewCoreService()
				defer svc.Close()
				if action == "status" {
					return svc.Status()
				}
				if kind == "tun" {
					return nil, svc.SetTUN(ctx, action == "on")
				}
				return nil, svc.SetSystemProxy(ctx, action == "on")
			})
		}})
	}
	return parent
}
func newCoreRecoveryCommand(output *string) *cobra.Command {
	return &cobra.Command{Use: "network-recover", Short: i18n.T("core.action.recover"), Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return coreActionResult(cmd, *output, "network-recover", func(ctx context.Context) (any, error) {
			svc := service.NewCoreService()
			defer svc.Close()
			return nil, svc.RecoverNetwork(ctx)
		})
	}}
}
func newCoreGuardCommand() *cobra.Command {
	var root string
	cmd := &cobra.Command{Use: "network-guard", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cleanup, err := setCoreHome(root)
		if err != nil {
			return err
		}
		defer cleanup()
		svc := service.NewCoreService()
		defer svc.Close()
		return svc.GuardNetwork(cmd.Context())
	}}
	cmd.Flags().StringVar(&root, "home", "", "")
	return cmd
}
