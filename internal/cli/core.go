package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/cobra"
)

func newCoreCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "core", SilenceUsage: true, SilenceErrors: true, Short: i18n.T("cli.core.short")}
	var output string
	cmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "plain|table|json")
	cmd.AddCommand(newCoreServiceCommand(&output), newCoreNetworkCommand("system-proxy", &output), newCoreNetworkCommand("tun", &output), newCoreRecoveryCommand(&output), newCoreGuardCommand())
	for _, action := range []string{"status", "releases", "install", "import", "configure", "start", "restart", "stop", "upgrade", "rollback", "activate"} {
		var version, source, digest string
		child := &cobra.Command{Use: action, Short: i18n.T("core.action." + action), Args: cobra.NoArgs}
		child.Flags().StringVar(&version, "version", "", i18n.T("cli.core.version"))
		child.Flags().StringVar(&source, "file", "", i18n.T("cli.core.file"))
		child.Flags().StringVar(&digest, "sha256", "", i18n.T("cli.core.sha256"))
		child.RunE = func(cmd *cobra.Command, args []string) (commandErr error) {
			format, err := parseOutputFormat(output)
			if err != nil {
				return wrapParameterError(err)
			}
			defer func() {
				if commandErr != nil && format == outputFormatJSON {
					_ = writeJSON(cmd.OutOrStdout(), map[string]string{"action": action, "stage": "failed", "result": "error", "error": commandErr.Error()})
				}
			}()
			svc := service.NewCoreService()
			defer svc.Close()
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			var result any
			var progress coreinfra.Progress
			notify := func(p coreinfra.Progress) { progress = p }
			selectedVersion := version
			switch action {
			case "status":
				result, err = svc.Status()
			case "releases":
				result, err = svc.Release(ctx, version, notify)
			case "configure":
				err = svc.Configure()
			case "install", "upgrade":
				release := coreinfra.PinnedRelease()
				if version != "" || action == "upgrade" {
					release, err = svc.Release(ctx, version, notify)
				}
				if err == nil {
					err = svc.Install(ctx, release, notify)
				}
				selectedVersion = release.Version()
				if err == nil && action == "upgrade" {
					err = svc.Control(ctx, "upgrade", release.Version())
					if errors.Is(err, service.ErrNoCoreOwner) {
						err = svc.UpgradeVersion(ctx, release.Version())
					}
				}
			case "import":
				err = svc.Import(ctx, source, version, digest)
			case "activate":
				err = svc.Control(ctx, "activate", version)
				if errors.Is(err, service.ErrNoCoreOwner) {
					err = svc.ActivateVersion(ctx, version)
				}
			case "rollback":
				err = svc.Control(ctx, "rollback", "")
				if errors.Is(err, service.ErrNoCoreOwner) {
					err = svc.ActivateVersion(ctx, "")
				}
			case "stop":
				err = svc.StopContext(ctx)
			case "start", "restart":
				// First-stage runtime is explicitly foreground: never leave an unowned child.
				if action == "start" {
					err = svc.Start(ctx)
				} else {
					err = svc.Control(ctx, "restart", "")
					if !errors.Is(err, service.ErrNoCoreOwner) {
						break
					}
					err = svc.Start(ctx)
				}
				if err == nil {
					err = svc.Wait(ctx)
				}
			}
			if err != nil {
				return wrapGeneralError(err)
			}
			if result == nil {
				result = map[string]string{"action": action, "stage": "complete", "version": selectedVersion, "route": progress.Route, "result": "ok"}
			}
			return writeCoreResult(cmd.OutOrStdout(), format, result)
		}
		cmd.AddCommand(child)
	}
	return cmd
}

func newSetupCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{Use: "setup", SilenceUsage: true, SilenceErrors: true, Short: i18n.T("cli.setup.short"), Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) (commandErr error) {
		format, err := parseOutputFormat(output)
		if err != nil {
			return wrapParameterError(err)
		}
		defer func() {
			if commandErr != nil && format == outputFormatJSON {
				_ = writeJSON(cmd.OutOrStdout(), map[string]string{"action": "setup", "stage": "failed", "result": "error", "error": commandErr.Error()})
			}
		}()
		svc := service.NewCoreService()
		defer svc.Close()
		status, err := svc.Status()
		if err != nil {
			return wrapGeneralError(err)
		}
		if status.Version == "" {
			if err = svc.Install(cmd.Context(), coreinfra.PinnedRelease(), nil); err != nil {
				return wrapGeneralError(err)
			}
		}
		if err = svc.Configure(); err != nil {
			return wrapGeneralError(err)
		}
		configured, err := svc.Status()
		if err != nil {
			return wrapGeneralError(err)
		}
		result := map[string]string{"action": "setup", "stage": "complete", "version": configured.Version, "result": "configured", "next": i18n.T("cli.setup.next")}
		return writeCoreResult(cmd.OutOrStdout(), format, result)
	}}
	cmd.Flags().StringVarP(&output, "output", "o", "table", "plain|table|json")
	return cmd
}

func writeCoreResult(out io.Writer, format outputFormat, result any) error {
	if format == outputFormatJSON {
		return writeJSON(out, result)
	}
	switch value := result.(type) {
	case service.CoreStatus:
		if format == outputFormatTable {
			w := newTabWriter(out)
			if _, err := fmt.Fprintln(w, "MODE\tVERSION\tRUNNING\tINSTALLED\tRUNTIME\tSERVICE\tAUTOSTART\tSYSTEM_PROXY\tTUN\tRECOVERY"); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%t\t%s\t%s\t%t\t%t\t%t\t%t\t%t\n", value.Mode, value.Version, value.Running, strings.Join(value.Installed, ","), value.Runtime, value.ServiceInstalled, value.AutoStart, value.SystemProxy, value.TUN, value.RecoveryPending); err != nil {
				return err
			}
			return w.Flush()
		}
		_, err := fmt.Fprintf(out, "%s %s %t %s %s %t %t %t %t %t\n", value.Mode, value.Version, value.Running, strings.Join(value.Installed, ","), value.Runtime, value.ServiceInstalled, value.AutoStart, value.SystemProxy, value.TUN, value.RecoveryPending)
		return err
	case coreinfra.Release:
		if format == outputFormatTable {
			w := newTabWriter(out)
			if _, err := fmt.Fprintln(w, "VERSION\tASSET\tSIZE\tSHA256\tROUTE"); err != nil {
				return err
			}
			for _, asset := range value.Assets {
				if _, err := fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", value.Tag, asset.Name, asset.Size, asset.Digest, value.Route); err != nil {
					return err
				}
			}
			return w.Flush()
		}
		for _, asset := range value.Assets {
			if _, err := fmt.Fprintf(out, "%s %s %d %s %s\n", value.Tag, asset.Name, asset.Size, asset.Digest, value.Route); err != nil {
				return err
			}
		}
		return nil
	case map[string]string:
		if format == outputFormatTable {
			w := newTabWriter(out)
			if _, err := fmt.Fprintln(w, "ACTION\tSTAGE\tVERSION\tROUTE\tRESULT"); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", value["action"], value["stage"], value["version"], value["route"], value["result"]); err != nil {
				return err
			}
			return w.Flush()
		}
		_, err := fmt.Fprintf(out, "%s %s %s %s %s\n", value["action"], value["stage"], value["version"], value["route"], value["result"])
		return err
	}
	return fmt.Errorf("unsupported core output")
}
