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
func (s *CoreService) ServiceAction(ctx context.Context, action string) error {
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
	if s.process != nil {
		return errors.New("stop the session core before managing its background service")
	}
	manager := s.serviceManager(root)
	if action == "install" {

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
			defer input.Close()
			output, e := os.CreateTemp(dir, ".service-install-*")
			if e != nil {
				return e
			}
			defer os.Remove(output.Name())
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
	if err = manager.Action(ctx, action); err != nil {
		return err
	}
	if action == "uninstall" {
		return config.Update(func(latest *config.Config) error { latest.Core.Runtime = "session"; return nil })
	}
	if action == "start" || action == "restart" {
		for {
			if err = s.Control(ctx, "status", ""); err == nil {
				endpoint := config.ResolveMihomoEndpoint()
				if _, err = api.NewClient(endpoint, 500).GetVersion(); err == nil {
					connection, dialErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", endpoint.MixedPort), 250*time.Millisecond)
					if dialErr == nil {
						connection.Close()
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
	return s.forwardControl(ctx, action, value, nil)
}

func (s *CoreService) forwardControl(ctx context.Context, action, value string, activation *ActivateResult) (bool, error) {
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
	err = s.sendControl(ctx, action, value, activation)
	if errors.Is(err, ErrNoCoreOwner) {
		return false, nil
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
