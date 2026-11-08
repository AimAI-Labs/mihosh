package core

import (
	"context"
	"errors"
	"net/http"
	"os"
	"runtime"
	"testing"
	"time"
)

// Opt in only: force the direct stage to fail, then exercise the production gateway
// transport against the real release API and a complete independently pinned asset.
func TestGitHubAcceleratorRealInstall(t *testing.T) {
	if os.Getenv("MIHOSH_CORE_NETWORK_TEST") != "1" {
		t.Skip("set MIHOSH_CORE_NETWORK_TEST=1 to verify real accelerated downloads")
	}
	f := NewFetcher()
	f.envProxy = nil
	f.clientFor = func(route Route, metadata bool) *http.Client {
		if route.Name == "direct" {
			return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("forced direct failure for fallback verification")
			})}
		}
		return newDownloadClient(route, metadata)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	release, err := f.Release(ctx, PinnedRelease().Tag)
	if err != nil {
		t.Fatal(err)
	}
	if release.Trusted {
		t.Fatal("gateway metadata must not establish trust")
	}
	if _, err = release.SelectAsset(runtime.GOOS, runtime.GOARCH); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	installation, err := f.Install(ctx, root, release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if installation.Route == "direct" || installation.BinarySHA256 == "" {
		t.Fatalf("unverified accelerated installation: %+v", installation)
	}
	t.Logf("verified release %s via %s", installation.Version, installation.Route)
}
