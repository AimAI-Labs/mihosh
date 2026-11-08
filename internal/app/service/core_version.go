package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"golang.org/x/mod/semver"
)

func (s *CoreService) ActivateVersion(ctx context.Context, version string) error {
	return s.activateVersion(ctx, version, false)
}
func (s *CoreService) UpgradeVersion(ctx context.Context, version string) error {
	return s.activateVersion(ctx, version, true)
}

func (s *CoreService) activateVersion(ctx context.Context, version string, upgrade bool) error {
	ctx, cancelOperation := s.operationContext(ctx)
	defer cancelOperation()
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	rootForLock, lockErr := config.GetConfigDir()
	if lockErr != nil {
		return lockErr
	}
	releaseMutation, lockErr := s.lockCoreMutation(rootForLock)
	if lockErr != nil {
		return lockErr
	}
	defer releaseMutation()

	cfg, err := loadCoreConfig()
	if err != nil {
		return err
	}
	if version == "" {
		version = cfg.Core.PreviousVersion
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	binary, err := coreinfra.BinaryPath(root, version, runtime.GOOS)
	if err != nil {
		return err
	}
	if _, err = os.Stat(binary); err != nil {
		return err
	}
	old := cfg.Core
	if upgrade && old.Version != "" && semver.Compare(version, old.Version) < 0 {
		return errors.New("upgrade metadata is older than the selected version; use explicit activation or rollback")
	}
	if version == old.Version {
		return nil
	}
	validationCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output := &coreinfra.LogBuffer{}
	command := exec.CommandContext(validationCtx, binary, "-v")
	command.Stdout = output
	command.Stderr = output
	coreinfra.ConfigureProcess(command)
	if err = command.Run(); err != nil || !strings.Contains(" "+output.String()+" ", " "+version+" ") {
		return errors.New("candidate executable version check failed")
	}
	if path, e := config.ManagedMihomoConfigPath(); e == nil {
		if _, e = os.Stat(path); e == nil {
			command = exec.CommandContext(validationCtx, binary, "-t", "-d", filepath.Dir(path), "-f", path)
			coreinfra.ConfigureProcess(command)
			if err = command.Run(); err != nil {
				return errors.New("candidate core configuration validation failed")
			}
		}
	}
	wasRunning := s.process != nil
	if wasRunning {
		select {
		case <-s.done:
			wasRunning = false
			s.stopLocked()
		default:
		}
	}
	if wasRunning {
		s.stopLocked()
	}
	pending := ""
	if wasRunning {
		pending = old.Version
	}
	if err = config.Update(func(latest *config.Config) error {
		latest.Core.Version = version
		latest.Core.PreviousVersion = old.Version
		latest.Core.PendingVersion = pending
		return nil
	}); err != nil {
		if wasRunning {
			_ = s.start(context.Background(), true)
		}
		return err
	}
	if !wasRunning {
		return nil
	}
	if err = s.start(ctx, true); err == nil {
		err = config.Update(func(latest *config.Config) error { latest.Core.PendingVersion = ""; return nil })
		if err == nil {
			return nil
		}
		s.stopLocked()
	}
	restoreErr := config.Update(func(latest *config.Config) error {
		latest.Core.Version = old.Version
		latest.Core.PreviousVersion = old.PreviousVersion
		latest.Core.PendingVersion = old.PendingVersion
		return nil
	})
	var restartErr error
	if restoreErr == nil {
		restartErr = s.start(context.Background(), true)
	}
	return errors.Join(fmt.Errorf("candidate core failed: %w", err), restoreErr, restartErr)
}
