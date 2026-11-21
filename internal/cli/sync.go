package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/cobra"
)

func newSyncCommand() *cobra.Command {
	return newSyncCommandWithFactory(service.NewSyncService)
}

func newSyncCommandWithFactory(factory func() *service.SyncService) *cobra.Command {
	syncCmd := &cobra.Command{
		Use: "sync",
	}

	// 1. mihosh sync export [file]
	var exportNoRedact bool
	var exportOutput string
	exportCmd := &cobra.Command{
		Use:     "export [file]",
		Args:    cobra.MaximumNArgs(1),
		Example: "  mihosh sync export ./mihosh-backup.tar.gz\n  mihosh sync export ./backup.zip --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(exportOutput)
			if err != nil {
				return wrapParameterError(err)
			}
			target := ""
			if len(args) > 0 {
				target = strings.TrimSpace(args[0])
			}
			res, err := factory().ExportArchive(target, !exportNoRedact)
			if err != nil {
				return wrapConfigError(fmt.Errorf("%s: %w", i18n.T("cli.sync.export.err"), err))
			}
			return renderSyncExportResult(cmdOut(cmd), res, format)
		},
	}
	exportCmd.Flags().BoolVar(&exportNoRedact, "no-redact", false, "")
	exportCmd.Flags().StringVar(&exportOutput, "output", string(outputFormatPlain), "")

	// 2. mihosh sync import <file>
	var importOutput string
	importCmd := &cobra.Command{
		Use:     "import <file>",
		Args:    cobra.ExactArgs(1),
		Example: "  mihosh sync import ./mihosh-backup.tar.gz\n  mihosh sync import ./backup.zip --output table",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(importOutput)
			if err != nil {
				return wrapParameterError(err)
			}
			res, err := factory().ImportArchive(strings.TrimSpace(args[0]))
			if err != nil {
				return wrapConfigError(fmt.Errorf("%s: %w", i18n.T("cli.sync.import.err"), err))
			}
			return renderSyncRestoreResult(cmdOut(cmd), res, format)
		},
	}
	importCmd.Flags().StringVar(&importOutput, "output", string(outputFormatPlain), "")

	// 3. mihosh sync push
	var pushDSN, pushKey, pushOutput string
	var pushNoRedact bool
	pushCmd := &cobra.Command{
		Use:     "push",
		Args:    cobra.NoArgs,
		Example: "  mihosh sync push --dsn postgres://user:pass@host/db --key laptop\n  mihosh sync push --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(pushOutput)
			if err != nil {
				return wrapParameterError(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			res, err := factory().Push(ctx, pushDSN, pushKey, !pushNoRedact)
			if err != nil {
				return wrapNetworkError(fmt.Errorf("%s: %w", i18n.T("cli.sync.push.err"), err))
			}
			return renderSyncPushResult(cmdOut(cmd), res, format)
		},
	}
	pushCmd.Flags().StringVar(&pushDSN, "dsn", "", "")
	pushCmd.Flags().StringVar(&pushKey, "key", "", "")
	pushCmd.Flags().BoolVar(&pushNoRedact, "no-redact", false, "")
	pushCmd.Flags().StringVar(&pushOutput, "output", string(outputFormatPlain), "")

	// 4. mihosh sync pull
	var pullDSN, pullKey, pullOutput string
	pullCmd := &cobra.Command{
		Use:     "pull",
		Args:    cobra.NoArgs,
		Example: "  mihosh sync pull --dsn postgres://user:pass@host/db --key laptop\n  mihosh sync pull --output table",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(pullOutput)
			if err != nil {
				return wrapParameterError(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			res, err := factory().Pull(ctx, pullDSN, pullKey)
			if err != nil {
				return wrapNetworkError(fmt.Errorf("%s: %w", i18n.T("cli.sync.pull.err"), err))
			}
			return renderSyncRestoreResult(cmdOut(cmd), res, format)
		},
	}
	pullCmd.Flags().StringVar(&pullDSN, "dsn", "", "")
	pullCmd.Flags().StringVar(&pullKey, "key", "", "")
	pullCmd.Flags().StringVar(&pullOutput, "output", string(outputFormatPlain), "")

	// 5. mihosh sync status
	var statusDSN, statusKey, statusOutput string
	statusSyncCmd := &cobra.Command{
		Use:     "status",
		Args:    cobra.NoArgs,
		Example: "  mihosh sync status --key default\n  mihosh sync status --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseOutputFormat(statusOutput)
			if err != nil {
				return wrapParameterError(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			res, err := factory().Status(ctx, statusDSN, statusKey)
			if err != nil {
				return wrapNetworkError(fmt.Errorf("%s: %w", i18n.T("cli.sync.status.err"), err))
			}
			return renderSyncStatusResult(cmdOut(cmd), res, format)
		},
	}
	statusSyncCmd.Flags().StringVar(&statusDSN, "dsn", "", "")
	statusSyncCmd.Flags().StringVar(&statusKey, "key", "", "")
	statusSyncCmd.Flags().StringVar(&statusOutput, "output", string(outputFormatPlain), "")

	syncCmd.AddCommand(exportCmd, importCmd, pushCmd, pullCmd, statusSyncCmd)
	return syncCmd
}

func cmdOut(cmd *cobra.Command) io.Writer {
	if cmd != nil && cmd.OutOrStdout() != nil {
		return cmd.OutOrStdout()
	}
	return os.Stdout
}

func renderSyncExportResult(w io.Writer, res service.SyncExportResult, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(w, res)
	case outputFormatTable:
		tw := newTabWriter(w)
		_, _ = fmt.Fprintf(tw, "FIELD\tVALUE\n")
		_, _ = fmt.Fprintf(tw, "path\t%s\n", res.Path)
		_, _ = fmt.Fprintf(tw, "size_bytes\t%d\n", res.SizeBytes)
		_, _ = fmt.Fprintf(tw, "subs\t%d\n", res.SubsCount)
		_, _ = fmt.Fprintf(tw, "redacted\t%t (%d)\n", res.Redacted, res.RedactedCount)
		_, _ = fmt.Fprintf(tw, "sha256\t%s\n", res.ContentSHA256)
		return tw.Flush()
	case outputFormatPlain:
		_, _ = fmt.Fprintf(w, "%s: %s (%d bytes, subs=%d, redacted=%t/%d)\n",
			i18n.T("cli.sync.export.ok"), res.Path, res.SizeBytes, res.SubsCount, res.Redacted, res.RedactedCount)
		return nil
	default:
		return fmt.Errorf("unsupported output format: %q", format)
	}
}

func renderSyncRestoreResult(w io.Writer, res service.SyncRestoreResult, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(w, res)
	case outputFormatTable:
		tw := newTabWriter(w)
		_, _ = fmt.Fprintf(tw, "FIELD\tVALUE\n")
		_, _ = fmt.Fprintf(tw, "source\t%s\n", res.Source)
		if res.ProfileKey != "" {
			_, _ = fmt.Fprintf(tw, "profile_key\t%s\n", res.ProfileKey)
		}
		_, _ = fmt.Fprintf(tw, "subs\t%d\n", res.SubsCount)
		_, _ = fmt.Fprintf(tw, "merge\t%t\n", res.RestoredMerge)
		_, _ = fmt.Fprintf(tw, "rules\t%t\n", res.RestoredRules)
		_, _ = fmt.Fprintf(tw, "chains\t%t\n", res.RestoredChains)
		_, _ = fmt.Fprintf(tw, "preserved_secrets\t%d\n", res.PreservedSecrets)
		return tw.Flush()
	case outputFormatPlain:
		_, _ = fmt.Fprintf(w, "%s: %s (subs=%d, merge=%t, rules=%t, chains=%t, preserved=%d)\n",
			i18n.T("cli.sync.import.ok"), res.Source, res.SubsCount, res.RestoredMerge, res.RestoredRules, res.RestoredChains, res.PreservedSecrets)
		return nil
	default:
		return fmt.Errorf("unsupported output format: %q", format)
	}
}

