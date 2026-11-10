package host

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcListenerExecutableMatchesControllerAndHandlesMissingOrAmbiguousOwners(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "net"), 0700))
	// Different ports and addresses must not match the selected controller.
	table := "0: 0100007F:2382 00000000:0000 0A 0:0 0:0 0 0 0 111\n" +
		"1: 0100007F:2383 00000000:0000 0A 0:0 0:0 0 0 0 222\n" +
		"2: 0200007F:2382 00000000:0000 0A 0:0 0:0 0 0 0 333\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "net", "tcp"), []byte(table), 0600))
	process := func(pid, inode, binary string) {
		dir := filepath.Join(root, pid)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "fd"), 0700))
		require.NoError(t, os.Symlink("socket:["+inode+"]", filepath.Join(dir, "fd", "4")))
		if binary != "" {
			require.NoError(t, os.Symlink(binary, filepath.Join(dir, "exe")))
		}
	}
	process("100", "111", "/opt/my core/mihomo")
	process("200", "222", "/other/mihomo")
	process("300", "333", "/wrong-address/mihomo")
	ips := []net.IP{net.ParseIP("127.0.0.1")}
	require.Equal(t, "/opt/my core/mihomo", procListenerExecutable(root, ips, 9090))
	require.Empty(t, procListenerExecutable(root, ips, 9999))
	process("400", "111", "/ambiguous/mihomo")
	require.Empty(t, procListenerExecutable(root, ips, 9090))
	require.NoError(t, os.RemoveAll(filepath.Join(root, "400")))
	require.NoError(t, os.Remove(filepath.Join(root, "100", "exe")))
	require.Empty(t, procListenerExecutable(root, ips, 9090))
	// IPv6 loopback and wildcard listeners use the same inode ownership lookup.
	for _, address := range []string{"00000000000000000000000001000000", "00000000000000000000000000000000"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, "net", "tcp6"), []byte("0: "+address+":2382 0:0 0A 0:0 0:0 0 0 0 555\n"), 0600))
		if _, err := os.Stat(filepath.Join(root, "500")); os.IsNotExist(err) {
			process("500", "555", "/ipv6/mihomo")
		}
		require.Equal(t, "/ipv6/mihomo", procListenerExecutable(root, []net.IP{net.IPv6loopback}, 9090))
	}
}
