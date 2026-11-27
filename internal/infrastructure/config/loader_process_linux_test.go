package config

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLinuxDiscoveryResolvesRelativeDirectoryFromProcessCWD(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "real config")
	require.NoError(t, os.Mkdir(configDir, 0700))
	configFile := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("mixed-port: 7890\n"), 0600))

	child := exec.Command("/bin/sh", "-c", "echo ready; read -r input", "--", "-d", "./real config")
	child.Args[0] = "/opt/Clash Verge/verge-mihomo"
	child.Dir = root
	stdin, err := child.StdinPipe()
	require.NoError(t, err)
	stdout, err := child.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, child.Start())
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = child.Wait()
	})
	ready, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "ready\n", ready)
	ps := fmt.Sprintf("%d %s -d ./real config\n", child.Process.Pid, child.Args[0])
	require.Equal(t, configFile, parseMihomoConfigFromProcessList(ps))
}
