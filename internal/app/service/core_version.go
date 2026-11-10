package service

import (
	"context"
	"errors"
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
	if forwarded, err := s.forward(ctx, "activate", version); forwarded {
		return err
	}
	return s.activateVersion(ctx, version, false)
}
func (s *CoreService) UpgradeVersion(ctx context.Context, version string) error {
	if forwarded, err := s.forward(ctx, "upgrade", version); forwarded {
		return err
	}
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
	return s.replaceCore(ctx, &versionReplacement{previous: old, version: version})
}

type versionReplacement struct {
	previous config.CoreConfig
	version  string
	running  bool
	applied  bool
}

func (c *versionReplacement) apply(running bool) error {
	c.running = running
	pending := ""
	if running {
		pending = c.previous.Version
	}
	err := config.Update(func(latest *config.Config) error {
		latest.Core.Version = c.version
		latest.Core.PreviousVersion = c.previous.Version
		latest.Core.PendingVersion = pending
		return nil
	})
	c.applied = err == nil
	return err
}

func (c *versionReplacement) commit() error {
	if !c.running {
		return nil
	}
	return config.Update(func(latest *config.Config) error { latest.Core.PendingVersion = ""; return nil })
}

func (c *versionReplacement) restore() error {
	if !c.applied {
		return nil
	}
	return config.Update(func(latest *config.Config) error {
		latest.Core.Version = c.previous.Version
		latest.Core.PreviousVersion = c.previous.PreviousVersion
		latest.Core.PendingVersion = c.previous.PendingVersion
		return nil
	})
}
