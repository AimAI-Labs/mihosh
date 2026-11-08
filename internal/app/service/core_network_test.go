//go:build linux

package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/host"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type fakeCoreNetwork struct {
	mu     sync.Mutex
	values host.ProxySettings
	fail   bool
	clean  []host.TUN
	writes int
}

func fakeSettings(server, mode string) host.ProxySettings {
	return host.ProxySettings{"server": {Data: []byte(server), Exists: true}, "mode": {Data: []byte(mode), Exists: true}}
}
func (n *fakeCoreNetwork) ReadProxy(context.Context) (host.ProxySettings, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := host.ProxySettings{}
	for k, v := range n.values {
		out[k] = v
	}
	return out, nil
}
func (*fakeCoreNetwork) ProxyTarget(int) host.ProxySettings { return fakeSettings("mihosh", "manual") }
func (n *fakeCoreNetwork) WriteProxy(_ context.Context, values host.ProxySettings) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.writes++
	if n.fail {
		return errors.New("desktop temporarily unavailable")
	}
	for k, v := range values {
		n.values[k] = v
	}
	return nil
}
func (*fakeCoreNetwork) CheckTUN(context.Context, string, host.TUN) error { return nil }
func (*fakeCoreNetwork) CaptureTUN(_ context.Context, value host.TUN) (host.TUN, error) {
	value.Index = 99
	return value, nil
}
func (n *fakeCoreNetwork) CleanupTUN(_ context.Context, value host.TUN) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.clean = append(n.clean, value)
	return nil
}
func networkService(t *testing.T) (*CoreService, *fakeCoreNetwork, string) {
	t.Helper()
	newIsolatedService(t)
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "runtime"), 0700))
	n := &fakeCoreNetwork{values: fakeSettings("mihosh", "manual")}
	s := NewCoreService()
	s.network = n
	s.startGuard = func(string) error { return nil }
	t.Cleanup(s.Close)
	return s, n, root
}
func TestNetworkRecoveryRestoresOnlyUnchangedOwnedSettings(t *testing.T) {
	s, n, root := networkService(t)
	identity := host.TUNIdentity(root)
	require.NoError(t, writeNetwork(root, networkRecovery{Before: fakeSettings("old", "auto"), Applied: n.ProxyTarget(7890), TUN: &identity}))
	n.values["server"] = host.Setting{Data: []byte("user-edited"), Exists: true}
	require.NoError(t, s.RecoverNetwork(context.Background()))
	current, _ := n.ReadProxy(context.Background())
	require.Equal(t, "user-edited", string(current["server"].Data))
	require.Equal(t, "auto", string(current["mode"].Data))
	require.Equal(t, []host.TUN{identity}, n.clean)
	_, err := os.Stat(networkJournal(root))
	require.True(t, os.IsNotExist(err))
}
func TestNetworkRecoveryFailureRetainsRecordForRetry(t *testing.T) {
	s, n, root := networkService(t)
	require.NoError(t, writeNetwork(root, networkRecovery{Before: fakeSettings("old", "auto"), Applied: n.ProxyTarget(1)}))
	n.fail = true
	require.Error(t, s.RecoverNetwork(context.Background()))
	_, err := os.Stat(networkJournal(root))
	require.NoError(t, err)
	n.fail = false
	require.NoError(t, s.RecoverNetwork(context.Background()))
	current, _ := n.ReadProxy(context.Background())
	require.Equal(t, fakeSettings("old", "auto"), current)
}
func TestNetworkRecoveryRejectsLiveOwnerAndInvalidIdentity(t *testing.T) {
	s, n, root := networkService(t)
	identity := host.TUNIdentity(root)
	identity.Device = "unrelated"
	require.NoError(t, writeNetwork(root, networkRecovery{TUN: &identity}))
	require.Error(t, s.RecoverNetwork(context.Background()))
	require.Empty(t, n.clean)
	identity = host.TUNIdentity(root)
	require.NoError(t, writeNetwork(root, networkRecovery{TUN: &identity}))
	unlock, err := coreinfra.LockRuntime(filepath.Join(root, "runtime", "process.lock"))
	require.NoError(t, err)
	require.Error(t, s.RecoverNetwork(context.Background()))
	require.Empty(t, n.clean)
	unlock()
	require.NoError(t, s.RecoverNetwork(context.Background()))
}
func TestNetworkGuardWaitsForOwnerAndRecoversAfterLockRelease(t *testing.T) {
	s, n, root := networkService(t)
	require.NoError(t, writeNetwork(root, networkRecovery{Before: fakeSettings("old", "auto"), Applied: n.ProxyTarget(1)}))
	unlock, err := coreinfra.LockRuntime(filepath.Join(root, "runtime", "process.lock"))
	require.NoError(t, err)
	require.NoError(t, privateWrite(filepath.Join(root, "runtime", "guard-challenge"), []byte("test-challenge")))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- s.GuardNetwork(ctx) }()
	require.Eventually(t, func() bool {
		data, _ := os.ReadFile(filepath.Join(root, "runtime", "guard-ready"))
		return string(data) == "test-challenge"
	}, time.Second, 10*time.Millisecond)
	n.mu.Lock()
	require.Zero(t, n.writes)
	n.mu.Unlock()
	unlock()
	require.NoError(t, <-finished)
	current, _ := n.ReadProxy(context.Background())
	require.Equal(t, fakeSettings("old", "auto"), current)
}
func TestManagedNetworkSettingsOverrideSubscriptionTUNAndKeepDNS(t *testing.T) {
	root := t.TempDir()
	data, err := applyManagedNetwork([]byte("tun: {enable: true, device: unrelated}\ndns: {enable: false, listen: '0.0.0.0:53', nameserver: [1.2.3.4]}\n"), true, root)
	require.NoError(t, err)
	var mapping map[string]any
	require.NoError(t, yaml.Unmarshal(data, &mapping))
	tun := mapping["tun"].(map[string]any)
	require.Equal(t, host.TUNIdentity(root).Device, tun["device"])
	require.Equal(t, false, tun["auto-redirect"])
	dns := mapping["dns"].(map[string]any)
	require.NotContains(t, dns, "listen")
	require.Equal(t, []any{"1.2.3.4"}, dns["nameserver"])
	data, err = applyManagedNetwork(data, false, root)
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &mapping))
	require.Equal(t, false, mapping["tun"].(map[string]any)["enable"])
}
func TestInterruptedTUNChangeRestoresConfigAndPreference(t *testing.T) {
	_, _, root := networkService(t)
	old := []byte("tun: {enable: false}\n")
	require.NoError(t, privateWrite(filepath.Join(root, "runtime", "config.yaml"), []byte("tun: {enable: true}\n")))
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.TUN = true; return nil }))
	data, err := json.Marshal(networkChange{Config: old, TUN: false})
	require.NoError(t, err)
	require.NoError(t, privateWrite(filepath.Join(root, "runtime", "network-change.json"), data))
	require.NoError(t, recoverNetworkChange(root))
	actual, err := os.ReadFile(filepath.Join(root, "runtime", "config.yaml"))
	require.NoError(t, err)
	require.Equal(t, old, actual)
	cfg, err := config.Load()
	require.NoError(t, err)
	require.False(t, cfg.Core.TUN)
}
