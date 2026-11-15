package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func runEnvForTest(args ...string) (string, error) {
	cmd := newEnvCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestEnvCommandPersistsSettingsAndSupportsAllFormats(t *testing.T) {
	i18n.Init()
	for _, format := range []string{"plain", "table", "json"} {
		t.Run(format, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("MIHOSH_CONFIG_DIR", root)
			out, err := runEnvForTest("on", "--output", format)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load()
			if err != nil || !cfg.ShellProxy {
				t.Fatal("CLI did not persist the enabled preference", err)
			}
			if _, err := os.Stat(filepath.Join(root, "proxy.sh")); err != nil {
				t.Fatal("CLI did not create a source script", err)
			}
			switch format {
			case "json":
				var result struct {
					Enabled    bool   `json:"enabled"`
					ScriptPath string `json:"script_path"`
					Script     string `json:"script"`
				}
				if err := json.Unmarshal([]byte(out), &result); err != nil || !result.Enabled || result.ScriptPath != filepath.Join(root, "proxy.sh") || !strings.HasSuffix(result.Script, "mihosh_proxy_on\n") {
					t.Fatalf("invalid JSON output: %q, %v", out, err)
				}
			case "plain":
				if !strings.HasPrefix(out, "mihosh_proxy_on()") || !strings.HasSuffix(out, "mihosh_proxy_on\n") {
					t.Fatal("plain output is not a clean shell script")
				}
			case "table":
				if !strings.Contains(out, "true") || !strings.Contains(out, filepath.Join(root, "proxy.sh")) {
					t.Fatal("table output does not show the saved preference and script")
				}
			}
			out, err = runEnvForTest("off")
			if err != nil || !strings.HasSuffix(out, "mihosh_proxy_off\n") {
				t.Fatal("off output does not unset the environment", err)
			}
			cfg, err = config.Load()
			if err != nil || cfg.ShellProxy {
				t.Fatal("off preference was not saved", err)
			}
		})
	}
}

func TestEnvCommandDefaultAndInvalidInputHaveNoSideEffects(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	if out, err := runEnvForTest(); err != nil || !strings.HasSuffix(out, "mihosh_proxy_off\n") {
		t.Fatal("first-use env did not render the disabled preference", err)
	}
	for _, args := range [][]string{{"invalid"}, {"on", "off"}, {"on", "--output", "invalid"}} {
		if _, err := runEnvForTest(args...); err == nil {
			t.Fatalf("invalid arguments accepted: %v", args)
		}
	}
	if _, err := config.Load(); !errors.Is(err, config.ErrConfigNotFound) {
		t.Fatal("read or rejected command changed the configuration", err)
	}
	if _, err := os.Stat(filepath.Join(root, "proxy.sh")); !os.IsNotExist(err) {
		t.Fatal("read or rejected command created a script", err)
	}
}
