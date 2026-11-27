package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"gopkg.in/yaml.v3"
)

func (s *CoreService) Configure() error {
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
	if cfg.Core.Version == "" {
		return errors.New("install a core version first")
	}
	path, err := config.ManagedMihomoConfigPath()
	if err != nil {
		return err
	}
	if _, err = createManagedBaseConfig(path, nil); err != nil {
		return err
	}
	return config.Update(func(latest *config.Config) error { latest.Core.Mode = "managed"; return nil })
}

func managedBaseConfig() ([]byte, error) {
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return nil, err
	}
	return fmt.Appendf(nil, "mixed-port: 7890\nexternal-controller: 127.0.0.1:9090\nsecret: %q\nallow-lan: false\nmode: rule\nlog-level: info\ntun:\n  enable: false\nrules:\n  - MATCH,DIRECT\n", hex.EncodeToString(secret[:])), nil
}

// The caller holds the config lock. Leave an existing file or symlink intact.
func createManagedBaseConfig(path string, beforeCreate func() error) (bool, error) {
	if _, err := os.Lstat(path); err == nil {
		if _, err = os.Stat(path); err != nil {
			return false, fmt.Errorf("managed configuration path has no usable target: %w", err)
		}
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	data, err := managedBaseConfig()
	if err != nil {
		return false, err
	}
	if beforeCreate != nil {
		if err = beforeCreate(); err != nil {
			return false, err
		}
	}
	if err = privateWrite(path, data); err != nil {
		return false, err
	}
	return true, nil
}

func prepareMissingManagedConfig(path string) (bool, error) {
	return createManagedBaseConfig(path, func() error {
		return config.Update(func(latest *config.Config) error {
			latest.ActiveSub = ""
			return nil
		})
	})
}

func (s *CoreService) SaveNetwork(proxy string, accelerators []coreinfra.Accelerator, timeoutSeconds ...int) error {
	// Fetchers snapshot these preferences; saving affects the next download.
	// Validate settings before persisting. No network request is performed.
	f := coreinfra.NewFetcher()
	f.Proxy = strings.TrimSpace(proxy)
	f.Accelerators = accelerators
	if len(timeoutSeconds) > 0 {
		if timeoutSeconds[0] != 0 && (timeoutSeconds[0] < 30 || timeoutSeconds[0] > 3600) {
			return errors.New("download timeout must be between 30 and 3600 seconds")
		}
		f.DownloadTimeout = time.Duration(timeoutSeconds[0]) * time.Second
	}
	if err := f.ValidateSettings(); err != nil {
		return err
	}
	return config.Update(func(latest *config.Config) error {
		latest.Core.Proxy = f.Proxy
		latest.Core.Accelerators = f.Accelerators
		if len(timeoutSeconds) > 0 {
			latest.Core.DownloadTimeoutSeconds = timeoutSeconds[0]
		}
		return nil
	})
}
func (s *CoreService) UseExternal() error {
	return s.switchToExternal("", "", 0)
}

func (s *CoreService) ConnectExternal(address, secret string, mixedPort int) error {
	address = strings.TrimSpace(address)
	normalized := address
	if !strings.Contains(normalized, "://") {
		normalized = "http://" + normalized
	}
	u, err := url.Parse(normalized)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(u.Host, ":") {
		return errors.New("invalid HTTP(S) controller address")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.ParseUint(port, 10, 16)
		if err != nil || n == 0 {
			return errors.New("invalid HTTP(S) controller address")
		}
	}
	if mixedPort < 1 || mixedPort > 65535 {
		return errors.New("invalid mixed proxy port")
	}
	return s.switchToExternal(strings.TrimRight(normalized, "/"), secret, mixedPort)
}

func (s *CoreService) switchToExternal(address, secret string, mixedPort int) (resultErr error) {
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	root, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	if runtime.GOOS == "linux" && s.usesService() {
		manager := s.serviceManager(root)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		state, err := manager.Status(ctx)
		if err != nil {
			return err
		}
		if state.Running {
			// The background owner must release the process lock before the mutation.
			if err := manager.Action(ctx, "stop"); err != nil {
				return err
			}
			defer func() {
				if resultErr != nil && s.usesService() {
					ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
					defer cancel()
					resultErr = errors.Join(resultErr, manager.Action(ctx, "start"))
				}
			}()
		}
	}
	releaseMutation, err := s.lockCoreMutation(root)
	if err != nil {
		return err
	}
	defer releaseMutation()
	if err := s.stopLocked(); err != nil {
		return err
	}
	return config.Update(func(latest *config.Config) error {
		latest.Core.Mode = "external"
		latest.Core.ExternalController = address
		latest.Core.ExternalSecret = secret
		latest.Core.ExternalMixedPort = mixedPort
		return nil
	})
}

func (s *CoreService) SetPorts(controllerPort, mixedPort int) error {
	if controllerPort < 1 || controllerPort > 65535 || mixedPort < 1 || mixedPort > 65535 || controllerPort == mixedPort {
		return errors.New("choose different controller/proxy ports between 1 and 65535")
	}
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	if s.process != nil {
		select {
		case <-s.done:
			if err := s.stopLocked(); err != nil {
				return err
			}
		default:
			return errors.New("stop the managed core before changing its ports")
		}
	}
	path, err := config.ManagedMihomoConfigPath()
	if err != nil {
		return err
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	unlock, err := s.lockCoreMutation(root)
	if err != nil {
		return err
	}
	defer unlock()
	if err = recoverManagedProfile(root); err != nil {
		return err
	}
	data, err := config.ReadMihomoYAML(path)
	if err != nil {
		return err
	}
	data["external-controller"] = fmt.Sprintf("127.0.0.1:%d", controllerPort)
	data["mixed-port"] = mixedPort
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return privateWrite(path, bytes)
}
