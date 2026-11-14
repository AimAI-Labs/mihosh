//go:build linux

package service

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/host"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

// Explicit opt-in only. Native service tests create a uniquely named service for
// a temporary root. TUN tests require an isolated Linux network namespace.
func nativeFixture(t *testing.T) (string, string, func(...string) ([]byte, error)) {
	t.Helper()
	app, source := os.Getenv("MIHOSH_INTEGRATION_APP"), os.Getenv("MIHOSH_INTEGRATION_BINARY")
	require.NotEmpty(t, app, "set MIHOSH_INTEGRATION_APP to the built application")
	require.NotEmpty(t, source, "set MIHOSH_INTEGRATION_BINARY to a verified kernel")
	root, err := os.MkdirTemp("", "mihosh-native-")
	require.NoError(t, err)
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	// Keep root on failure for diagnostics and recovery; no automatic removal can
	// leave an installed service pointing at a deleted executable.
	t.Cleanup(func() {
		if !t.Failed() {
			_ = os.RemoveAll(root)
		} else {
			t.Logf("native test data retained at %s", root)
		}
	})
	version := coreinfra.PinnedRelease().Tag
	binary, err := coreinfra.BinaryPath(root, version, runtime.GOOS)
	require.NoError(t, err)
	data, err := os.ReadFile(source)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(binary), 0700))
	require.NoError(t, os.WriteFile(binary, data, 0700))
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.Version = version; return nil }))
	s := NewCoreService()
	require.NoError(t, s.Configure())
	s.Close()
	freePort := func() int {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, e)
		port := l.Addr().(*net.TCPAddr).Port
		_ = l.Close()
		return port
	}
	s = NewCoreService()
	require.NoError(t, s.SetPorts(freePort(), freePort()))
	s.Close()
	run := func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
		defer cancel()
		return exec.CommandContext(ctx, app, args...).CombinedOutput()
	}
	return root, app, run
}

