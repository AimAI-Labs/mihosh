package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const geoDataTestBody = "verified geodata"

func geoDataTestRelease() Release {
	digest := sha256.Sum256([]byte(geoDataTestBody))
	return Release{Tag: "latest", Assets: []Asset{{
		Name: "country.mmdb", URL: geoDataURL + "country.mmdb",
		Digest: "sha256:" + hex.EncodeToString(digest[:]), Size: int64(len(geoDataTestBody)),
	}}}
}

func geoDataTestJSON(t *testing.T, release Release) string {
	t.Helper()
	data, err := json.Marshal(release)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestDownloadGeoDataUsesTrustedMetadataAndAssetFallback(t *testing.T) {
	for _, name := range []string{"country.mmdb", "country-lite.mmdb", "geoip.metadb", "geoip.dat", "geoip-lite.dat", "geosite.dat", "geosite-lite.dat", "GeoLite2-ASN.mmdb"} {
		t.Run(name, func(t *testing.T) {
			release := geoDataTestRelease()
			release.Assets[0].Name, release.Assets[0].URL = name, geoDataURL+name
			metadata := geoDataTestJSON(t, release)
			mirrored := false
			f := testFetcher(t, func(route Route, req *http.Request) (int, string, error) {
				if req.URL.String() == geoDataAPI {
					return http.StatusOK, metadata, nil
				}
				if route.Gateway == "" {
					return http.StatusForbidden, "", nil
				}
				if req.URL.String() != route.Gateway+geoDataURL+name {
					t.Fatalf("unexpected asset URL: %s", req.URL)
				}
				mirrored = true
				return http.StatusOK, geoDataTestBody, nil
			})
			destination := filepath.Join(t.TempDir(), name)
			if err := f.DownloadGeoData(context.Background(), name, destination); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(destination)
			if err != nil || string(data) != geoDataTestBody || !mirrored {
				t.Fatalf("data=%q mirrored=%v err=%v", data, mirrored, err)
			}
		})
	}
}

func TestDownloadGeoDataDigestMismatchPreservesPreviousFile(t *testing.T) {
	metadata := geoDataTestJSON(t, geoDataTestRelease())
	f := testFetcher(t, func(_ Route, req *http.Request) (int, string, error) {
		if req.URL.String() == geoDataAPI {
			return http.StatusOK, metadata, nil
		}
		return http.StatusOK, "corrupt", nil
	})
	destination := filepath.Join(t.TempDir(), "country.mmdb")
	if err := os.WriteFile(destination, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.DownloadGeoData(context.Background(), "country.mmdb", destination); err == nil {
		t.Fatal("accepted corrupt GeoData")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "previous" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	if _, err := os.Stat(destination + ".part"); !os.IsNotExist(err) {
		t.Fatal("partial GeoData left behind")
	}
}

func TestDownloadGeoDataRejectsUntrustedMetadata(t *testing.T) {
	metadata := geoDataTestJSON(t, geoDataTestRelease())
	assets := 0
	f := testFetcher(t, func(route Route, req *http.Request) (int, string, error) {
		if !strings.HasSuffix(req.URL.String(), geoDataAPI) {
			assets++
		}
		if route.Gateway == "" {
			return http.StatusForbidden, "", nil
		}
		return http.StatusOK, metadata, nil
	})
	err := f.DownloadGeoData(context.Background(), "country.mmdb", filepath.Join(t.TempDir(), "country.mmdb"))
	if err == nil || !strings.Contains(err.Error(), "independently trusted") || assets != 0 {
		t.Fatalf("assets=%d err=%v", assets, err)
	}
}

func TestDownloadGeoDataRejectsInvalidMetadata(t *testing.T) {
	for name, mutate := range map[string]func(*Release){
		"wrong tag":     func(r *Release) { r.Tag = "other" },
		"draft":         func(r *Release) { r.Draft = true },
		"wrong source":  func(r *Release) { r.Assets[0].URL = "https://example.com/country.mmdb" },
		"duplicate":     func(r *Release) { r.Assets = append(r.Assets, r.Assets[0]) },
		"missing":       func(r *Release) { r.Assets = nil },
		"empty size":    func(r *Release) { r.Assets[0].Size = 0 },
		"excess size":   func(r *Release) { r.Assets[0].Size = MaxAssetBytes + 1 },
		"absent digest": func(r *Release) { r.Assets[0].Digest = "" },
	} {
		t.Run(name, func(t *testing.T) {
			release := geoDataTestRelease()
			mutate(&release)
			metadata := geoDataTestJSON(t, release)
			f := testFetcher(t, func(_ Route, req *http.Request) (int, string, error) {
				if !strings.HasSuffix(req.URL.String(), geoDataAPI) {
					t.Fatalf("asset requested with invalid metadata: %s", req.URL)
				}
				return http.StatusOK, metadata, nil
			})
			if err := f.DownloadGeoData(context.Background(), "country.mmdb", filepath.Join(t.TempDir(), "country.mmdb")); err == nil {
				t.Fatal("accepted invalid GeoData metadata")
			}
		})
	}
}

func TestDownloadGeoDataRejectsUnknownAssetWithoutRequests(t *testing.T) {
	f := testFetcher(t, func(Route, *http.Request) (int, string, error) {
		t.Fatal("unknown asset caused a request")
		return 0, "", nil
	})
	for _, name := range []string{"other.mmdb", "../country.mmdb", "Country.mmdb", ""} {
		if err := f.DownloadGeoData(context.Background(), name, filepath.Join(t.TempDir(), "geodata")); err == nil {
			t.Fatalf("accepted unknown asset %q", name)
		}
	}
}
