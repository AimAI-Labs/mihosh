package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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
	if _, err = os.Stat(path); os.IsNotExist(err) {
		var secret [32]byte
		if _, err = rand.Read(secret[:]); err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		data := fmt.Sprintf("mixed-port: 7890\nexternal-controller: 127.0.0.1:9090\nsecret: %q\nallow-lan: false\nmode: rule\nlog-level: info\ntun:\n  enable: false\nrules:\n  - MATCH,DIRECT\n", hex.EncodeToString(secret[:]))
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := file.WriteString(data)
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	} else if err != nil {
		return err
	}
	return config.Update(func(latest *config.Config) error { latest.Core.Mode = "managed"; return nil })
}
func (s *CoreService) SaveNetwork(proxy string, accelerators []coreinfra.Accelerator, timeoutSeconds ...int) error {
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
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

	s.stopLocked()
	return config.Update(func(latest *config.Config) error {
		latest.Core.Mode = "external"
		latest.Core.ExternalController = ""
		latest.Core.ExternalSecret = ""
		latest.Core.ExternalMixedPort = 0
		return nil
	})
}
func (s *CoreService) ConnectExternal(address, secret string, mixedPort int) error {
	address = strings.TrimSpace(address)
	normalized := address
	if !strings.Contains(normalized, "://") {
		normalized = "http://" + normalized
	}
	u, err := url.Parse(normalized)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("invalid HTTP(S) controller address")
	}
	if mixedPort < 1 || mixedPort > 65535 {
		return errors.New("invalid mixed proxy port")
	}
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

	if err = config.Update(func(latest *config.Config) error {
		latest.Core.Mode = "external"
		latest.Core.ExternalController = strings.TrimRight(normalized, "/")
		latest.Core.ExternalSecret = secret
		latest.Core.ExternalMixedPort = mixedPort
		return nil
	}); err != nil {
		return err
	}
	s.stopLocked()
	return nil
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
			s.stopLocked()
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
