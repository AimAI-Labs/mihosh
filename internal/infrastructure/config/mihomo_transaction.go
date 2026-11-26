package config

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
)

var mihomoWriteMu sync.Mutex

// Mutators serialize their full read-modify-write operation. Managed files also
// share the runtime transaction lock with profile activation and version changes.
func lockMihomoFile(path string) (func(), error) {
	mihomoWriteMu.Lock()
	managed, err := ManagedMihomoConfigPath()
	if err == nil && filepath.Clean(path) == filepath.Clean(managed) {
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			mihomoWriteMu.Unlock()
			return nil, err
		}
		lockPath := filepath.Join(filepath.Dir(path), "config.lock")
		deadline := time.Now().Add(time.Second)
		for {
			release, err := coreinfra.LockRuntime(lockPath)
			if err == nil {
				return func() { release(); mihomoWriteMu.Unlock() }, nil
			}
			if !errors.Is(err, coreinfra.ErrRuntimeLocked) || time.Now().After(deadline) {
				mihomoWriteMu.Unlock()
				return nil, err
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	return mihomoWriteMu.Unlock, nil
}
