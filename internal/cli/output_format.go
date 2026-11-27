package cli

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

type outputFormat string

const (
	outputFormatJSON  outputFormat = "json"
	outputFormatTable outputFormat = "table"
	outputFormatPlain outputFormat = "plain"
)

func parseOutputFormat(raw string) (outputFormat, error) {
	switch outputFormat(strings.ToLower(strings.TrimSpace(raw))) {
	case outputFormatJSON:
		return outputFormatJSON, nil
	case outputFormatTable:
		return outputFormatTable, nil
	case outputFormatPlain:
		return outputFormatPlain, nil
	default:
		return "", errors.New(i18n.Tf("cli.output.unsupported_format", raw))
	}
}

func writeJSON(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func newTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}
