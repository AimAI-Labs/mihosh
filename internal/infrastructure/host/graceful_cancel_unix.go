//go:build linux || darwin

package host

import (
	"os/exec"
	"syscall"
)

func GracefulCancel(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
}
