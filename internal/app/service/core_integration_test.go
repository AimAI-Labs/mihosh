package service

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

// Opt-in integration test: all configuration, ports and process ownership are isolated.
// The default suite never requires a downloaded binary or an external service.
func TestCoreRealBinary(t *testing.T) {
	source := os.Getenv("MIHOSH_INTEGRATION_BINARY")
	if source == "" {
		t.Skip("set MIHOSH_INTEGRATION_BINARY to a verified Mihomo executable")
	}
	newIsolatedService(t)
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	tag := coreinfra.PinnedRelease().Tag
	binary, err := coreinfra.BinaryPath(root, tag, runtime.GOOS)
	require.NoError(t, err)
	data, err := os.ReadFile(source)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(binary), 0700))
	require.NoError(t, os.WriteFile(binary, data, 0700))
	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.Core.Version = tag
	require.NoError(t, config.Save(cfg))
	svc := NewCoreService()
	defer svc.Close()
	require.NoError(t, svc.Configure())
	freePort := func() int {
		listener, e := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, e)
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		return port
	}
	controller := net.JoinHostPort("127.0.0.1", strconv.Itoa(freePort()))
	proxyPort := freePort()
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	require.NoError(t, config.WriteMihomoField(path, "external-controller", controller))
	require.NoError(t, config.WriteMihomoField(path, "mixed-port", proxyPort))
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err = svc.Start(ctx); err != nil {
		t.Fatalf("start: %v\n%s", err, svc.logs.String())
	}
	client := api.NewClient(config.ResolveMihomoEndpoint(), 1000)
	version, err := client.GetVersion()
	require.NoError(t, err)
	require.Equal(t, tag, version.Version)
	proxy, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(proxyPort)), time.Second)
	require.NoError(t, err)
	proxy.Close()
	other := NewCoreService()
	defer other.Close()
	require.NoError(t, other.Start(ctx), "another client should reuse the live owner")
	require.Nil(t, other.process, "another client must not own a second core")
	observed, e := other.Status()
	require.NoError(t, e)
	require.True(t, observed.Running)
	require.NoError(t, other.Control(ctx, "restart", ""))
	ps := NewProfileService(client, svc)
	p, err := ps.AddProfile("integration", profile.SubSource{Kind: profile.SourceLocal, Path: "unused"})
	require.NoError(t, err)
	require.NoError(t, profile.WriteRaw(p.UID, []byte("proxies: []\nrules:\n  - MATCH,DIRECT\n")))
	if _, err = ps.Activate(p.UID); err != nil {
		t.Fatalf("activate: %v\n%s", err, svc.logs.String())
	}
	cfg, err = config.Load()
	require.NoError(t, err)
	require.Equal(t, p.UID, cfg.ActiveSub)
	waited := make(chan error, 1)
	go func() { waited <- svc.Wait(ctx) }()
	require.NoError(t, other.Control(ctx, "stop", ""))
	select {
	case e := <-waited:
		require.NoError(t, e)
	case <-time.After(5 * time.Second):
		t.Fatal("foreground owner did not observe remote stop")
	}
	status, err := svc.Status()
	require.NoError(t, err)
	require.False(t, status.Running)
	cfg, err = config.Load()
	require.NoError(t, err)
	cfg.Core.Version = "v9.9.9"
	cfg.Core.PendingVersion = tag
	require.NoError(t, config.Save(cfg))
	require.NoError(t, svc.Start(ctx))
	cfg, err = config.Load()
	require.NoError(t, err)
	require.Equal(t, tag, cfg.Core.Version)
	require.Empty(t, cfg.Core.PendingVersion)
	require.NoError(t, svc.Stop())
	if app := os.Getenv("MIHOSH_INTEGRATION_APP"); app != "" {
		run := func(args ...string) []byte {
			command := exec.CommandContext(ctx, app, args...)
			out, e := command.CombinedOutput()
			require.NoError(t, e, string(out))
			return out
		}
		var state CoreStatus
		require.NoError(t, json.Unmarshal(run("core", "status", "--output", "json"), &state))
		require.False(t, state.Running)
		foreground := exec.CommandContext(ctx, app, "core", "start", "--output", "json")
		var output coreinfra.LogBuffer
		foreground.Stdout = &output
		foreground.Stderr = &output
		require.NoError(t, foreground.Start())
		finished := make(chan error, 1)
		go func() { finished <- foreground.Wait() }()
		defer func() { _ = foreground.Process.Kill() }()
		require.Eventually(t, func() bool { _, e := client.GetVersion(); return e == nil }, 10*time.Second, 50*time.Millisecond)
		require.NoError(t, json.Unmarshal(run("core", "status", "--output", "json"), &state))
		require.True(t, state.Running)
		var result map[string]string
		require.NoError(t, other.Control(ctx, "status", ""))
		require.NoError(t, json.Unmarshal(run("core", "restart", "--output", "json"), &result))
		require.Equal(t, "ok", result["result"])
		require.NoError(t, json.Unmarshal(run("core", "stop", "--output", "json"), &result))
		require.Equal(t, "ok", result["result"])
		select {
		case e := <-finished:
			require.NoError(t, e, output.String())
		case <-time.After(10 * time.Second):
			t.Fatal("foreground CLI did not exit after remote stop")
		}
		require.NoError(t, json.Unmarshal([]byte(output.String()), &result))
		require.Equal(t, "ok", result["result"])
		// A still-open TUI owner must republish its control entry after another
		// foreground session has temporarily owned and then stopped the core.
		require.NoError(t, svc.Start(ctx))
		require.NoError(t, other.Control(ctx, "stop", ""))
	}
}
