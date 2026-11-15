package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/stretchr/testify/require"
)

func testStableReleaseForPlatform(tag, goos, arch string) coreinfra.Release {
	platform := arch
	if arch == "amd64" {
		platform = "amd64-compatible"
	}
	ext := ".gz"
	if goos == "windows" {
		ext = ".zip"
	}
	name := fmt.Sprintf("mihomo-%s-%s-%s%s", goos, platform, tag, ext)
	return coreinfra.Release{
		Tag:         tag,
		PublishedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		FetchedAt:   time.Now().UTC(),
		Trusted:     true,
		Route:       "direct",
		Assets: []coreinfra.Asset{{
			Name: name,
			URL:  "https://github.com/MetaCubeX/mihomo/releases/download/" + tag + "/" + name,
			Size: 1024,
		}},
	}
}

func TestCoreReleasesUsesPersistentDiskCacheAndSupportsExplicitRefresh(t *testing.T) {
	newIsolatedService(t)
	upstreamHits := 0
	currentTag := "v1.20.0"

	newTestSvc := func() *CoreService {
		svc := NewCoreService()
		svc.releaseFetcher = func(_ context.Context, _ *config.Config, page int, goos, goarch string, _ func(coreinfra.Progress)) (coreinfra.ReleasePage, error) {
			upstreamHits++
			switch page {
			case 1:
				return coreinfra.ReleasePage{
					Page:     1,
					HasNext:  true,
					Releases: []coreinfra.Release{testStableReleaseForPlatform(currentTag, goos, goarch)},
				}, nil
			case 2:
				return coreinfra.ReleasePage{
					Page:     2,
					HasNext:  false,
					Releases: []coreinfra.Release{testStableReleaseForPlatform("v1.19.0", goos, goarch)},
				}, nil
			default:
				return coreinfra.ReleasePage{Page: page}, nil
			}
		}
		return svc
	}

	svc1 := newTestSvc()
	defer svc1.Close()

	// 1. First fetch for page 1 hits upstream once and populates disk cache.
	page1, err := svc1.Releases(context.Background(), 1, nil)
	require.NoError(t, err)
	require.Equal(t, 1, upstreamHits)
	require.Len(t, page1.Releases, 1)
	require.Equal(t, "v1.20.0", page1.Releases[0].Tag)

	// 2. Repeated calls on same service and a brand-new service instance hit disk cache (0 upstream requests).
	cachedAgain, err := svc1.Releases(context.Background(), 1, nil)
	require.NoError(t, err)
	require.Equal(t, 1, upstreamHits)
	require.Equal(t, "v1.20.0", cachedAgain.Releases[0].Tag)

	svc2 := newTestSvc()
	defer svc2.Close()
	cachedNewSession, err := svc2.Releases(context.Background(), 1, nil)
	require.NoError(t, err)
	require.Equal(t, 1, upstreamHits)
	require.Equal(t, "v1.20.0", cachedNewSession.Releases[0].Tag)

	// 3. Page 2 caches independently and survives across service instances.
	page2, err := svc2.Releases(context.Background(), 2, nil)
	require.NoError(t, err)
	require.Equal(t, 2, upstreamHits)
	require.Equal(t, "v1.19.0", page2.Releases[0].Tag)

	page2Cached, err := svc1.Releases(context.Background(), 2, nil)
	require.NoError(t, err)
	require.Equal(t, 2, upstreamHits)
	require.Equal(t, "v1.19.0", page2Cached.Releases[0].Tag)

	// 4. Explicit refresh bypasses cache, invalidates old page 2 cache, and persists updated page 1.
	currentTag = "v1.21.0"
	refreshed, err := svc2.RefreshReleases(context.Background(), 1, nil)
	require.NoError(t, err)
	require.Equal(t, 3, upstreamHits)
	require.Equal(t, "v1.21.0", refreshed.Releases[0].Tag)

	dir, err := releasePageCacheDir()
	require.NoError(t, err)
	_, err = os.Stat(releasePageCachePath(dir, runtime.GOOS, runtime.GOARCH, 2))
	require.True(t, os.IsNotExist(err), "stale page 2 cache should be cleared on explicit page 1 refresh")

	afterRefresh, err := svc1.Releases(context.Background(), 1, nil)
	require.NoError(t, err)
	require.Equal(t, 3, upstreamHits)
	require.Equal(t, "v1.21.0", afterRefresh.Releases[0].Tag)
}

func TestCoreReleasesIgnoresExpiredAndInvalidCacheEntries(t *testing.T) {
	newIsolatedService(t)
	upstreamHits := 0
	svc := NewCoreService()
	defer svc.Close()
	svc.releaseFetcher = func(_ context.Context, _ *config.Config, page int, goos, goarch string, _ func(coreinfra.Progress)) (coreinfra.ReleasePage, error) {
		upstreamHits++
		return coreinfra.ReleasePage{
			Page:     page,
			HasNext:  false,
			Releases: []coreinfra.Release{testStableReleaseForPlatform("v1.22.0", goos, goarch)},
		}, nil
	}

	dir, err := releasePageCacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0700))
	cachePath := releasePageCachePath(dir, runtime.GOOS, runtime.GOARCH, 1)

	// Expired cache triggers upstream fetch.
	expired := cachedReleasePage{
		FetchedAt: time.Now().UTC().Add(-(releasePageCacheTTL + time.Minute)),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		Page: coreinfra.ReleasePage{
			Page:     1,
			Releases: []coreinfra.Release{testStableReleaseForPlatform("v1.18.0", runtime.GOOS, runtime.GOARCH)},
		},
	}
	raw, err := json.Marshal(expired)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, raw, 0600))

	got, err := svc.Releases(context.Background(), 1, nil)
	require.NoError(t, err)
	require.Equal(t, 1, upstreamHits)
	require.Equal(t, "v1.22.0", got.Releases[0].Tag)

	// Prerelease injected into cache is rejected cleanly.
	badRelease := testStableReleaseForPlatform("v1.18.0", runtime.GOOS, runtime.GOARCH)
	badRelease.Prerelease = true
	tampered := cachedReleasePage{
		FetchedAt: time.Now().UTC(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		Page:      coreinfra.ReleasePage{Page: 1, Releases: []coreinfra.Release{badRelease}},
	}
	raw, err = json.Marshal(tampered)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, raw, 0600))

	got, err = svc.Releases(context.Background(), 1, nil)
	require.NoError(t, err)
	require.Equal(t, 2, upstreamHits)
	require.Equal(t, "v1.22.0", got.Releases[0].Tag)
}
