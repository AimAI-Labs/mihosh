//go:build !linux && !darwin

package host

import "os/exec"

func GracefulCancel(*exec.Cmd) {}
