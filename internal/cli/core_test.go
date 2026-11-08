package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestCoreStatusOutputFormatsWithoutCoreOrSubscription(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	viper.Reset()
	defer viper.Reset()
	for _, format := range []string{"plain", "table", "json"} {
		t.Run(format, func(t *testing.T) {
			cmd := newCoreCommand()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs([]string{"status", "--output", format})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			output := buf.String()
			if format == "json" {
				var value map[string]any
				if err := json.Unmarshal(buf.Bytes(), &value); err != nil {
					t.Fatal(err)
				}
				if value["mode"] != "external" {
					t.Fatal(value)
				}
			} else if !strings.Contains(output, "external") {
				t.Fatal(output)
			}
			if format == "plain" && strings.Contains(output, "MODE") {
				t.Fatal("table header in plain output")
			}
		})
	}
}
func TestCoreRejectsInvalidFormatBeforeInstall(t *testing.T) {
	cmd := newCoreCommand()
	cmd.SetArgs([]string{"install", "--output", "invalid"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("invalid format accepted")
	}
}

func TestCoreJSONFailureHasMachineReadableResult(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	viper.Reset()
	defer viper.Reset()
	cmd := newCoreCommand()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"import", "--version", "v1.2.3", "--file", "missing", "--output", "json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("invalid import succeeded")
	}
	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("%v output=%q", err, out.String())
	}
	if result["action"] != "import" || result["result"] != "error" || result["error"] == "" {
		t.Fatal(result)
	}
}
