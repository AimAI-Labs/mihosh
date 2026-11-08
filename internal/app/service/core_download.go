package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
)

func coreFetcher(cfg *config.Config, notify func(coreinfra.Progress)) *coreinfra.Fetcher {
	f := coreinfra.NewFetcher()
	f.Proxy = cfg.Core.Proxy
	if cfg.Core.DownloadTimeoutSeconds != 0 {
		if cfg.Core.DownloadTimeoutSeconds < 30 || cfg.Core.DownloadTimeoutSeconds > 3600 {
			f.DownloadTimeout = -time.Second
		} else {
			f.DownloadTimeout = time.Duration(cfg.Core.DownloadTimeoutSeconds) * time.Second
		}
	}
	if cfg.Core.Accelerators != nil {
		f.Accelerators = cfg.Core.Accelerators
	}
	f.Notify = notify
	return f
}
func (s *CoreService) Release(ctx context.Context, tag string, notify func(coreinfra.Progress)) (coreinfra.Release, error) {
	cfg, err := loadCoreConfig()
	if err != nil {
		return coreinfra.Release{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	unregister := context.AfterFunc(s.lifetime, cancel)
	defer unregister()
	fetcher := coreFetcher(cfg, notify)
	fetcher.RunningProxy = managedDownloadProxy(cfg)
	release, err := fetcher.Release(ctx, tag)
	if err != nil {
		return coreinfra.Release{}, err
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return release, err
	}
	if err = os.MkdirAll(filepath.Join(root, "core"), 0700); err != nil {
		return release, err
	}
	data, err := json.Marshal(release)
	if err != nil {
		return release, err
	}
	if err = privateWrite(filepath.Join(root, "core", "release-cache.json"), data); err != nil {
		return release, err
	}
	return release, nil
}
func (s *CoreService) Install(ctx context.Context, r coreinfra.Release, notify func(coreinfra.Progress)) error {
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	cfg, err := loadCoreConfig()
	if err != nil {
		return err
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	unregister := context.AfterFunc(s.lifetime, cancel)
	defer unregister()
	fetcher := coreFetcher(cfg, notify)
	fetcher.RunningProxy = managedDownloadProxy(cfg)
	if _, err = fetcher.Install(ctx, root, r, runtime.GOOS, runtime.GOARCH); err != nil {
		return err
	}
	// Installation does not take ownership of an existing external instance.
	return config.Update(func(latest *config.Config) error {
		if latest.Core.Version == "" {
			latest.Core.Version = r.Tag
		}
		return nil
	})
}
func (s *CoreService) Import(ctx context.Context, source, tag, digest string) error {
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	root, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	if _, err = coreinfra.Import(ctx, root, source, tag, digest, runtime.GOOS); err != nil {
		return err
	}
	return config.Update(func(latest *config.Config) error {
		if latest.Core.Version == "" {
			latest.Core.Version = tag
		}
		return nil
	})
}

// CachedRelease is explicitly marked as cached; it is never reported as latest.
func (s *CoreService) CachedRelease() coreinfra.Release {
	fallback := coreinfra.PinnedRelease()
	root, err := config.GetConfigDir()
	if err != nil {
		return fallback
	}
	data, err := os.ReadFile(filepath.Join(root, "core", "release-cache.json"))
	if err != nil || len(data) > 4<<20 {
		return fallback
	}
	var release coreinfra.Release
	if json.Unmarshal(data, &release) != nil || release.Tag == "" || len(release.Assets) == 0 {
		return fallback
	}
	if _, err = release.SelectAsset(runtime.GOOS, runtime.GOARCH); err != nil {
		return fallback
	}
	release.Route = "cache:" + release.Route
	return release
}
func managedDownloadProxy(cfg *config.Config) string {
	if cfg.Core.Mode != "managed" || cfg.ActiveSub == "" {
		return ""
	}
	endpoint := config.ResolveMihomoEndpoint()
	client := api.NewClient(endpoint, 500)
	proxies, err := client.GetProxies()
	if err != nil {
		return ""
	}
	for name, p := range proxies {
		if len(p.All) == 0 && name != "DIRECT" && name != "REJECT" && name != "REJECT-DROP" && name != "PASS" && name != "COMPATIBLE" {
			return config.MixedPortToProxyURL(endpoint.MixedPort)
		}
	}
	return ""
}

func (s *CoreService) Upgrade(ctx context.Context, notify func(coreinfra.Progress)) error {
	release, err := s.Release(ctx, "latest", notify)
	if err != nil {
		return err
	}
	if err = s.Install(ctx, release, notify); err != nil {
		return err
	}
	return s.UpgradeVersion(ctx, release.Tag)
}
