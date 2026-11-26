//go:build !linux && !windows && !darwin

package core

import (
	"errors"
	"os/exec"
)

func ConfigureProcess(*exec.Cmd) {}
func LockRuntime(string) (func(), error) {
	return nil, errors.New("managed core runtime is supported on Linux and Windows")
}

func AttachProcess(*exec.Cmd) (func(), error) { return func() {}, nil }
