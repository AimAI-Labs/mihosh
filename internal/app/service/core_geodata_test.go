package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func geoValidationFixture(t *testing.T, script string) (binary, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX shell")
	}
	dir := t.TempDir()
	binary, path = filepath.Join(dir, "mihomo"), filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	return binary, path
}

func TestManagedValidationFetchesMissingGeoDataAndRevalidatesCompleteConfig(t *testing.T) {
	binary, path := geoValidationFixture(t, `
if [ ! -s "$3/Country.mmdb" ]; then
  printf "can't download MMDB: network unavailable\n" >&2
  exit 1
fi
if [ ! -s "$3/GeoSite.dat" ]; then
  printf "can't download GeoSite.dat: network unavailable\n" >&2
  exit 1
fi
exit 0
`)
	data := []byte("rules: ['GEOIP,CN,DIRECT', 'GEOSITE,cn,DIRECT']\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	var assets []string
	download := func(_ context.Context, asset, destination string) error {
		assets = append(assets, asset)
		return os.WriteFile(destination, []byte("fixture database"), 0600)
	}
	if err := validateWithGeoData(context.Background(), binary, path, path, data, download); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(assets, []string{"country.mmdb", "geosite.dat"}) {
		t.Fatal("missing databases were not prepared in order", assets)
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != string(data) {
		t.Fatal("validation changed subscription rules", err)
	}
	assets = nil
	if err := validateWithGeoData(context.Background(), binary, path, path, data, download); err != nil || len(assets) != 0 {
		t.Fatal("cached databases caused another download", err)
	}
}

func TestManagedValidationInterruptsStalledNativeDownload(t *testing.T) {
	binary, path := geoValidationFixture(t, `
if [ ! -s "$3/Country.mmdb" ]; then
  printf "Can't find MMDB, start download\n"
  exec sleep 30
fi
exit 0
`)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	called := false
	err := validateWithGeoData(ctx, binary, path, path, []byte("password: MMDB\nrules: []\n"), func(ctx context.Context, asset, destination string) error {
		called = true
		if ctx.Err() != nil {
			t.Fatal("native timeout exhausted the application download context")
		}
		return os.WriteFile(destination, []byte("fixture database"), 0600)
	})
	if err != nil || !called {
		t.Fatal("stalled native download prevented application fallback", err)
	}
}

func TestManagedValidationDoesNotRetryConfigErrorsOrCustomGeoSources(t *testing.T) {
	for _, entry := range []struct{ output, data string }{
		{"proxy 0: unsupported proxy type", "proxies: []\n"},
		{"can't download MMDB: unavailable", "geox-url: {mmdb: 'https://custom.example/database.mmdb'}\n"},
	} {
		binary, path := geoValidationFixture(t, "printf '%s\\n' \""+entry.output+"\" >&2\nexit 1\n")
		err := validateWithGeoData(context.Background(), binary, path, path, []byte(entry.data), func(context.Context, string, string) error {
			t.Fatal("unrelated error caused a download")
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), entry.output) {
			t.Fatal("configuration error was lost", err)
		}
	}
}

func TestManagedGeoPreparationPreservesCancellationAndStopsRepeatedFailures(t *testing.T) {
	binary, path := geoValidationFixture(t, "printf \"can't download MMDB: unavailable\\n\" >&2\nexit 1\n")
	data := []byte("rules: []\n")
	err := validateWithGeoData(context.Background(), binary, path, path, data, func(context.Context, string, string) error {
		return context.Canceled
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("download cancellation was lost", err)
	}
	attempts := 0
	err = validateWithGeoData(context.Background(), binary, path, path, data, func(context.Context, string, string) error {
		attempts++
		return nil
	})
	if err == nil || attempts != 1 {
		t.Fatal("unusable database caused repeated downloads", attempts, err)
	}
}

func TestMissingGeoDataPreservesOfficialDatasetChoiceAndExistingFilename(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "geoip.metadb"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	err := errors.New("can't download MMDB: unavailable")
	for _, raw := range []string{
		"https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country-lite.mmdb",
		"https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/country-lite.mmdb",
	} {
		asset, destination, ok := missingGeoData(err, []byte("geox-url: {mmdb: '"+raw+"'}\n"), dir)
		if !ok || asset != "country-lite.mmdb" || destination != filepath.Join(dir, "geoip.metadb") {
			t.Fatal("explicit GEO dataset or existing path was changed", asset, destination)
		}
	}
	for _, raw := range []string{
		"https://evil.example/gh/MetaCubeX/meta-rules-dat@release/country.mmdb",
		"https://github.com/other/repo/releases/download/latest/country.mmdb",
		"https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country.mmdb?token=secret",
		"https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/unknown.mmdb",
	} {
		if _, ok := officialGeoDataAsset(raw); ok {
			t.Fatal("custom source entered official gateway chain")
		}
	}
	for _, output := range []string{"can't download ASN.mmdb: unavailable", "can't download ASN: unavailable", "ASN invalid, remove and download"} {
		asset, destination, ok := missingGeoData(errors.New(output), []byte("{}"), dir)
		if !ok || asset != "GeoLite2-ASN.mmdb" || destination != filepath.Join(dir, "ASN.mmdb") {
			t.Fatal("ASN recovery branch was not recognized", asset, destination)
		}
	}
}
