package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

const (
	releasePageCacheTTL   = 24 * time.Hour
	releasePageCacheLimit = 4 << 20
)

type cachedReleasePage struct {
	FetchedAt time.Time             `json:"fetched_at"`
	GOOS      string                `json:"goos"`
	GOARCH    string                `json:"goarch"`
	Page      coreinfra.ReleasePage `json:"page"`
}

// Releases queries one page without installing, activating or changing the runtime.
// Cached pages are served from disk within releasePageCacheTTL to avoid redundant requests.
func (s *CoreService) Releases(ctx context.Context, page int, notify func(coreinfra.Progress)) (coreinfra.ReleasePage, error) {
	return s.releases(ctx, page, notify, false)
}

// RefreshReleases bypasses the cached release catalog and fetches a fresh page from upstream.
func (s *CoreService) RefreshReleases(ctx context.Context, page int, notify func(coreinfra.Progress)) (coreinfra.ReleasePage, error) {
	return s.releases(ctx, page, notify, true)
}

func (s *CoreService) releases(ctx context.Context, page int, notify func(coreinfra.Progress), force bool) (coreinfra.ReleasePage, error) {
	if page < 1 {
		return coreinfra.ReleasePage{}, errors.New("invalid release page")
	}
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if !force {
		if cached, ok := loadCachedReleasePage(page, goos, goarch); ok {
			return cached, nil
		}
	}
	cfg, err := loadCoreConfig()
	if err != nil {
		return coreinfra.ReleasePage{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	unregister := context.AfterFunc(s.lifetime, cancel)
	defer unregister()
	result, err := s.fetchReleasePage(ctx, cfg, page, goos, goarch, notify)
	if err != nil {
		return coreinfra.ReleasePage{}, err
	}
	if force && page == 1 {
		clearCachedReleasePages(goos, goarch)
	}
	saveCachedReleasePage(result, goos, goarch)
	return result, nil
}

func (s *CoreService) fetchReleasePage(ctx context.Context, cfg *config.Config, page int, goos, goarch string, notify func(coreinfra.Progress)) (coreinfra.ReleasePage, error) {
	if s.releaseFetcher != nil {
		return s.releaseFetcher(ctx, cfg, page, goos, goarch, notify)
	}
	fetcher := coreFetcher(cfg, notify)
	fetcher.RunningProxy = managedDownloadProxy(cfg)
	return fetcher.Releases(ctx, page, goos, goarch)
}

func releasePageCacheDir() (string, error) {
	root, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "core", "releases-cache"), nil
}

func releasePageCachePath(dir, goos, goarch string, page int) string {
	return filepath.Join(dir, fmt.Sprintf("%s-%s-page-%d.json", goos, goarch, page))
}

func loadCachedReleasePage(page int, goos, goarch string) (coreinfra.ReleasePage, bool) {
	if page < 1 {
		return coreinfra.ReleasePage{}, false
	}
	dir, err := releasePageCacheDir()
	if err != nil {
		return coreinfra.ReleasePage{}, false
	}
	data, err := os.ReadFile(releasePageCachePath(dir, goos, goarch, page))
	if err != nil || len(data) > releasePageCacheLimit {
		return coreinfra.ReleasePage{}, false
	}
	var entry cachedReleasePage
	if json.Unmarshal(data, &entry) != nil || entry.GOOS != goos || entry.GOARCH != goarch || entry.Page.Page != page {
		return coreinfra.ReleasePage{}, false
	}
	if entry.FetchedAt.IsZero() {
		return coreinfra.ReleasePage{}, false
	}
	age := time.Since(entry.FetchedAt)
	if age < -time.Minute || age >= releasePageCacheTTL {
		return coreinfra.ReleasePage{}, false
	}
	for _, r := range entry.Page.Releases {
		if !r.IsStable() || len(r.Assets) == 0 {
			return coreinfra.ReleasePage{}, false
		}
		if _, err := r.SelectAsset(goos, goarch); err != nil {
			return coreinfra.ReleasePage{}, false
		}
	}
	if entry.Page.Releases == nil {
		entry.Page.Releases = []coreinfra.Release{}
	}
	return entry.Page, true
}

func saveCachedReleasePage(page coreinfra.ReleasePage, goos, goarch string) {
	if page.Page < 1 {
		return
	}
	dir, err := releasePageCacheDir()
	if err != nil || os.MkdirAll(dir, 0700) != nil {
		return
	}
	entry := cachedReleasePage{
		FetchedAt: time.Now().UTC(),
		GOOS:      goos,
		GOARCH:    goarch,
		Page:      page,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = privateWrite(releasePageCachePath(dir, goos, goarch, page.Page), data)
}

func clearCachedReleasePages(goos, goarch string) {
	dir, err := releasePageCacheDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := fmt.Sprintf("%s-%s-page-", goos, goarch)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
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
			latest.Core.Version = r.Version()
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
	return s.UpgradeVersion(ctx, release.Version())
}
