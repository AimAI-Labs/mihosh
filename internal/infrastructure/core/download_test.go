package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

const testAssetURL = "https://github.com/MetaCubeX/mihomo/releases/download/v1.2.3/test.gz"

func TestDefaultAcceleratorRoutes(t *testing.T) {
	f := NewFetcher()
	f.envProxy = nil
	for _, metadata := range []bool{false, true} {
		routes, err := f.routes(testAssetURL, metadata)
		want := []Route{{Name: "direct"}, {Name: "accelerator:gh-proxy.org", Gateway: "https://gh-proxy.org/"}}
		if err != nil || !reflect.DeepEqual(routes, want) {
			t.Fatalf("metadata=%v routes=%v err=%v", metadata, routes, err)
		}
	}
	f.Accelerators = []Accelerator{}
	routes, err := f.routes(testAssetURL, false)
	if err != nil || len(routes) != 1 || routes[0].Name != "direct" {
		t.Fatalf("disabled accelerators: routes=%v err=%v", routes, err)
	}
}

func testFetcher(t *testing.T, fn func(Route, *http.Request) (int, string, error)) *Fetcher {
	t.Helper()
	f := NewFetcher()
	f.envProxy = nil
	f.clientFor = func(route Route, _ bool) *http.Client {
		return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			status, body, err := fn(route, req)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Request: req}, nil
		})}
	}
	return f
}
func TestRoutesAndResponseValidation(t *testing.T) {
	var got []string
	f := testFetcher(t, func(route Route, req *http.Request) (int, string, error) {
		got = append(got, route.Name)
		switch route.Name {
		case "direct":
			return 0, "", errors.New("network unavailable")
		case "proxy:localhost:8888":
			return 200, "<html>blocked</html>", nil
		default:
			if !strings.HasPrefix(req.URL.String(), "https://gh-proxy.org/https://api.github.com/") {
				t.Errorf("unexpected gateway URL %s", req.URL)
			}
			if req.Header.Get("Authorization") != "" {
				t.Fatal("credential forwarded")
			}
			return 200, `{"tag":"ok"}`, nil
		}
	})
	f.Proxy = "http://user:private@localhost:8888"
	_, route, err := f.Metadata(context.Background(), releaseAPI+"latest", func(b []byte) error {
		if !strings.HasPrefix(string(b), "{") {
			return errors.New("not JSON")
		}
		return nil
	})
	if err != nil || route.Trusted() {
		t.Fatalf("route=%+v err=%v", route, err)
	}
	want := []string{"direct", "direct", "proxy:localhost:8888", "accelerator:gh-proxy.org"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}
func TestDirectSuccessSkipsProxyAndGateway(t *testing.T) {
	calls := 0
	f := testFetcher(t, func(route Route, _ *http.Request) (int, string, error) {
		calls++
		if route.Name != "direct" {
			t.Fatal(route)
		}
		return 200, "{}", nil
	})
	f.Proxy = "http://localhost:8888"
	_, _, err := f.Metadata(context.Background(), releaseAPI+"latest", func([]byte) error { return nil })
	if err != nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
func TestProxySuccessSkipsGateway(t *testing.T) {
	calls := 0
	f := testFetcher(t, func(route Route, _ *http.Request) (int, string, error) {
		calls++
		if route.Name == "direct" {
			return 502, "", nil
		}
		return 200, "{}", nil
	})
	f.Proxy = "http://localhost:8888"
	_, route, err := f.Metadata(context.Background(), releaseAPI+"latest", func([]byte) error { return nil })
	if err != nil || calls != 3 || !route.Trusted() {
		t.Fatalf("calls=%d route=%v err=%v", calls, route, err)
	}
}
func TestDownloadRestartsAndVerifiesBeforeCommit(t *testing.T) {
	body := "verified compressed asset"
	digest := sha256.Sum256([]byte(body))
	destination := filepath.Join(t.TempDir(), "asset.gz")
	if err := os.WriteFile(destination, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}
	var got []string
	f := testFetcher(t, func(route Route, _ *http.Request) (int, string, error) {
		got = append(got, route.Name)
		if route.Name == "direct" {
			return 200, "corrupt", nil
		}
		return 200, body, nil
	})
	route, err := f.Download(context.Background(), testAssetURL, destination, hex.EncodeToString(digest[:]))
	if err != nil || route.Gateway == "" {
		t.Fatalf("%v %v", route, err)
	}
	data, _ := os.ReadFile(destination)
	if string(data) != body {
		t.Fatalf("got %q", data)
	}
	if len(got) != 2 {
		t.Fatalf("routes %v", got)
	}
	if _, err := os.Stat(destination + ".part"); !os.IsNotExist(err) {
		t.Fatal("part left behind")
	}
}
func TestDownloadFailurePreservesPreviousFile(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "asset.gz")
	if err := os.WriteFile(destination, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}
	f := testFetcher(t, func(Route, *http.Request) (int, string, error) { return 200, "corrupt", nil })
	_, err := f.Download(context.Background(), testAssetURL, destination, strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("accepted invalid digest")
	}
	data, _ := os.ReadFile(destination)
	if string(data) != "previous" {
		t.Fatalf("got %q", data)
	}
	if _, err := os.Stat(destination + ".part"); !os.IsNotExist(err) {
		t.Fatal("part left behind")
	}
}
func TestDownloadCancellationPreservesPreviousFileAndStopsFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body := strings.Repeat("buffered asset", 16<<10)
	digest := sha256.Sum256([]byte(body))
	destination := filepath.Join(t.TempDir(), "asset.gz")
	if err := os.WriteFile(destination, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	f := testFetcher(t, func(Route, *http.Request) (int, string, error) {
		calls++
		return http.StatusOK, body, nil
	})
	f.Notify = func(progress Progress) {
		if progress.Downloaded > 0 {
			cancel()
		}
	}
	_, err := f.Download(ctx, testAssetURL, destination, hex.EncodeToString(digest[:]))
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "previous" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	if _, err := os.Stat(destination + ".part"); !os.IsNotExist(err) {
		t.Fatal("part left behind after cancellation")
	}
}

func TestCancellationStopsFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	f := testFetcher(t, func(Route, *http.Request) (int, string, error) {
		calls++
		cancel()
		return 0, "", errors.New("cancelled")
	})
	_, _, err := f.Metadata(ctx, releaseAPI+"latest", func([]byte) error { return nil })
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
func TestSourceAndRedirectRestrictions(t *testing.T) {
	for _, raw := range []string{"https://evil.example/file", "http://github.com/MetaCubeX/mihomo/releases/download/a/b", "https://api.github.com/repos/other/repo/releases/latest"} {
		if validateSource(raw) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, raw := range []string{"http://github.com/file", "https://evil.example/file", "https://github.com:443/file", "https://user:password@github.com/file"} {
		u, _ := url.Parse(raw)
		if allowedRedirect(u, Route{}) {
			t.Fatalf("accepted redirect %s", raw)
		}
	}
	u, _ := url.Parse("https://release-assets.githubusercontent.com/file?signature=secret")
	if !allowedRedirect(u, Route{}) {
		t.Fatal("official CDN rejected")
	}
}
func TestFailureRedactsProxyCredentials(t *testing.T) {
	f := testFetcher(t, func(Route, *http.Request) (int, string, error) {
		return 0, "", errors.New("http://user:password@localhost:8888 failed")
	})
	f.Proxy = "http://user:password@localhost:8888"
	_, _, err := f.Metadata(context.Background(), releaseAPI+"latest", func([]byte) error { return nil })
	if err == nil || strings.Contains(err.Error(), "password") {
		t.Fatalf("error %v", err)
	}
}
func TestAcceleratorCapabilityAndProxyDeduplication(t *testing.T) {
	f := NewFetcher()
	f.Proxy = "http://localhost:8888"
	f.RunningProxy = f.Proxy
	f.envProxy = func(*http.Request) (*url.URL, error) { return url.Parse(f.Proxy) }
	f.Accelerators = []Accelerator{{"https://asset.example/", false, true}, {"https://api.example/", true, false}, {"https://api.example", true, false}}
	routes, err := f.routes(releaseAPI+"latest", true)
	if err != nil || len(routes) != 3 || routes[2].Name != "accelerator:api.example" {
		t.Fatalf("%v %v", routes, err)
	}
}

func TestNetworkStagesHaveIndependentTransportAndTimeBudgets(t *testing.T) {
	f := NewFetcher()
	direct := f.client(Route{Name: "direct"}, false)
	defer direct.CloseIdleConnections()
	if direct.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("direct stage uses environment proxy")
	}
	gateway := f.client(Route{Name: "accelerator:test", Gateway: "https://gh-proxy.org/"}, false)
	defer gateway.CloseIdleConnections()
	if gateway.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("gateway stage uses forward proxy")
	}
	if direct.Timeout >= gateway.Timeout {
		t.Fatal("direct download can consume the entire fallback budget")
	}
}

func TestConfiguredDownloadBudgetScalesStagesAndRejectsInvalidLimits(t *testing.T) {
	f := NewFetcher()
	f.DownloadTimeout = 20 * time.Minute
	client := f.client(Route{Name: "direct"}, false)
	defer client.CloseIdleConnections()
	if client.Timeout != 2*time.Minute {
		t.Fatalf("unexpected direct route budget: %s", client.Timeout)
	}
	f.DownloadTimeout = -time.Second
	if _, err := f.routes(testAssetURL, false); err == nil {
		t.Fatal("negative download timeout accepted")
	}
}
