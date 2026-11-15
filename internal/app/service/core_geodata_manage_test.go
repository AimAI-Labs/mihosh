package service

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
)

func geoDataManageFixture(t *testing.T, data string) (*CoreService, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	cfg := config.DefaultConfig
	cfg.Core.Mode = "managed"
	if err := config.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "runtime")
	if data != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	svc := NewCoreService()
	t.Cleanup(svc.Close)
	return svc, dir
}

func TestGeoDataFilesShowsMissingDefaultsWithoutTakingOperationLock(t *testing.T) {
	svc, dir := geoDataManageFixture(t, "")
	svc.mu.Lock()
	defer svc.mu.Unlock()
	files, err := svc.GeoDataFiles()
	if err != nil || len(files) != 4 {
		t.Fatalf("files=%v err=%v", files, err)
	}
	for i, file := range files {
		resource := geoDataResources[i]
		if file.Key != resource.key || file.Name != resource.file || file.Path != filepath.Join(dir, resource.file) || file.Asset != resource.asset || !file.Updatable || file.Exists || file.Size != 0 || !file.UpdatedAt.IsZero() {
			t.Fatalf("unexpected missing resource: %+v", file)
		}
	}
}

func TestGeoDataFilesPreservesMetadataAliasesAndOfficialLiteChoice(t *testing.T) {
	for _, alias := range []string{"country.MMDB", "geoip.metadb", "geoip.db"} {
		t.Run(alias, func(t *testing.T) {
			svc, dir := geoDataManageFixture(t, "geox-url:\n  mmdb: https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country-lite.mmdb\n  geoip: https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip-lite.dat\n")
			path := filepath.Join(dir, alias)
			if err := os.WriteFile(path, []byte("database"), 0600); err != nil {
				t.Fatal(err)
			}
			mtime := time.Unix(1700000000, 0)
			if err := os.Chtimes(path, mtime, mtime); err != nil {
				t.Fatal(err)
			}
			files, err := svc.GeoDataFiles()
			if err != nil {
				t.Fatal(err)
			}
			file := files[0]
			if file.Name != alias || file.Path != path || file.Asset != "country-lite.mmdb" || !file.Exists || !file.Updatable || file.Size != 8 || !file.UpdatedAt.Equal(mtime) {
				t.Fatalf("unexpected existing resource: %+v", file)
			}
			if files[1].Asset != "geoip-lite.dat" || files[1].Exists || !files[1].Updatable {
				t.Fatalf("explicit GeoIP dataset changed: %+v", files[1])
			}
		})
	}
}

func TestGeoDataCustomProviderRemainsUntouched(t *testing.T) {
	svc, dir := geoDataManageFixture(t, "geox-url: {mmdb: 'https://custom.example/private.mmdb'}\n")
	path := filepath.Join(dir, "Country.mmdb")
	if err := os.WriteFile(path, []byte("custom"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := svc.GeoDataFiles()
	if err != nil || files[0].Updatable || files[0].Asset != "" || !files[0].Exists {
		t.Fatalf("files=%+v err=%v", files, err)
	}
	if err := svc.UpdateGeoData(context.Background(), "mmdb", func(coreinfra.Progress) { t.Fatal("custom source caused a request") }); err == nil || !strings.Contains(err.Error(), "custom") {
		t.Fatalf("custom update error=%v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "custom" {
		t.Fatalf("custom database changed: %q err=%v", data, err)
	}
}

func TestGeoDataManagementRejectsExternalMode(t *testing.T) {
	svc, _ := geoDataManageFixture(t, "{}\n")
	if err := config.Update(func(cfg *config.Config) error { cfg.Core.Mode = "external"; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GeoDataFiles(); err == nil || !strings.Contains(err.Error(), "managed") {
		t.Fatalf("external list error=%v", err)
	}
	if err := svc.UpdateGeoData(context.Background(), "mmdb", nil); err == nil || !strings.Contains(err.Error(), "managed") {
		t.Fatalf("external update error=%v", err)
	}
}

func TestGeoDataUpdateRejectsUnknownBusyAndCancelledBeforeNetwork(t *testing.T) {
	svc, dir := geoDataManageFixture(t, "")
	notify := func(coreinfra.Progress) { t.Fatal("rejected operation caused a request") }
	if err := svc.UpdateGeoData(context.Background(), "../other", notify); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("invalid key error=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := svc.UpdateGeoData(ctx, "mmdb", notify); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled update error=%v", err)
	}
	svc.mu.Lock()
	err := svc.UpdateGeoData(context.Background(), "mmdb", notify)
	svc.mu.Unlock()
	if err == nil || !strings.Contains(err.Error(), "active") {
		t.Fatalf("busy update error=%v", err)
	}
	svc.shutdown()
	if err := svc.UpdateGeoData(context.Background(), "mmdb", notify); !errors.Is(err, context.Canceled) {
		t.Fatalf("closed service update error=%v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("rejected operation created runtime directory: %v", err)
	}
}

func TestGeoDataUpdateDownloadsVerifiedOfficialAssetAndRefreshesFileStatus(t *testing.T) {
	const assetURL = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country-lite.mmdb"
	const configuration = "geox-url: {mmdb: '" + assetURL + "'}\n"
	svc, dir := geoDataManageFixture(t, configuration)
	path := filepath.Join(dir, "Country.mmdb")
	if err := os.WriteFile(path, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Unix(1700000000, 0)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	const body = "verified updated lite GEO database"
	digest := sha256.Sum256([]byte(body))
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch {
		case r.Host == "api.github.com" && r.URL.Path == "/repos/MetaCubeX/meta-rules-dat/releases/tags/latest":
			release := coreinfra.Release{Tag: "latest", Assets: []coreinfra.Asset{{
				Name: "country-lite.mmdb", URL: assetURL,
				Digest: "sha256:" + hex.EncodeToString(digest[:]), Size: int64(len(body)),
			}}}
			if err := json.NewEncoder(w).Encode(release); err != nil {
				t.Error(err)
			}
		case r.Host == "github.com" && r.URL.Path == "/MetaCubeX/meta-rules-dat/releases/download/latest/country-lite.mmdb":
			if _, err := w.Write([]byte(body)); err != nil {
				t.Error(err)
			}
		default:
			t.Errorf("unexpected official request: %s %s", r.Host, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.ServerName = server.Listener.Addr().(*net.TCPAddr).IP.String()
	dialer := tls.Dialer{Config: transport.TLSClientConfig}
	transport.DialTLSContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}
	previousTransport := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = previousTransport
		transport.CloseIdleConnections()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var progress coreinfra.Progress
	if err := svc.UpdateGeoData(ctx, "mmdb", func(p coreinfra.Progress) { progress = p }); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 || progress.Downloaded != int64(len(body)) {
		t.Fatalf("requests=%d progress=%+v", requests.Load(), progress)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != body {
		t.Fatalf("updated database=%q err=%v", data, err)
	}
	if _, err := os.Stat(path + ".part"); !os.IsNotExist(err) {
		t.Fatalf("partial download remains: %v", err)
	}
	files, err := svc.GeoDataFiles()
	if err != nil {
		t.Fatal(err)
	}
	file := files[0]
	if !file.Exists || file.Size != int64(len(body)) || file.Asset != "country-lite.mmdb" || file.Path != path || !file.UpdatedAt.After(oldTime) {
		t.Fatalf("updated file status=%+v", file)
	}
	data, err = os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil || string(data) != configuration {
		t.Fatalf("runtime configuration changed: %q err=%v", data, err)
	}
}
