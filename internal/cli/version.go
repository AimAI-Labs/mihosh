package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/spf13/cobra"
)

var versionOutput string

var versionCmd = &cobra.Command{
	Use:  "version",
	Args: cobra.NoArgs,
	Example: `  mihosh version
  mihosh version --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := parseOutputFormat(versionOutput)
		if err != nil {
			return wrapParameterError(err)
		}
		return renderVersion(os.Stdout, format)
	},
}

func init() {
	versionCmd.Flags().StringVarP(&versionOutput, "output", "o", string(outputFormatPlain), "")
}

func renderVersion(w io.Writer, format outputFormat) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(w, map[string]string{
			"version": model.Version,
			"commit":  model.Commit,
			"date":    model.Date,
		})
	case outputFormatTable:
		tw := newTabWriter(w)
		if _, err := fmt.Fprintln(tw, "KEY\tVALUE"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "VERSION\t%s\n", model.Version); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "COMMIT\t%s\n", model.Commit); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "DATE\t%s\n", model.Date); err != nil {
			return err
		}
		return tw.Flush()
	default:
		if _, err := fmt.Fprintf(w, "mihosh %s (commit: %s, built: %s)\n", model.Version, model.Commit, model.Date); err != nil {
			return err
		}
		return nil
	}
}
