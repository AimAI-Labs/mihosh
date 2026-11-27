//go:build linux || darwin

package host

import (
	"bufio"
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGracefulCancelSendsSIGTERM(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", "trap 'exit 0' TERM; echo ready; while true; do sleep 0.05; done")
	cmd.WaitDelay = time.Second
	GracefulCancel(cmd)
	out, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	line, readErr := bufio.NewReader(out).ReadString('\n')
	cancel()
	waitErr := cmd.Wait()
	require.NoError(t, readErr)
	require.Equal(t, "ready\n", line)
	require.ErrorIs(t, waitErr, context.Canceled)
	require.Zero(t, cmd.ProcessState.ExitCode())
}
