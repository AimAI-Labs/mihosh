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
	Use:   "version",
	Short: "显示版本信息",
	Long:  `显示 mihosh 的版本号、提交哈希和构建日期。`,
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
	versionCmd.Flags().StringVar(&versionOutput, "output", string(outputFormatPlain), "输出格式: json|plain")
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
		fmt.Fprintln(tw, "KEY\tVALUE")
		fmt.Fprintf(tw, "VERSION\t%s\n", model.Version)
		fmt.Fprintf(tw, "COMMIT\t%s\n", model.Commit)
		fmt.Fprintf(tw, "DATE\t%s\n", model.Date)
		return tw.Flush()
	default:
		fmt.Fprintf(w, "mihosh %s (commit: %s, built: %s)\n", model.Version, model.Commit, model.Date)
		return nil
	}
}
