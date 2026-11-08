package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
)

// Update applies a field-scoped mutation to freshly loaded configuration. Callbacks
// must not call other config functions; the configuration mutex is held throughout.
func Update(change func(*Config) error) error {
	mu.Lock()
	defer mu.Unlock()
	unlock, err := lockAppConfig()
	if err != nil {
		return err
	}
	defer unlock()
	cfg, err := loadLocked()
	if errors.Is(err, ErrConfigNotFound) {
		copy := DefaultConfig
		cfg = &copy
	} else if err != nil {
		return err
	}
	if err = change(cfg); err != nil {
		return err
	}
	return saveLocked(cfg)
}

func lockAppConfig() (func(), error) {
	// Managed core support is Linux/Windows. Keep the existing client-only behavior
	// on other platforms rather than making their application preferences unusable.
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return func() {}, nil
	}
	root, err := GetConfigDir()
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(time.Second)
	for {
		release, err := coreinfra.LockRuntime(filepath.Join(root, ".config.lock"))
		if err == nil {
			return release, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("application configuration is busy: %w", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
