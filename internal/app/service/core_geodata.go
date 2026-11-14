package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"gopkg.in/yaml.v3"
)

type geoDataRequired struct {
	asset, destination string
}

func (g *geoDataRequired) Error() string { return "core validation requires GEO database " + g.asset }

func (s *CoreService) validateConfig(ctx context.Context, cfg *config.Config, binary, path, candidate string, data []byte) error {
	return validateWithGeoData(ctx, binary, path, candidate, data, func(ctx context.Context, asset, destination string) error {
		fetcher := coreFetcher(cfg, nil)
		fetcher.RunningProxy = managedDownloadProxy(cfg)
		return fetcher.DownloadGeoData(ctx, asset, destination)
	})
}

// The core reports which database is needed; configs without GEO dependencies stay offline.
func validateWithGeoData(ctx context.Context, binary, path, candidate string, data []byte, download func(context.Context, string, string) error) error {
	attempted := make(map[string]bool)
	for {
		err := validateManagedProfile(ctx, binary, path, candidate, data, func(output string) error {
			if !strings.Contains(output, "start download") && !strings.Contains(output, "remove and download") && !strings.Contains(output, "can't download") {
				return nil
			}
			asset, destination, ok := missingGeoData(errors.New(output), data, filepath.Dir(path))
			if ok && !attempted[destination] {
				return &geoDataRequired{asset: asset, destination: destination}
			}
			return nil
		})
		if err == nil || ctx.Err() != nil {
			return err
		}
		var missing *geoDataRequired
		if !errors.As(err, &missing) || attempted[missing.destination] {
			return err
		}
		attempted[missing.destination] = true
		if fetchErr := download(ctx, missing.asset, missing.destination); fetchErr != nil {
			return errors.Join(err, fmt.Errorf("prepare GEO database %s: %w", missing.asset, fetchErr))
		}
	}
}

func missingGeoData(validationErr error, data []byte, dir string) (asset, destination string, ok bool) {
	var cfg struct {
		URLs map[string]string `yaml:"geox-url"`
	}
	if yaml.Unmarshal(data, &cfg) != nil {
		return "", "", false
	}
	for _, resource := range []struct{ key, file, asset, failure, starting, invalid string }{
		{"mmdb", "Country.mmdb", "country.mmdb", "can't download MMDB:", "Can't find MMDB, start download", "MMDB invalid, remove and download"},
		{"geoip", "GeoIP.dat", "geoip.dat", "can't download GeoIP.dat:", "Can't find GeoIP.dat, start download", "GeoIP.dat invalid, remove and download"},
		{"geosite", "GeoSite.dat", "geosite.dat", "can't download GeoSite.dat:", "Can't find GeoSite.dat, start download", "GeoSite.dat invalid, remove and download"},
		{"asn", "ASN.mmdb", "GeoLite2-ASN.mmdb", "can't download ASN", "Can't find ASN.mmdb, start download", "ASN invalid, remove and download"},
	} {
		output := validationErr.Error()
		if !strings.Contains(output, resource.failure) && !strings.Contains(output, resource.starting) && !strings.Contains(output, resource.invalid) {
			continue
		}
		asset = resource.asset
		if raw := cfg.URLs[resource.key]; raw != "" {
			var supported bool
			asset, supported = officialGeoDataAsset(raw)
			if !supported {
				return "", "", false
			}
		}
		file := resource.file
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() && (strings.EqualFold(name, file) || resource.key == "mmdb" && (strings.EqualFold(name, "geoip.metadb") || strings.EqualFold(name, "geoip.db"))) {
				file = name
				break
			}
		}
		return asset, filepath.Join(dir, file), true
	}
	return "", "", false
}

// Preserve explicit custom providers; only known official distributions use our gateway chain.
func officialGeoDataAsset(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	prefix := ""
	switch {
	case u.Host == "github.com":
		prefix = "/MetaCubeX/meta-rules-dat/releases/download/latest/"
	case strings.HasSuffix(u.Host, ".jsdelivr.net"):
		prefix = "/gh/MetaCubeX/meta-rules-dat@release/"
	default:
		return "", false
	}
	if !strings.HasPrefix(u.Path, prefix) {
		return "", false
	}
	name := strings.TrimPrefix(u.Path, prefix)
	return name, coreinfra.IsGeoDataAsset(name)
}
