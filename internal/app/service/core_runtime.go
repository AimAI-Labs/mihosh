package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/host"
)

func (s *CoreService) Start(ctx context.Context) error {
	if s.usesService() {
		s.processLogs.Store(false)
		return s.ServiceAction(ctx, "start")
	}
	if cfg, err := loadCoreConfig(); err == nil && cfg.Core.Mode == "managed" {
		if forwarded, err := s.forward(ctx, "status", ""); forwarded && err == nil {
			if _, err = api.NewClient(config.ResolveMihomoEndpoint(), 500).GetVersion(); err == nil {
				if !s.logging.Load() {
					s.processLogs.Store(false)
				}
				return nil
			}
		}
	}
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	return s.start(ctx)
}
func (s *CoreService) start(ctx context.Context, skipRecovery ...bool) error {
	ctx, cancelOperation := s.operationContext(ctx)
	defer cancelOperation()
	s.failure = nil
	if s.process != nil {
		select {
		case <-s.done:
			if err := s.stopLocked(); err != nil {
				return err
			}
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
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
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
	if err = s.recoverNetworkLocked(ctx, root); err != nil {
		return err
	}
	if len(skipRecovery) == 0 || !skipRecovery[0] {
		if err = recoverNetworkChange(root); err != nil {
			return err
		}
		cfg, err = loadCoreConfig()
		if err != nil {
			return err
		}
	}
	// Never execute an installed file if its recorded directory is missing.
	if _, err = os.Stat(binary); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) && (len(skipRecovery) == 0 || !skipRecovery[0]) {
		if err = s.createFallbackConfig(root, path); err != nil {
			return fmt.Errorf("cannot prepare basic managed configuration: %w", err)
		}
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return err
	}
	if err = s.validateConfig(ctx, cfg, binary, path, path, raw); err != nil {
		err = fmt.Errorf("managed core configuration validation failed: %w", err)
		s.processLogs.Store(true)
		_, _ = fmt.Fprintf(&s.logs, "level=error msg=%q\n", err.Error())
		return err
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
		_ = listener.Close()
	}
	listener, e := net.Listen("tcp", endpoint.ExternalController)
	if e != nil {
		return errors.New("controller address is already in use")
	}
	_ = listener.Close()
	client := api.NewClient(endpoint, 500)
	if _, err = client.GetVersion(); err == nil {
		return errors.New("controller address is already in use; choose another port")
	}
	if err = s.prepareNetwork(ctx, root, cfg, binary); err != nil {
		return err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = s.recoverNetworkLocked(context.Background(), root)
		}
	}()
	runCtx, stop := context.WithCancel(s.lifetime)
	cmd := exec.CommandContext(runCtx, binary, "-d", filepath.Dir(path), "-f", path)
	coreinfra.ConfigureProcess(cmd)
	host.GracefulCancel(cmd)
	if cfg.Core.TUN {
		host.ConfigureTUNProcess(cmd)
	}
	cmd.WaitDelay = 5 * time.Second
	cmd.Stdout = &s.logs
	cmd.Stderr = &s.logs
	if s.daemon {
		cmd.Stdout = io.MultiWriter(&s.logs, os.Stderr)
		cmd.Stderr = cmd.Stdout
	}
	if err = cmd.Start(); err != nil {
		stop()
		return errors.New("cannot start managed core")
	}
	s.logging.Store(true)
	s.processLogs.Store(true)
	releaseJob, err := coreinfra.AttachProcess(cmd)
	if err != nil {
		stop()
		_ = cmd.Wait()
		s.logging.Store(false)
		return errors.New("cannot attach managed core lifecycle")
	}
	s.releaseJob = releaseJob
	keepLock = true
	s.releaseLock = unlock
	s.process = cmd
	s.endpoint = endpoint
	s.stop = stop
	s.done = make(chan error, 1)
	done := s.done
	go func() {
		err := cmd.Wait()
		s.logging.Store(false)
		done <- err
		close(done)
		s.mu.Lock()
		current := s.done == done
		s.mu.Unlock()
		if current {
			s.notifyOwner("exit")
		}
	}()
	if err = s.startControl(root); err != nil {
		return errors.Join(err, s.stopLocked())
	}
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), s.stopLocked())
		case <-timer.C:
			return errors.Join(errors.New("managed core did not become ready before timeout"), s.stopLocked())
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
					return errors.Join(errors.New("running core version does not match the selected release"), s.stopLocked())
				}
				proxy, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", endpoint.MixedPort), 250*time.Millisecond)
				if e == nil {
					_ = proxy.Close()
					if cfg.Core.TUN {
						if _, err = s.network.CaptureTUN(ctx, host.TUNIdentity(root)); err != nil {
							continue
						}
					}
					if err = s.activateNetwork(ctx, root, cfg); err != nil {
						return errors.Join(err, s.stopLocked())
					}
					succeeded = true
					return nil
				}
			}
		}
	}
}

func (s *CoreService) createFallbackConfig(root, path string) error {
	unlock, err := coreinfra.LockRuntime(filepath.Join(root, "runtime", "config.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	created, err := prepareMissingManagedConfig(path)
	if err != nil || !created {
		return err
	}
	const notice = `level=warn msg="Managed configuration was missing; using a basic direct configuration"` + "\n"
	s.processLogs.Store(true)
	_, _ = io.WriteString(&s.logs, notice)
	if s.daemon {
		_, _ = io.WriteString(os.Stderr, notice)
	}
	return nil
}

func (s *CoreService) stopLocked() error {
	if s.stop == nil {
		return nil
	}
	s.disableRunningTUN()
	s.stop()
	if s.done != nil {
		<-s.done
	}
	s.process = nil
	s.stop = nil
	s.done = nil
	var recoveryErr error
	if root, err := config.GetConfigDir(); err == nil {
		recoveryErr = s.recoverNetworkLocked(context.Background(), root)
	} else {
		recoveryErr = err
	}
	if s.releaseJob != nil {
		s.releaseJob()
		s.releaseJob = nil
	}
	if s.releaseLock != nil {
		s.releaseLock()
		s.releaseLock = nil
	}
	return recoveryErr
}
func (s *CoreService) Stop() error {
	return s.StopContext(context.Background())
}
func (s *CoreService) StopContext(ctx context.Context) error {
	if s.usesService() {
		return s.ServiceAction(ctx, "stop")
	}
	if forwarded, err := s.forward(ctx, "stop", ""); forwarded {
		return err
	}
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	s.failure = nil
	return s.stopLocked()
}
func (s *CoreService) Restart(ctx context.Context) error {
	if s.usesService() {
		return s.ServiceAction(ctx, "restart")
	}
	if forwarded, err := s.forward(ctx, "restart", ""); forwarded {
		return err
	}
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	if err := s.stopLocked(); err != nil {
		return err
	}
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
