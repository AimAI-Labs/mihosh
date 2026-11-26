package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var ErrConfigBusy = errors.New("application configuration is busy")

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
	cfg, err := loadOrDefaultLocked()
	if err != nil {
		return err
	}
	if err = change(cfg); err != nil {
		return err
	}
	return saveLocked(cfg)
}

func lockAppConfig() (func(), error) {
	root, err := GetConfigDir()
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		return func() {}, recoverProfileRestore(root)
	}
	deadline := time.Now().Add(time.Second)
	for {
		release, err := lockAppConfigFile(filepath.Join(root, ".config.lock"))
		if err == nil {
			if err := recoverProfileRestore(root); err != nil {
				release()
				return nil, err
			}
			return release, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("%w: %v", ErrConfigBusy, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
