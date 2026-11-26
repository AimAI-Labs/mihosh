//go:build darwin

package core

import (
	"errors"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

func ConfigureProcess(*exec.Cmd) {}

func LockRuntime(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, ErrRuntimeLocked
		}
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}

func AttachProcess(*exec.Cmd) (func(), error) { return func() {}, nil }