func renderSyncPushResult(w io.Writer, res service.SyncPushResult, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(w, res)
	case outputFormatTable:
		tw := newTabWriter(w)
		_, _ = fmt.Fprintf(tw, "FIELD\tVALUE\n")
		_, _ = fmt.Fprintf(tw, "profile_key\t%s\n", res.ProfileKey)
		_, _ = fmt.Fprintf(tw, "dsn\t%s\n", res.DSN)
		_, _ = fmt.Fprintf(tw, "size_bytes\t%d\n", res.SizeBytes)
		_, _ = fmt.Fprintf(tw, "subs\t%d\n", res.SubsCount)
		_, _ = fmt.Fprintf(tw, "redacted\t%t (%d)\n", res.Redacted, res.RedactedCount)
		_, _ = fmt.Fprintf(tw, "updated_at\t%s\n", res.UpdatedAt.Format(time.RFC3339))
		return tw.Flush()
	case outputFormatPlain:
		_, _ = fmt.Fprintf(w, "%s: key=%s (%d bytes, subs=%d, redacted=%t/%d)\n",
			i18n.T("cli.sync.push.ok"), res.ProfileKey, res.SizeBytes, res.SubsCount, res.Redacted, res.RedactedCount)
		return nil
	default:
		return fmt.Errorf("unsupported output format: %q", format)
	}
}

func renderSyncStatusResult(w io.Writer, res service.SyncStatusResult, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(w, res)
	case outputFormatTable:
		tw := newTabWriter(w)
		_, _ = fmt.Fprintf(tw, "FIELD\tVALUE\n")
		_, _ = fmt.Fprintf(tw, "profile_key\t%s\n", res.ProfileKey)
		_, _ = fmt.Fprintf(tw, "state\t%s\n", res.State)
		_, _ = fmt.Fprintf(tw, "dsn\t%s\n", res.DSN)
		_, _ = fmt.Fprintf(tw, "local_sha256\t%s\n", res.LocalSHA256)
		if res.RemoteSHA256 != "" {
			_, _ = fmt.Fprintf(tw, "remote_sha256\t%s\n", res.RemoteSHA256)
		}
		if res.RemoteDevice != "" {
			_, _ = fmt.Fprintf(tw, "remote_device\t%s\n", res.RemoteDevice)
		}
		if !res.RemoteUpdated.IsZero() {
			_, _ = fmt.Fprintf(tw, "remote_updated_at\t%s\n", res.RemoteUpdated.Format(time.RFC3339))
		}
		return tw.Flush()
	case outputFormatPlain:
		if res.State == "remote_empty" {
			_, _ = fmt.Fprintf(w, "key=%s state=%s dsn=%s\n", res.ProfileKey, res.State, res.DSN)
			return nil
		}
		_, _ = fmt.Fprintf(w, "key=%s state=%s device=%s updated=%s size=%d\n",
			res.ProfileKey, res.State, res.RemoteDevice, res.RemoteUpdated.Format(time.RFC3339), res.RemoteSize)
		return nil
	default:
		return fmt.Errorf("unsupported output format: %q", format)
	}
}