func TestNativeManagedService(t *testing.T) {
	if os.Getenv("MIHOSH_NATIVE_SERVICE_TEST") != "1" {
		t.Skip("set MIHOSH_NATIVE_SERVICE_TEST=1 on a native service-manager host")
	}
	root, app, run := nativeFixture(t)
	installed := false
	t.Cleanup(func() {
		if installed {
			out, err := run("core", "service", "uninstall", "--output", "json")
			if err != nil {
				t.Errorf("native service cleanup failed: %v %s", err, out)
			}
		}
	})
	check := func(args ...string) []byte {
		out, err := run(args...)
		require.NoError(t, err, string(out))
		return out
	}
	args := []string{"core", "service", "install", "--output", "json"}

	check(args...)
	installed = true
	unit := host.ServiceName(root) + ".service"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	_, err := host.Run(ctx, "systemctl", "--user", "set-property", "--runtime", unit, "MemoryMax=256M", "TasksMax=64")
	cancel()
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = host.Run(ctx, "systemctl", "--user", "revert", unit)
	})
	check("core", "service", "enable", "--output", "json")
	check("core", "start", "--output", "json")
	var status CoreStatus
	require.NoError(t, json.Unmarshal(check("core", "status", "--output", "json"), &status))
	require.True(t, status.Running)
	require.True(t, status.ServiceInstalled)
	require.True(t, status.AutoStart)
	check("core", "restart", "--output", "json")
	// Exercise the real transient guard with an empty record: this changes only
	// fixture files, never this host's proxy, DNS or routes. Hold the config lock
	// so we can prove the guard survives its primary cgroup being killed.
	unlockConfig, err := coreinfra.LockRuntime(filepath.Join(root, "runtime", "config.lock"))
	require.NoError(t, err)
	var releaseOnce sync.Once
	releaseConfig := func() { releaseOnce.Do(unlockConfig) }
	t.Cleanup(releaseConfig)
	require.NoError(t, writeNetwork(root, networkRecovery{}))
	challenge := []byte("native-service-guard-challenge")
	require.NoError(t, privateWrite(filepath.Join(root, "runtime", "guard-challenge"), challenge))
	t.Setenv("INVOCATION_ID", "native-service-test")
	guard := host.GuardCommand(app, root, true)
	separator := 0
	for i, arg := range guard.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	require.Positive(t, separator)
	argsWithLimits := append([]string{}, guard.Args[:separator]...)
	argsWithLimits = append(argsWithLimits, "--property=MemoryMax=128M", "--property=TasksMax=32")
	guard.Args = append(argsWithLimits, guard.Args[separator:]...)
	output, err := guard.CombinedOutput()
	require.NoError(t, err, string(output))
	guardUnit := host.ServiceName(root) + "-guard.service"
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = host.Run(ctx, "systemctl", "--user", "stop", guardUnit)
	})
	require.Eventually(t, func() bool {
		data, _ := os.ReadFile(filepath.Join(root, "runtime", "guard-ready"))
		return string(data) == string(challenge)
	}, 5*time.Second, 50*time.Millisecond)
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	beforePID, err := host.Run(ctx, "systemctl", "--user", "show", unit, "--property=MainPID", "--value")
	cancel()
	require.NoError(t, err)
	require.NotEqual(t, "0", strings.TrimSpace(string(beforePID)))
	// Signal only this fixture's uniquely named cgroup. The existing user's core
	// is never selected by process name or by the legacy mihomo.service unit.
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	_, err = host.Run(ctx, "systemctl", "--user", "kill", "--kill-whom=all", "--signal=SIGKILL", unit)
	cancel()
	require.NoError(t, err)
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	guardState, err := host.Run(ctx, "systemctl", "--user", "show", guardUnit, "--property=ActiveState", "--value")
	cancel()
	require.NoError(t, err)
	require.Equal(t, "active", strings.TrimSpace(string(guardState)))
	releaseConfig()
	require.Eventually(t, func() bool { _, e := os.Stat(networkJournal(root)); return os.IsNotExist(e) }, 5*time.Second, 50*time.Millisecond)
	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		pid, e := host.Run(ctx, "systemctl", "--user", "show", unit, "--property=MainPID", "--value")
		if e != nil || strings.TrimSpace(string(pid)) == "0" || string(pid) == string(beforePID) {
			return false
		}
		data, e := run("core", "status", "--output", "json")
		var current CoreStatus
		return e == nil && json.Unmarshal(data, &current) == nil && current.Running
	}, 15*time.Second, 100*time.Millisecond, "service did not recover after its own coordinator was killed")
	// A real candidate with a deliberately incorrect tag must be rejected without
	// stopping the running background service or changing the selected version.
	bad, err := coreinfra.BinaryPath(root, "v0.0.0", runtime.GOOS)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(bad), 0700))
	data, err := os.ReadFile(os.Getenv("MIHOSH_INTEGRATION_BINARY"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(bad, data, 0700))
	_, err = run("core", "activate", "--version", "v0.0.0", "--output", "json")
	require.Error(t, err)
	require.NoError(t, json.Unmarshal(check("core", "status", "--output", "json"), &status))
	require.True(t, status.Running)
	require.Equal(t, coreinfra.PinnedRelease().Tag, status.Version)
	check("core", "stop", "--output", "json")
	check("core", "service", "disable", "--output", "json")
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	_, err = host.Run(ctx, "systemctl", "--user", "revert", unit)
	cancel()
	require.NoError(t, err)
	check("core", "service", "uninstall", "--output", "json")
	installed = false
}

