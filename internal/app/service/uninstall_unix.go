//go:build !windows

package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/unix"
)

type binaryRemoveOutcome struct {
	Removed       bool
	Deferred      bool
	NeedsManual   bool
	ManualCommand string
	Err           error
}

func isDirWritable(dir string) bool {
	if dir == "" {
		return false
	}
	return unix.Access(dir, unix.W_OK) == nil
}

func removeExecutableFile(ctx context.Context, path string, allowInteractiveSudo bool) binaryRemoveOutcome {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return binaryRemoveOutcome{Removed: true}
	}
	if !errors.Is(err, os.ErrPermission) {
		return binaryRemoveOutcome{Err: err}
	}
	manual := "sudo rm -f " + quoteShellProxy(path)
	if os.Geteuid() != 0 {
		if _, lookErr := exec.LookPath("sudo"); lookErr == nil {
			nonInteractiveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			cmd := exec.CommandContext(nonInteractiveCtx, "sudo", "-n", "rm", "-f", "--", path)
			cmd.Stdin = nil
			runErr := cmd.Run()
			cancel()
			if runErr == nil {
				return binaryRemoveOutcome{Removed: true}
			}
			if allowInteractiveSudo {
				interactive := exec.CommandContext(ctx, "sudo", "rm", "-f", "--", path)
				interactive.Stdin = os.Stdin
				interactive.Stdout = os.Stdout
				interactive.Stderr = os.Stderr
				if interactive.Run() == nil {
					return binaryRemoveOutcome{Removed: true}
				}
			}
		}
	}
	return binaryRemoveOutcome{
		NeedsManual:   true,
		ManualCommand: manual,
	}
}
