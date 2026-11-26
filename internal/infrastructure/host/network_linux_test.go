package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestProxyBackendPreservesValuesAndWritesModeLast(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/isolated/test")
	var writes []string
	n := &DesktopNetwork{Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		require.Equal(t, "gsettings", name)
		if args[0] == "get" {
			return []byte("'original'\n"), nil
		}
		writes = append(writes, args[2])
		return nil, nil
	}}
	before, err := n.ReadProxy(context.Background())
	require.NoError(t, err)
	require.Len(t, before, len(desktopKeys))
	require.NoError(t, n.WriteProxy(context.Background(), n.ProxyTarget(12345)))
	require.Equal(t, "mode", writes[len(writes)-1])
}
func TestHeadlessProxyFailsBeforeExecutingSettings(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	n := &DesktopNetwork{Run: func(context.Context, string, ...string) ([]byte, error) { t.Fatal("no desktop bus"); return nil, nil }}
	_, err := n.ReadProxy(context.Background())
	require.ErrorContains(t, err, "D-Bus")
}
func TestMissingGSettingsExplainsHeadlessFallback(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/isolated/test")
	n := &DesktopNetwork{Run: func(context.Context, string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("gsettings failed: %w", &exec.Error{Name: "gsettings", Err: exec.ErrNotFound})
	}}
	_, err := n.ReadProxy(context.Background())
	require.ErrorContains(t, err, "requires gsettings")
	require.ErrorContains(t, err, "mihosh proxy on/off")
	require.NotContains(t, err.Error(), "executable file not found")
	err = n.WriteProxy(context.Background(), n.ProxyTarget(7890))
	require.ErrorContains(t, err, "requires gsettings")
}
func TestTUNCleanupOnlyDeletesReservedPriorities(t *testing.T) {
	identity := TUNIdentity(t.TempDir())
	var mutations []string
	n := &DesktopNetwork{Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		require.Equal(t, "ip", name)
		if strings.Contains(strings.Join(args, " "), "rule show") {
			return json.Marshal([]map[string]any{{"priority": identity.Rule}, {"priority": identity.Rule + 10}, {"priority": identity.Rule + 11}, {"priority": 32766}})
		}
		mutations = append(mutations, strings.Join(args, " "))
		return nil, nil
	}}
	require.NoError(t, n.CleanupTUN(context.Background(), identity))
	require.Len(t, mutations, 4)
	for _, mutation := range mutations {
		require.True(t, strings.HasSuffix(mutation, "priority "+strconv.Itoa(identity.Rule)) || strings.HasSuffix(mutation, "priority "+strconv.Itoa(identity.Rule+10)))
	}
}
func TestCheckTUNReportsSetcapCommandWhenUnprivileged(t *testing.T) {
	if _, err := os.Stat("/dev/net/tun"); err != nil {
		t.Skip("/dev/net/tun unavailable in test environment")
	}
	if hasEffectiveCap(unix.CAP_NET_ADMIN) {
		t.Skip("process already holds CAP_NET_ADMIN")
	}
	n := &DesktopNetwork{}
	err := n.CheckTUN(context.Background(), "/tmp/fake/mihomo", TUNIdentity(t.TempDir()))
	require.Error(t, err)
	require.Contains(t, err.Error(), "sudo setcap cap_net_admin,cap_net_raw+ep ")
	require.NotContains(t, err.Error(), "/tmp/fake/mihomo")
}

func TestCleanupTUN_SkipsDisabledIPv6(t *testing.T) {
	identity := TUNIdentity(t.TempDir())
	var ipv4Deleted int
	n := &DesktopNetwork{Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		require.Equal(t, "ip", name)
		joined := strings.Join(args, " ")
		if strings.HasPrefix(joined, "-6 ") {
			return nil, errors.New("RTNETLINK answers: Address family not supported by protocol")
		}
		if strings.Contains(joined, "rule show") {
			return json.Marshal([]map[string]any{{"priority": identity.Rule}})
		}
		if strings.Contains(joined, "rule del") {
			ipv4Deleted++
		}
		return nil, nil
	}}
	require.NoError(t, n.CleanupTUN(context.Background(), identity))
	require.Equal(t, 1, ipv4Deleted)
}
