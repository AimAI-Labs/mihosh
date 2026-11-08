package config

import (
	"os"
	"path/filepath"
	"sync"

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
		release, err := coreinfra.LockRuntime(filepath.Join(filepath.Dir(path), "config.lock"))
		if err != nil {
			mihomoWriteMu.Unlock()
			return nil, err
		}
		return func() { release(); mihomoWriteMu.Unlock() }, nil
	}
	return mihomoWriteMu.Unlock, nil
}
