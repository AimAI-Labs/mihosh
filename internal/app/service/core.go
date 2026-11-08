package service

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
)

// CoreService serializes mutations and owns only the child it started.
// One instance is shared by all Core page commands for the TUI's lifetime.
type CoreService struct {
	control        *http.Server
	controlOwner   coreOwner
	controlCleanup func()
	ownerEvents    chan string
	lifetime       context.Context
	shutdown       context.CancelFunc
	releaseLock    func()
	logs           coreinfra.LogBuffer
	releaseJob     func()
	mu             sync.Mutex
	process        *exec.Cmd
	done           chan error
	stop           context.CancelFunc
}

type CoreStatus = coreinfra.Status

func NewCoreService() *CoreService {
	ctx, cancel := context.WithCancel(context.Background())
	return &CoreService{lifetime: ctx, shutdown: cancel, ownerEvents: make(chan string, 1)}
}
func loadCoreConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if errors.Is(err, config.ErrConfigNotFound) {
		c := config.DefaultConfig
		return &c, nil
	}
	return cfg, err
}
func (s *CoreService) Status() (CoreStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := loadCoreConfig()
	if err != nil {
		return CoreStatus{}, err
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return CoreStatus{}, err
	}
	state := CoreStatus{Mode: cfg.Core.Mode, Version: cfg.Core.Version, Installed: []string{}}
	if state.Mode == "" {
		state.Mode = "external"
	}
	entries, _ := os.ReadDir(filepath.Join(root, "core", "versions"))
	for _, entry := range entries {
		if entry.IsDir() {
			path, err := coreinfra.BinaryPath(root, entry.Name(), runtime.GOOS)
			if err == nil {
				if _, err = os.Stat(path); err == nil {
					state.Installed = append(state.Installed, entry.Name())
				}
			}
		}
	}
	if s.process != nil {
		select {
		case <-s.done:
			s.process = nil
			s.done = nil
			if s.releaseLock != nil {
				s.releaseLock()
				s.releaseLock = nil
			}
			if s.releaseJob != nil {
				s.releaseJob()
				s.releaseJob = nil
			}
			if s.stop != nil {
				s.stop()
				s.stop = nil
			}
		default:
			state.Running = true
		}
	}
	if state.Mode == "managed" {
		client := api.NewClient(config.ResolveMihomoEndpoint(), 500)
		if !state.Running {
			lock := filepath.Join(root, "runtime", "process.lock")
			if _, err := os.Stat(lock); err == nil {
				unlock, lockErr := coreinfra.LockRuntime(lock)
				if lockErr == nil {
					unlock()
				} else {
					_, e := client.GetVersion()
					state.Running = e == nil
				}
			}
		}
		if state.Running {
			if proxies, e := client.GetProxies(); e == nil {
				for name, p := range proxies {
					if len(p.All) == 0 && name != "DIRECT" && name != "REJECT" && name != "REJECT-DROP" && name != "PASS" && name != "COMPATIBLE" {
						state.HasNodes = true
						break
					}
				}
			}
		}
	}
	return state, nil
}

// The caller holds s.mu. An inactive session must prove there is no other runtime
// owner before changing the active target or version.
func (s *CoreService) lockCoreMutation(root string) (func(), error) {
	dir := filepath.Join(root, "runtime")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	releaseConfig, err := coreinfra.LockRuntime(filepath.Join(dir, "config.lock"))
	if err != nil {
		return nil, err
	}
	if s.process != nil {
		select {
		case <-s.done:
			s.stopLocked()
		default:
			return releaseConfig, nil
		}
	}
	releaseProcess, err := coreinfra.LockRuntime(filepath.Join(dir, "process.lock"))
	if err != nil {
		releaseConfig()
		return nil, err
	}
	return func() { releaseProcess(); releaseConfig() }, nil
}
func (s *CoreService) operationContext(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	unregister := context.AfterFunc(s.lifetime, cancel)
	return ctx, func() { unregister(); cancel() }
}
func (s *CoreService) Close() {
	s.shutdown()
	s.mu.Lock()
	s.stopLocked()
	cleanup := s.controlCleanup
	s.mu.Unlock()
	if cleanup != nil {
		cleanup()
	}
}
