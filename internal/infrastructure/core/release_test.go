package core

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReleaseLocksTagAndRejectsGatewayDigest(t *testing.T) {
	digest := strings.Repeat("a", 64)
	json := fmt.Sprintf(`{"tag_name":"v1.2.3","assets":[{"name":"mihomo-linux-amd64-compatible-v1.2.3.gz","browser_download_url":"https://github.com/MetaCubeX/mihomo/releases/download/v1.2.3/mihomo-linux-amd64-compatible-v1.2.3.gz","digest":"sha256:%s","size":123}]}`, digest)
	f := testFetcher(t, func(route Route, _ *http.Request) (int, string, error) {
		if route.Name == "direct" {
			return 503, "", nil
		}
		return 200, json, nil
	})
	release, err := f.Release(context.Background(), "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	asset, err := release.SelectAsset("linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = release.TrustedDigest(asset, nil); err == nil {
		t.Fatal("gateway-supplied digest was trusted")
	}
	pins := map[string]string{"v1.2.3/" + asset.Name: digest}
	if got, err := release.TrustedDigest(asset, pins); err != nil || got != digest {
		t.Fatalf("%s %v", got, err)
	}
	release.Trusted = true
	if _, err = release.TrustedDigest(asset, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = release.SelectAsset("linux", "386"); err == nil {
		t.Fatal("unsupported CPU accepted")
	}
}
func TestReleaseRejectsWrongTagAndAssetOrigin(t *testing.T) {
	for _, body := range []string{
		`<html>blocked</html>`,
		`{"tag_name":"v9.9.9","assets":[{"name":"test.gz","browser_download_url":"https://github.com/MetaCubeX/mihomo/releases/download/v9.9.9/test.gz"}]}`,
		`{"tag_name":"v1.2.3","assets":[{"name":"test.gz","browser_download_url":"https://evil.example/test.gz"}]}`,
	} {
		f := testFetcher(t, func(Route, *http.Request) (int, string, error) { return 200, body, nil })
		if _, err := f.Release(context.Background(), "v1.2.3"); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func catalogRelease(tag, platform string) Release {
	name := "mihomo-" + platform + "-" + tag + ".gz"
	return Release{Tag: tag, Assets: []Asset{{Name: name, URL: "https://github.com/MetaCubeX/mihomo/releases/download/" + tag + "/" + name, Digest: "sha256:" + strings.Repeat("a", 64), Size: 4 << 20}}}
}

func TestReleaseListFiltersPlatformAndStableReleasesWithoutLosingPagination(t *testing.T) {
	input := []Release{catalogRelease("v1.9.0", "linux-amd64-compatible"), catalogRelease("v1.8.0", "linux-arm64")}
	for len(input) < ReleasePageSize {
		input = append(input, Release{Tag: "alpha", Prerelease: true})
	}
	input[0].PublishedAt = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	data, _ := json.Marshal(input)
	f := testFetcher(t, func(_ Route, req *http.Request) (int, string, error) {
		if req.URL.Query().Get("page") != "2" || req.URL.Query().Get("per_page") != "10" {
			t.Fatalf("wrong pagination: %s", req.URL)
		}
		return 200, string(data), nil
	})
	page, err := f.Releases(context.Background(), 2, "linux", "amd64")
	if err != nil || page.Page != 2 || !page.HasNext || len(page.Releases) != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	r := page.Releases[0]
	if r.Tag != "v1.9.0" || !r.Trusted || r.Route != "direct" || r.FetchedAt.IsZero() || r.PublishedAt != input[0].PublishedAt || len(r.Assets) != 1 {
		t.Fatalf("lost release metadata or provenance: %+v", r)
	}
}

func TestReleaseListRejectsMacOSBeforeFetching(t *testing.T) {
	requests := 0
	f := testFetcher(t, func(Route, *http.Request) (int, string, error) {
		requests++
		return 200, "[]", nil
	})
	page, err := f.Releases(context.Background(), 1, "darwin", "arm64")
	if err == nil || requests != 0 || page.Page != 0 {
		t.Fatalf("macOS should be rejected before fetching or returning a page: %+v, %v, %d requests", page, err, requests)
	}
}

func TestReleaseListFallbackDoesNotTrustAcceleratorMetadata(t *testing.T) {
	data, _ := json.Marshal([]Release{catalogRelease("v1.9.0", "linux-amd64-compatible")})
	var routes []string
	f := testFetcher(t, func(route Route, req *http.Request) (int, string, error) {
		routes = append(routes, route.Name)
		switch route.Name {
		case "direct":
			return 200, "<html>blocked</html>", nil
		case "proxy:localhost:8888":
			return 403, "limited", nil
		default:
			if !strings.Contains(req.URL.String(), "/https://api.github.com/repos/MetaCubeX/mihomo/releases?per_page=10&page=1") {
				t.Fatalf("list pagination was lost in gateway URL: %s", req.URL)
			}
			return 200, string(data), nil
		}
	})
	f.Proxy = "http://localhost:8888"
	page, err := f.Releases(context.Background(), 1, "linux", "amd64")
	if err != nil || len(page.Releases) != 1 {
		t.Fatalf("%+v %v", page, err)
	}
	if strings.Join(routes, ",") != "direct,proxy:localhost:8888,accelerator:gh-proxy.org" {
		t.Fatalf("fallback order: %v", routes)
	}
	r := page.Releases[0]
	if _, err := r.TrustedDigest(r.Assets[0], nil); err == nil || r.Trusted {
		t.Fatal("gateway metadata created a trusted digest")
	}
}

func TestReleaseListRejectsForgedAssetsAndMalformedResponses(t *testing.T) {
	forged := catalogRelease("v1.9.0", "linux-amd64-compatible")
	forged.Assets[0].URL = "https://github.com/MetaCubeX/mihomo/releases/download/v1.8.0/" + forged.Assets[0].Name
	badAsset, _ := json.Marshal([]Release{forged})
	duplicate, _ := json.Marshal([]Release{catalogRelease("v1.9.0", "linux-amd64-compatible"), catalogRelease("v1.9.0", "linux-amd64-compatible")})
	for _, body := range []string{"null", `{}`, `<html>not JSON</html>`, string(badAsset), string(duplicate)} {
		t.Run(body[:min(len(body), 40)], func(t *testing.T) {
			f := testFetcher(t, func(Route, *http.Request) (int, string, error) { return 200, body, nil })
			f.Accelerators = []Accelerator{}
			if _, err := f.Releases(context.Background(), 1, "linux", "amd64"); err == nil {
				t.Fatal("accepted invalid list")
			}
		})
	}
}

func TestReleaseListEmptyPagesAndInvalidQueries(t *testing.T) {
	calls := 0
	f := testFetcher(t, func(Route, *http.Request) (int, string, error) {
		calls++
		return 200, `[]`, nil
	})
	page, err := f.Releases(context.Background(), 3, "linux", "amd64")
	if err != nil || page.Page != 3 || page.HasNext || len(page.Releases) != 0 {
		t.Fatalf("empty page: %+v %v", page, err)
	}
	for _, page := range []int{0, -1} {
		if _, err := f.Releases(context.Background(), page, "linux", "amd64"); err == nil {
			t.Fatal("invalid page accepted")
		}
	}
	if calls != 1 {
		t.Fatal("invalid pages reached the network")
	}
	for _, raw := range []string{
		"https://api.github.com/repos/MetaCubeX/mihomo/releases?page=1&per_page=100",
		"https://api.github.com/repos/MetaCubeX/mihomo/releases?page=1&per_page=10&token=secret",
		"https://api.github.com/repos/MetaCubeX/mihomo/releases?page=1&page=2&per_page=10",
		"https://api.github.com/repos/MetaCubeX/mihomo/releases/latest?page=1&per_page=10",
		testAssetURL + "?page=1&per_page=10",
	} {
		if validateSource(raw) == nil {
			t.Fatalf("accepted unexpected query: %s", raw)
		}
	}
}

func TestListedReleaseDownloadsTheSelectedAssetWithoutActivation(t *testing.T) {
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	payload := []byte("verified core fixture, never executed")
	if _, err := gz.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(archive.Bytes())
	input := []Release{catalogRelease("v9.1.0", "linux-amd64-compatible"), catalogRelease("v9.0.0", "linux-amd64-compatible")}
	input[1].Assets[0].Digest = "sha256:" + hex.EncodeToString(hash[:])
	data, _ := json.Marshal(input)
	var downloaded string
	f := testFetcher(t, func(_ Route, req *http.Request) (int, string, error) {
		if req.URL.Host == "api.github.com" {
			return 200, string(data), nil
		}
		downloaded = req.URL.String()
		return 200, archive.String(), nil
	})
	page, err := f.Releases(context.Background(), 1, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	installed, err := f.Install(context.Background(), root, page.Releases[1], "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if downloaded != input[1].Assets[0].URL || installed.Version != "v9.0.0" {
		t.Fatal("download did not use the selected row")
	}
	binary, _ := BinaryPath(root, "v9.0.0", "linux")
	actual, err := os.ReadFile(binary)
	if err != nil || !bytes.Equal(actual, payload) {
		t.Fatalf("downloaded content: %q %v", actual, err)
	}
	if _, err := os.Stat(filepath.Join(root, "runtime")); !os.IsNotExist(err) {
		t.Fatal("download changed runtime state")
	}
}

func TestReleaseListDoesNotTruncateOlderHistoryAtAnArbitraryPage(t *testing.T) {
	f := testFetcher(t, func(_ Route, req *http.Request) (int, string, error) {
		if req.URL.Query().Get("page") != "1001" {
			t.Fatal("older page was not requested")
		}
		return 200, `[]`, nil
	})
	page, err := f.Releases(context.Background(), 1001, "linux", "amd64")
	if err != nil || page.Page != 1001 || page.HasNext {
		t.Fatalf("older history was capped: %+v %v", page, err)
	}
}

func TestReleasePaginationDoesNotRelaxProxyOrAcceleratorValidation(t *testing.T) {
	if _, err := ParseDownloadProxy("http://localhost:8888/?token=secret"); err == nil {
		t.Fatal("proxy query string was accepted")
	}
	f := NewFetcher()
	f.Accelerators = []Accelerator{{URL: "https://gh-proxy.org/?token=secret", Metadata: true, Asset: true}}
	if err := f.ValidateSettings(); err == nil {
		t.Fatal("accelerator query string was accepted")
	}
}

func TestReleaseListOnlyIncludesStableVersionsAndKeepsExplicitLookup(t *testing.T) {
	alpha := catalogRelease("Prerelease-Alpha", "linux-amd64-compatible")
	alpha.Prerelease = true
	alpha.Assets[0].Name = "mihomo-linux-amd64-compatible-alpha-478af2a.gz"
	alpha.Assets[0].URL = "https://github.com/MetaCubeX/mihomo/releases/download/Prerelease-Alpha/" + alpha.Assets[0].Name
	rc := catalogRelease("v1.20.0-rc.1", "linux-amd64-compatible")
	marked := catalogRelease("v1.18.0", "linux-amd64-compatible")
	marked.Prerelease = true
	stable := catalogRelease("v1.19.0", "linux-amd64-compatible")
	list, _ := json.Marshal([]Release{alpha, rc, marked, stable})
	f := testFetcher(t, func(_ Route, req *http.Request) (int, string, error) {
		if strings.Contains(req.URL.Path, "/tags/") || strings.HasSuffix(req.URL.Path, "/latest") {
			data, _ := json.Marshal(alpha)
			return 200, string(data), nil
		}
		return 200, string(list), nil
	})
	page, err := f.Releases(context.Background(), 1, "linux", "amd64")
	if err != nil || len(page.Releases) != 1 || page.Releases[0].Tag != stable.Tag || page.HasNext {
		t.Fatalf("list includes non-stable releases: %+v %v", page, err)
	}
	prereleases := make([]Release, ReleasePageSize)
	for i := range prereleases {
		prereleases[i] = alpha
	}
	list, _ = json.Marshal(prereleases)
	page, err = f.Releases(context.Background(), 2, "linux", "amd64")
	if err != nil || len(page.Releases) != 0 || !page.HasNext || page.Page != 2 {
		t.Fatalf("filtered prerelease page ended browsing: %+v %v", page, err)
	}
	lookup, err := f.Release(context.Background(), "Prerelease-Alpha")
	if err != nil || lookup.Version() != "alpha-478af2a" {
		t.Fatalf("alpha lookup failed: %+v %v", lookup, err)
	}
	f.Accelerators = []Accelerator{}
	if _, err = f.Release(context.Background(), "latest"); err == nil {
		t.Fatal("stable update accepted prerelease metadata")
	}
	for _, version := range []string{"alpha-478af2a", "beta-1234567", "v1.20.0-rc.1"} {
		path, err := BinaryPath(t.TempDir(), version, "windows")
		if err != nil || !strings.HasSuffix(path, "mihomo.exe") {
			t.Fatalf("prerelease cannot be installed: %s %v", version, err)
		}
	}
}

func TestAlphaInstallUsesBuildVersionAndVerifiesArchive(t *testing.T) {
	var archive bytes.Buffer
	w := gzip.NewWriter(&archive)
	_, _ = w.Write([]byte("alpha executable"))
	_ = w.Close()
	digest := sha256.Sum256(archive.Bytes())
	r := Release{Tag: "Prerelease-Alpha", Prerelease: true, Trusted: true}
	name := "mihomo-linux-amd64-compatible-alpha-478af2a.gz"
	r.Assets = []Asset{{Name: name, URL: "https://github.com/MetaCubeX/mihomo/releases/download/Prerelease-Alpha/" + name, Digest: "sha256:" + hex.EncodeToString(digest[:])}}
	f := testFetcher(t, func(Route, *http.Request) (int, string, error) { return 200, archive.String(), nil })
	root := t.TempDir()
	installed, err := f.Install(context.Background(), root, r, "linux", "amd64")
	if err != nil || installed.Version != "alpha-478af2a" {
		t.Fatalf("alpha installation failed: %+v %v", installed, err)
	}
	binary, _ := BinaryPath(root, installed.Version, "linux")
	data, err := os.ReadFile(binary)
	if err != nil || string(data) != "alpha executable" {
		t.Fatalf("wrong executable installation: %s %v", data, err)
	}
	r.Assets[0].Digest = "sha256:" + strings.Repeat("0", 64)
	if _, err = f.Install(context.Background(), t.TempDir(), r, "linux", "amd64"); err == nil {
		t.Fatal("alpha download skipped checksum verification")
	}
}
