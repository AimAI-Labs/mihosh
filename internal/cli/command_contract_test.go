package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestCLIOutputAndArgumentContracts(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	originalRun, originalSystemd := runSystemCommandFn, isLinuxSystemdFn
	t.Cleanup(func() { runSystemCommandFn, isLinuxSystemdFn = originalRun, originalSystemd })
	isLinuxSystemdFn = func() bool { return true }
	var systemCalls int
	runSystemCommandFn = func(string, ...string) error {
		systemCalls++
		return nil
	}
	for _, args := range [][]string{
		{"status", "--unit", "fixture", "--output", "json"},
		{"logs", "--output", "nonsense"},
		{"logs", "--output", "json"},
	} {
		cmd := newServiceCommand()
		cmd.SilenceErrors, cmd.SilenceUsage = true, true
		cmd.SetArgs(args)
		err := cmd.Execute()
		if err == nil || exitCodeForError(err) != exitCodeParameter {
			t.Errorf("service %v should return a parameter error, got %v", args, err)
		}
	}
	if systemCalls != 0 {
		t.Errorf("invalid service invocations ran %d system commands", systemCalls)
	}

	cmd := newCoreCommand()
	cmd.SetArgs([]string{"activate"})
	if err := cmd.Execute(); err == nil || exitCodeForError(err) != exitCodeParameter {
		t.Errorf("core activate without --version should return a parameter error, got %v", err)
	}
	for _, action := range []string{"status", "configure", "start", "restart", "stop", "rollback"} {
		child, _, err := cmd.Find([]string{action})
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range []string{"version", "file", "sha256"} {
			if child.Flags().Lookup(flag) != nil {
				t.Errorf("core %s has unused --%s", action, flag)
			}
		}
	}
	for _, args := range [][]string{
		{"version"}, {"status"}, {"list"}, {"connections"}, {"doctor"},
		{"config", "show"}, {"config", "edit"}, {"on"}, {"off"},
	} {
		child, _, err := rootCmd.Find(args)
		if err != nil {
			t.Fatal(err)
		}
		if child.Args == nil || child.Args(child, []string{"unexpected"}) == nil {
			t.Errorf("%v accepted an unexpected positional argument", args)
		}
	}
	if modeCmd.Args == nil || modeCmd.Args(modeCmd, []string{"rule", "unexpected"}) == nil {
		t.Error("mode accepted more than one argument")
	}
	for _, cmd := range []*cobra.Command{modeCmd, selectCmd, configSetCmd} {
		if flag := cmd.Flags().Lookup("output"); flag == nil || flag.Shorthand != "o" {
			t.Errorf("%s is missing -o/--output", cmd.Name())
		}
	}
}

func TestModeSelectAndConfigSetWriteSupportedFormats(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("auto") })
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	var apiCalls atomic.Int32
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls.Add(1)
		switch r.Method + " " + r.URL.Path {
		case "GET /configs":
			_, _ = w.Write([]byte(`{"mode":"rule"}`))
		case "PATCH /configs", "PUT /proxies/Group":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer controller.Close()
	cfg := config.DefaultConfig
	cfg.Core.ExternalController = controller.URL
	require.NoError(t, config.Save(&cfg))

	for _, tc := range []struct {
		makeCmd   func() *cobra.Command
		args      []string
		want      map[string]string
		tableName string
	}{
		{newModeCommand, nil, map[string]string{"mode": "rule"}, "MODE"},
		{newModeCommand, []string{"DIRECT"}, map[string]string{"mode": "direct"}, "MODE"},
		{newSelectCommand, []string{"Group", "Node"}, map[string]string{"group": "Group", "proxy": "Node"}, "GROUP"},
		{newConfigSetCommand, []string{"timeout", "4200"}, map[string]string{"key": "timeout", "value": "4200"}, "KEY"},
	} {
		for _, format := range []string{"plain", "table", "json"} {
			cmd := tc.makeCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs(append([]string{"-o", format}, tc.args...))
			require.NoError(t, cmd.Execute())
			switch format {
			case "json":
				got := map[string]string{}
				require.NoError(t, json.Unmarshal(out.Bytes(), &got))
				require.Equal(t, tc.want, got)
			case "table":
				require.Contains(t, out.String(), tc.tableName)
			}
			for _, value := range tc.want {
				require.Contains(t, out.String(), value)
			}
		}
	}
	before := apiCalls.Load()
	for _, tc := range []struct {
		makeCmd func() *cobra.Command
		args    []string
	}{
		{newModeCommand, []string{"rule"}},
		{newSelectCommand, []string{"Group", "Node"}},
		{newConfigSetCommand, []string{"timeout", "1234"}},
	} {
		cmd := tc.makeCmd()
		cmd.SilenceErrors, cmd.SilenceUsage = true, true
		cmd.SetArgs(append([]string{"--output", "unsupported"}, tc.args...))
		require.Equal(t, exitCodeParameter, exitCodeForError(cmd.Execute()))
	}
	require.Equal(t, before, apiCalls.Load())
	latest, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, 4200, latest.Timeout)
}
