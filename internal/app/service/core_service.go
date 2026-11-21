package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/host"
)

func (s *CoreService) serviceManager(root string) host.Services {
	if s.services != nil {
		return s.services
	}
	return host.NewServices(root)
}
func (s *CoreService) usesService() bool {
	if s.daemon {
		return false
	}
	cfg, err := loadCoreConfig()
	return err == nil && cfg.Core.Mode == "managed" && cfg.Core.Runtime == "service"
}
func (s *CoreService) ServiceAction(ctx context.Context, action string) (resultErr error) {
	switch action {
	case "install", "uninstall", "start", "stop", "restart", "enable", "disable", "boot":
	default:
		return errors.New("unsupported service action")
	}
	if runtime.GOOS != "linux" {
		return errors.New("managed background services require Linux/systemd")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	root, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	cfg, err := loadCoreConfig()
	if err != nil {
		return err
	}
	if cfg.Core.Mode != "managed" {
		return errors.New("configure a managed core before managing its service")
	}
	manager := s.serviceManager(root)
	if action == "install" {
		if err = s.installService(ctx, root, manager); err != nil {
			return err
		}
		if s.process == nil {
			return nil
		}
		action = "start"
	} else {
		state, statusErr := manager.Status(ctx)
		if statusErr != nil {
			return statusErr
		}
		if !state.Installed {
			return errors.New("background service is not installed; install it first")
		}
	}
	if action == "enable" || action == "boot" {
		if err = manager.Action(ctx, action); err != nil {
			return err
		}
		action = "start"
	}
	hadSession := s.process != nil
	if hadSession && (action == "start" || action == "restart") {
		// Release ports and the runtime lock before systemd starts the new owner.
		if err = s.stopLocked(); err != nil {
			return err
		}
		removeOwner(root, s.controlOwner.Token)
	}
	if action == "start" || action == "restart" {
		restoreRuntime := cfg.Core.Runtime
		defer func() {
			if resultErr == nil {
				return
			}
			// A failed handoff or start must not strand an active session or corrupt runtime mode.
			recovery, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if stopErr := manager.Action(recovery, "stop"); stopErr != nil {
				state, statusErr := manager.Status(recovery)
				if statusErr == nil && state.Running {
					resultErr = errors.Join(resultErr, stopErr)
					return
				}
			}
			restoreErr := config.Update(func(latest *config.Config) error { latest.Core.Runtime = restoreRuntime; return nil })
			resultErr = errors.Join(resultErr, restoreErr)
			if hadSession && restoreErr == nil {
				resultErr = errors.Join(resultErr, s.start(recovery))
			}
		}()
		if err = config.Update(func(latest *config.Config) error { latest.Core.Runtime = "service"; return nil }); err != nil {
			return err
		}
	}
	if err = manager.Action(ctx, action); err != nil {
		return err
	}
	if action == "uninstall" {
		return config.Update(func(latest *config.Config) error { latest.Core.Runtime = "session"; return nil })
	}
	if action == "start" || action == "restart" {
		s.processLogs.Store(false)
		for {
			if err = s.Control(ctx, "status", ""); err == nil {
				endpoint := config.ResolveMihomoEndpoint()
				if _, err = api.NewClient(endpoint, 500).GetVersion(); err == nil {
					connection, dialErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", endpoint.MixedPort), 250*time.Millisecond)
					if dialErr == nil {
						_ = connection.Close()
						return nil
					}
				}
			}
			select {
			case <-ctx.Done():
				return errors.New("background core did not become ready; inspect the service manager logs")
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	return nil
}

func (s *CoreService) installService(ctx context.Context, root string, manager host.Services) error {
	unlock, err := s.lockCoreMutation(root)
	if err != nil {
		return err
	}
	defer unlock()
	source, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "bin")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	name := "mihosh"

	target := filepath.Join(dir, name)
	if filepath.Clean(source) != filepath.Clean(target) {
		input, e := os.Open(source)
		if e != nil {
			return e
		}
		defer func() { _ = input.Close() }()
		output, e := os.CreateTemp(dir, ".service-install-*")
		if e != nil {
			return e
		}
		defer func() { _ = os.Remove(output.Name()) }()
		_, copyErr := io.Copy(output, input)
		syncErr := output.Sync()
		closeErr := output.Close()
		if err = errors.Join(copyErr, syncErr, closeErr); err != nil {
			return err
		}
		if err = os.Chmod(output.Name(), 0700); err != nil {
			return err
		}
		if err = os.Rename(output.Name(), target); err != nil {
			return err
		}
	}
	if err = manager.Install(ctx, target); err != nil {
		return err
	}
	return config.Update(func(latest *config.Config) error { latest.Core.Runtime = "service"; return nil })
}

// Serve is the only service-manager entry. The selected kernel remains a child of
// this coordinator, so upgrades and recovery use exactly the session implementation.
func (s *CoreService) Serve(ctx context.Context, ready func()) error {
	s.daemon = true
	if err := s.Start(ctx); err != nil {
		return err
	}
	ready()
	return s.Wait(ctx)
}
func (s *CoreService) forward(ctx context.Context, action, value string) (bool, error) {
	return s.forwardControlRequest(ctx, coreControlRequest{Action: action, Version: value}, nil, nil)
}

func (s *CoreService) forwardControlRequest(ctx context.Context, request coreControlRequest, activation *ActivateResult, overrideRes *MihomoOverridesSaveResult) (bool, error) {
	s.mu.Lock()
	owned := s.process != nil || s.daemon
	token := s.controlOwner.Token
	s.mu.Unlock()
	if owned {
		return false, nil
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return false, err
	}
	data, _ := os.ReadFile(filepath.Join(root, "runtime", "owner.json"))
	var owner coreOwner
	if json.Unmarshal(data, &owner) == nil && owner.Token == token && owner.Token != "" {
		return false, nil
	}
	err = s.sendControlRequest(ctx, request, activation, overrideRes)
	if errors.Is(err, ErrNoCoreOwner) {
		return false, nil
	}
	if err == nil {
		s.processLogs.Store(false)
	}
	return true, err
}

// Recovery is allowed only after both locks prove that no session/service owns the
// runtime. A leftover PID never establishes ownership.
func (s *CoreService) RecoverNetwork(ctx context.Context) error {
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	if s.process != nil {
		return errors.New("stop the owned core before recovering its network")
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(root, "runtime"), 0700); err != nil {
		return err
	}
	unlock, err := coreinfra.LockRuntime(filepath.Join(root, "runtime", "config.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	release, err := coreinfra.LockRuntime(filepath.Join(root, "runtime", "process.lock"))
	if err != nil {
		return err
	}
	defer release()
	return s.recoverNetworkLocked(ctx, root)
}
