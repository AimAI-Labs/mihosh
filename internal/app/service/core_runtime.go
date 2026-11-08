package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
)

func (s *CoreService) Start(ctx context.Context) error {
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	return s.start(ctx)
}
func (s *CoreService) start(ctx context.Context, skipRecovery ...bool) error {
	ctx, cancelOperation := s.operationContext(ctx)
	defer cancelOperation()
	if s.process != nil {
		select {
		case <-s.done:
			s.stopLocked()
		default:
			return errors.New("managed core is already running")
		}
	}
	cfg, err := loadCoreConfig()
	if err != nil {
		return err
	}
	if cfg.Core.Mode != "managed" {
		return errors.New("enable managed mode before starting the core")
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	binary, err := coreinfra.BinaryPath(root, cfg.Core.Version, runtime.GOOS)
	if err != nil {
		return err
	}
	path, err := config.ManagedMihomoConfigPath()
	if err != nil {
		return err
	}
	unlock, err := coreinfra.LockRuntime(filepath.Join(root, "runtime", "process.lock"))
	if err != nil {
		return err
	}
	keepLock := false
	defer func() {
		if !keepLock {
			unlock()
		}
	}()
	if len(skipRecovery) == 0 || !skipRecovery[0] {
		unlockConfig, e := coreinfra.LockRuntime(filepath.Join(root, "runtime", "config.lock"))
		if e != nil {
			return e
		}
		cfg, err = loadCoreConfig()
		if err != nil {
			unlockConfig()
			return err
		}
		if cfg.Core.PendingVersion != "" {
			if err = config.Update(func(latest *config.Config) error {
				if latest.Core.PendingVersion != "" {
					latest.Core.Version = latest.Core.PendingVersion
					latest.Core.PendingVersion = ""
				}
				cfg = latest
				return nil
			}); err != nil {
				unlockConfig()
				return err
			}
			binary, err = coreinfra.BinaryPath(root, cfg.Core.Version, runtime.GOOS)
			if err != nil {
				unlockConfig()
				return err
			}
		}
		e = recoverManagedProfile(root)
		unlockConfig()
		if e != nil {
			return e
		}
	}
	// Never execute an installed file if its recorded directory is missing.
	if _, err = os.Stat(binary); err != nil {
		return err
	}
	validationCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	validation := exec.CommandContext(validationCtx, binary, "-t", "-d", filepath.Dir(path), "-f", path)
	coreinfra.ConfigureProcess(validation)
	err = validation.Run()
	cancel()
	if err != nil {
		return errors.New("managed core configuration validation failed")
	}
	endpoint := config.ResolveMihomoEndpoint()
	data, err := config.ReadMihomoYAML(path)
	if err != nil {
		return err
	}
	if port, ok := data["mixed-port"].(int); ok && port > 0 {
		listener, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if e != nil {
			return errors.New("proxy port is already in use")
		}
		listener.Close()
	}
	listener, e := net.Listen("tcp", endpoint.ExternalController)
	if e != nil {
		return errors.New("controller address is already in use")
	}
	listener.Close()
	client := api.NewClient(endpoint, 500)
	if _, err = client.GetVersion(); err == nil {
		return errors.New("controller address is already in use; choose another port")
	}
	runCtx, stop := context.WithCancel(s.lifetime)
	cmd := exec.CommandContext(runCtx, binary, "-d", filepath.Dir(path), "-f", path)
	coreinfra.ConfigureProcess(cmd)
	cmd.Stdout = &s.logs
	cmd.Stderr = &s.logs
	if err = cmd.Start(); err != nil {
		stop()
		return errors.New("cannot start managed core")
	}
	releaseJob, err := coreinfra.AttachProcess(cmd)
	if err != nil {
		stop()
		cmd.Wait()
		return errors.New("cannot attach managed core lifecycle")
	}
	s.releaseJob = releaseJob
	keepLock = true
	s.releaseLock = unlock
	s.process = cmd
	s.stop = stop
	s.done = make(chan error, 1)
	done := s.done
	go func() { done <- cmd.Wait(); close(done) }()
	if err = s.startControl(root); err != nil {
		s.stopLocked()
		return err
	}
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.stopLocked()
			return ctx.Err()
		case <-timer.C:
			s.stopLocked()
			return errors.New("managed core did not become ready before timeout")
		case <-done:
			s.process = nil
			if s.releaseLock != nil {
				s.releaseLock()
				s.releaseLock = nil
			}
			if s.releaseJob != nil {
				s.releaseJob()
				s.releaseJob = nil
			}
			stop()
			s.stop = nil
			return errors.New("managed core exited during startup")
		case <-ticker.C:
			if info, versionErr := client.GetVersion(); versionErr == nil {
				if info.Version != cfg.Core.Version {
					s.stopLocked()
					return errors.New("running core version does not match the selected release")
				}
				proxy, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", endpoint.MixedPort), 250*time.Millisecond)
				if e == nil {
					proxy.Close()
					return nil
				}
			}
		}
	}
}
func (s *CoreService) stopLocked() {
	if s.stop == nil {
		return
	}
	s.stop()
	if s.done != nil {
		<-s.done
	}
	s.process = nil
	s.stop = nil
	s.done = nil
	if s.releaseJob != nil {
		s.releaseJob()
		s.releaseJob = nil
	}
	if s.releaseLock != nil {
		s.releaseLock()
		s.releaseLock = nil
	}
}
func (s *CoreService) Stop() error {
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	s.stopLocked()
	return nil
}
func (s *CoreService) Restart(ctx context.Context) error {
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	s.stopLocked()
	return s.start(ctx)
}

// Setup completes the subscription-free baseline. The caller can import a profile later.
func (s *CoreService) Setup(ctx context.Context, notify func(coreinfra.Progress)) error {
	status, err := s.Status()
	if err != nil {
		return err
	}
	if status.Version == "" {
		if err = s.Install(ctx, coreinfra.PinnedRelease(), notify); err != nil {
			return err
		}
	}
	if err = s.Configure(); err != nil {
		return err
	}
	return s.Start(ctx)
}
