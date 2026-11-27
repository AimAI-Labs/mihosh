//go:build windows

package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

func removeExecutableFile(_ context.Context, path string, _ bool) binaryRemoveOutcome {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return binaryRemoveOutcome{Removed: true}
	}
	cmd := exec.Command("cmd.exe", "/C", `ping -n 3 127.0.0.1 >NUL & del /F /Q "%MIHOSH_UNINSTALL_TARGET%"`)
	cmd.Env = append(os.Environ(), "MIHOSH_UNINSTALL_TARGET="+path)
	if startErr := cmd.Start(); startErr == nil {
		return binaryRemoveOutcome{Deferred: true}
	}
	return binaryRemoveOutcome{
		NeedsManual:   true,
		ManualCommand: fmt.Sprintf("del /F /Q %q", path),
	}
}
