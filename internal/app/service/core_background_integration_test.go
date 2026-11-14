//go:build linux

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/host"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

// The coordinator, CLI and recovery guard are real processes. OS desktop settings
// and the service-manager transport are file-backed fakes, never the host's D-Bus.
func TestCoreRealBackgroundAndNetworkGuard(t *testing.T) {
	source, app := os.Getenv("MIHOSH_INTEGRATION_BINARY"), os.Getenv("MIHOSH_INTEGRATION_APP")
	if source == "" || app == "" {
		t.Skip("requires verified kernel and built Mihosh")
	}
	newIsolatedService(t)
	t.Setenv("INVOCATION_ID", "")
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	version := coreinfra.PinnedRelease().Tag
	binary, err := coreinfra.BinaryPath(root, version, "linux")
	require.NoError(t, err)
	data, err := os.ReadFile(source)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(binary), 0700))
	require.NoError(t, os.WriteFile(binary, data, 0700))
	previous := os.Getenv("MIHOSH_INTEGRATION_PREVIOUS_BINARY")
	if previous != "" {
		path, e := coreinfra.BinaryPath(root, "v1.19.32", "linux")
		require.NoError(t, e)
		data, e := os.ReadFile(previous)
		require.NoError(t, e)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, data, 0700))
	}
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.Version = version; return nil }))
	frontend := NewCoreService()
	defer frontend.Close()
	require.NoError(t, frontend.Configure())
	freePort := func() int {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, e)
		port := l.Addr().(*net.TCPAddr).Port
		_ = l.Close()
		return port
	}
	require.NoError(t, frontend.SetPorts(freePort(), freePort()))
	settingsFile := filepath.Join(t.TempDir(), "settings.json")
	original := map[string]string{}
	for _, key := range []string{"http.host", "http.port", "http.use-authentication", "https.host", "https.port", "socks.host", "socks.port", "ignore-hosts", "mode"} {
		original[key] = "'original'"
	}
	original["mode"] = "'auto'"
	encoded, err := json.Marshal(original)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(settingsFile, encoded, 0600))
	t.Setenv("MIHOSH_TEST_PROXY_SETTINGS", settingsFile)
	t.Setenv("MIHOSH_TEST_ROOT", root)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/isolated/test-bus")
	python, err := exec.LookPath("python3")
	require.NoError(t, err)
	tools := t.TempDir()
	script := fmt.Sprintf(`#!%s
import json,os,sys,urllib.request
if os.path.basename(sys.argv[0])=='systemctl':
    if 'show' in sys.argv:
        print('ActiveState=active\nUnitFileState=enabled')
    elif 'stop' in sys.argv:
        with open(os.path.join(os.environ['MIHOSH_TEST_ROOT'],'runtime','owner.json')) as f: owner=json.load(f)
        request=urllib.request.Request(owner['url']+'/control',data=b'{"action":"stop"}',headers={'Authorization':'Bearer '+owner['token'],'Content-Type':'application/json'})
        with urllib.request.urlopen(request,timeout=10) as response: response.read()
    sys.exit(0)
path=os.environ['MIHOSH_TEST_PROXY_SETTINGS']
with open(path) as f: settings=json.load(f)
key=sys.argv[3]
group=sys.argv[2].removeprefix('org.gnome.system.proxy')
if group: key=group[1:]+'.'+key
if sys.argv[1]=='get': print(settings[key])
else:
    settings[key]=sys.argv[4]
    with open(path+'.tmp','w') as f: json.dump(settings,f)
    os.replace(path+'.tmp',path)
`, python)
	for _, name := range []string{"gsettings", "systemctl"} {
		require.NoError(t, os.WriteFile(filepath.Join(tools, name), []byte(script), 0700))
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	unitDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "systemd", "user")
	require.NoError(t, os.MkdirAll(unitDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(unitDir, host.ServiceName(root)+".service"), []byte("# Managed by Mihosh\n"), 0600))
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.Runtime = "service"; cfg.Core.SystemProxy = true; return nil }))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := api.NewClient(config.ResolveMihomoEndpoint(), 500)
	readSettings := func() map[string]string {
		data, _ := os.ReadFile(settingsFile)
		var values map[string]string
		_ = json.Unmarshal(data, &values)
		return values
	}
	start := func() (*exec.Cmd, <-chan error) {
		command := exec.CommandContext(ctx, app, "core", "service", "run", "--home", root)
		var logs coreinfra.LogBuffer
		command.Stdout = &logs
		command.Stderr = &logs
		require.NoError(t, command.Start())
		done := make(chan error, 1)
		go func() { done <- command.Wait() }()
		t.Cleanup(func() { _ = command.Process.Kill() })
		require.Eventually(t, func() bool { _, e := client.GetVersion(); return e == nil && readSettings()["mode"] == "'manual'" }, 15*time.Second, 100*time.Millisecond, "%s", logs.String())
		return command, done
	}
	run := func(args ...string) []byte {
		out, e := exec.CommandContext(ctx, app, args...).CombinedOutput()
		require.NoError(t, e, string(out))
		return out
	}
	daemon, done := start()
	var state CoreStatus
	require.NoError(t, json.Unmarshal(run("core", "status", "--output", "json"), &state))
	require.True(t, state.Running)
	require.Equal(t, "service", state.Runtime)
	other := NewCoreService()
	other.Close()
	select {
	case e := <-done:
		t.Fatalf("client close stopped background owner: %v", e)
	default:
	}
	ps := NewProfileService(client, frontend)
	p, err := ps.AddProfile("background", profile.SubSource{Kind: profile.SourceLocal, Path: "unused"})
	require.NoError(t, err)
	require.NoError(t, profile.WriteRaw(p.UID, []byte("proxies: []\nrules:\n  - MATCH,DIRECT\n")))
	_, err = frontend.ApplyProfile(ctx, p.UID, true)
	require.NoError(t, err)
	if previous != "" {
		require.NoError(t, frontend.ActivateVersion(ctx, "v1.19.32"))
		info, e := client.GetVersion()
		require.NoError(t, e)
		require.Equal(t, "v1.19.32", info.Version)
		require.NoError(t, frontend.UpgradeVersion(ctx, version))
		info, e = client.GetVersion()
		require.NoError(t, e)
		require.Equal(t, version, info.Version)
		require.NoError(t, frontend.ActivateVersion(ctx, ""))
		info, e = client.GetVersion()
		require.NoError(t, e)
		require.Equal(t, "v1.19.32", info.Version)
		require.NoError(t, frontend.UpgradeVersion(ctx, version))
		info, e = client.GetVersion()
		require.NoError(t, e)
		require.Equal(t, version, info.Version)
	}
	run("core", "restart", "--output", "json")
	// SIGKILL bypasses every application defer: the independent guard must restore
	// settings after the coordinator lock closes and the owned child is killed.
	require.NoError(t, daemon.Process.Kill())
	require.Error(t, <-done)
	require.Eventually(t, func() bool { return reflect.DeepEqual(original, readSettings()) }, 10*time.Second, 100*time.Millisecond)
	_, done = start()
	run("core", "stop", "--output", "json")
	require.NoError(t, <-done)
	require.Eventually(t, func() bool { return reflect.DeepEqual(original, readSettings()) }, 5*time.Second, 100*time.Millisecond)
	_, err = os.Stat(networkJournal(root))
	require.True(t, os.IsNotExist(err))
}
