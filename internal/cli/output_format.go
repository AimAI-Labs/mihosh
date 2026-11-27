package cli

import (
	"encoding/json"
	"errors"
	"fmt"
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

type outputField struct {
	name, value string
}

func writeFieldResult(w io.Writer, format outputFormat, plain string, fields ...outputField) error {
	switch format {
	case outputFormatJSON:
		record := make(map[string]string, len(fields))
		for _, field := range fields {
			record[field.name] = field.value
		}
		return writeJSON(w, record)
	case outputFormatTable:
		var columns, values []string
		for _, field := range fields {
			columns = append(columns, strings.ToUpper(field.name))
			values = append(values, field.value)
		}
		table := newTabWriter(w)
		if _, err := fmt.Fprintln(table, strings.Join(columns, "\t")); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(table, strings.Join(values, "\t")); err != nil {
			return err
		}
		return table.Flush()
	default:
		_, err := fmt.Fprintln(w, plain)
		return err
	}
}