func TestNativeManagedTUN(t *testing.T) {
	if os.Getenv("MIHOSH_NATIVE_TUN_TEST") != "1" {
		t.Skip("set MIHOSH_NATIVE_TUN_TEST=1 in an isolated Linux network namespace")
	}

	current, e := os.Readlink("/proc/self/ns/net")
	require.NoError(t, e)
	initial, e := os.Readlink("/proc/1/ns/net")
	require.NoError(t, e)
	require.NotEqual(t, initial, current, "native TUN test must not alter the host network namespace")

	root, app, run := nativeFixture(t)
	check := func(args ...string) []byte {
		out, err := run(args...)
		require.NoError(t, err, string(out))
		return out
	}
	check("core", "tun", "on", "--output", "json")
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, app, "core", "start", "--output", "json")
	var logs coreinfra.LogBuffer
	command.Stdout = &logs
	command.Stderr = &logs
	require.NoError(t, command.Start())
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, err := run("core", "network-recover", "--output", "json")
		if err != nil {
			t.Errorf("TUN recovery failed: %v", err)
		}
	})
	identity := host.TUNIdentity(root)
	require.Eventually(t, func() bool { _, e := host.NewNetwork().CaptureTUN(ctx, identity); return e == nil }, 20*time.Second, 100*time.Millisecond, "%s", logs.String())
	svc := NewCoreService()
	defer svc.Close()
	profiles := NewProfileService(nil, svc)
	p, err := profiles.AddProfile("native-tun", profile.SubSource{Kind: profile.SourceLocal, Path: "unused"})
	require.NoError(t, err)
	require.NoError(t, profile.WriteRaw(p.UID, []byte("proxies: []\nrules:\n  - MATCH,DIRECT\n")))
	_, err = svc.ApplyProfile(ctx, p.UID, true)
	require.NoError(t, err)
	record, err := svc.readNetwork(root)
	require.NoError(t, err)
	active, err := host.NewNetwork().CaptureTUN(ctx, identity)
	require.NoError(t, err)
	require.Equal(t, active.Index, record.TUN.Index)
	// Turning TUN off keeps the foreground coordinator alive across replacement.
	check("core", "tun", "off", "--output", "json")
	require.Eventually(t, func() bool { return nativeTUNClean(t, identity) }, 10*time.Second, 100*time.Millisecond)
	check("core", "tun", "on", "--output", "json")
	require.Eventually(t, func() bool { _, e := host.NewNetwork().CaptureTUN(ctx, identity); return e == nil }, 20*time.Second, 100*time.Millisecond)
	require.NoError(t, command.Process.Kill())
	require.Error(t, <-done)
	require.Eventually(t, func() bool {
		_, e := os.Stat(networkJournal(root))
		return os.IsNotExist(e) && nativeTUNClean(t, identity)
	}, 20*time.Second, 100*time.Millisecond)
}
func nativeTUNClean(t *testing.T, identity host.TUN) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := net.InterfaceByName(identity.Device); err == nil {
		return false
	}
	for _, family := range []string{"-4", "-6"} {
		data, err := host.Run(ctx, "ip", family, "-json", "rule", "show")
		if err != nil {
			return false
		}
		var rules []map[string]any
		if json.Unmarshal(data, &rules) != nil {
			return false
		}
		for _, rule := range rules {
			value, _ := rule["priority"].(float64)
			if int(value) >= identity.Rule && int(value) <= identity.Rule+10 {
				return false
			}
		}
	}
	return true

}

func TestNativeManagedSystemProxy(t *testing.T) {
	if os.Getenv("MIHOSH_NATIVE_PROXY_TEST") != "1" {
		t.Skip("set MIHOSH_NATIVE_PROXY_TEST=1 on an acknowledged desktop test host")
	}
	require.Equal(t, "1", os.Getenv("MIHOSH_ALLOW_NETWORK_CHANGES"), "native proxy test changes user desktop settings")
	root, app, run := nativeFixture(t)
	network := host.NewNetwork()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	before, err := network.ReadProxy(ctx)
	require.NoError(t, err)
	check := func(args ...string) { out, err := run(args...); require.NoError(t, err, string(out)) }
	check("core", "system-proxy", "on", "--output", "json")
	command := exec.CommandContext(ctx, app, "core", "start", "--output", "json")
	var logs coreinfra.LogBuffer
	command.Stdout = &logs
	command.Stderr = &logs
	require.NoError(t, command.Start())
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, err := run("core", "network-recover", "--output", "json")
		if err != nil {
			t.Errorf("desktop proxy recovery failed: %v", err)
		}
	})
	target := network.ProxyTarget(config.ResolveMihomoEndpoint().MixedPort)
	require.Eventually(t, func() bool {
		settings, e := network.ReadProxy(ctx)
		return e == nil && reflect.DeepEqual(settings, target)
	}, 15*time.Second, 100*time.Millisecond)
	check("core", "system-proxy", "off", "--output", "json")
	restored, err := network.ReadProxy(ctx)
	require.NoError(t, err)
	require.Equal(t, before, restored)
	check("core", "system-proxy", "on", "--output", "json")
	require.NoError(t, command.Process.Kill())
	require.Error(t, <-done)
	require.Eventually(t, func() bool {
		settings, e := network.ReadProxy(ctx)
		_, journalErr := os.Stat(networkJournal(root))
		return e == nil && reflect.DeepEqual(settings, before) && os.IsNotExist(journalErr)
	}, 15*time.Second, 100*time.Millisecond)
}
