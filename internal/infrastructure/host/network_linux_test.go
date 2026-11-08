package host

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
