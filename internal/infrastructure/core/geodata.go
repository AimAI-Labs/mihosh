package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
)

const geoDataAPI = "https://api.github.com/repos/MetaCubeX/meta-rules-dat/releases/tags/latest"
const geoDataURL = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/"

// IsGeoDataAsset identifies resources supported by the official downloader.
func IsGeoDataAsset(name string) bool {
	switch name {
	case "country.mmdb", "country-lite.mmdb", "geoip.metadb", "geoip.dat", "geoip-lite.dat", "geosite.dat", "geosite-lite.dat", "GeoLite2-ASN.mmdb":
		return true
	}
	return false
}

// DownloadGeoData accepts asset digests only from a trusted official API route.
func (f *Fetcher) DownloadGeoData(ctx context.Context, name, destination string) error {
	if !IsGeoDataAsset(name) {
		return errors.New("unsupported GeoData asset")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Manual updates and startup validation can run in different Mihosh processes.
	unlock, err := LockRuntime(filepath.Join(filepath.Dir(destination), ".geo-data.lock"))
	if err != nil {
		return fmt.Errorf("cannot start GEO download: %w", err)
	}
	defer unlock()
	var asset Asset
	_, route, err := f.Metadata(ctx, geoDataAPI, func(data []byte) error {
		var release Release
		if err := json.Unmarshal(data, &release); err != nil || release.Tag != "latest" || release.Draft {
			return errors.New("invalid GeoData release metadata")
		}
		var match Asset
		for _, candidate := range release.Assets {
			if candidate.Name != name {
				continue
			}
			if match.Name != "" {
				return errors.New("duplicate GeoData asset")
			}
			if candidate.URL != geoDataURL+name || candidate.Size <= 0 || candidate.Size > MaxAssetBytes || !validDigest(candidate.Digest) {
				return errors.New("invalid GeoData asset metadata")
			}
			match = candidate
		}
		if match.Name == "" {
			return errors.New("GeoData asset missing from release")
		}
		asset = match
		return nil
	})
	if err != nil {
		return err
	}
	if !route.Trusted() {
		return errors.New("no independently trusted GeoData digest; retry through direct/proxy access")
	}
	_, err = f.Download(ctx, asset.URL, destination, asset.Digest)
	return err
}
