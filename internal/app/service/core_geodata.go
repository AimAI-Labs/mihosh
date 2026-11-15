package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"gopkg.in/yaml.v3"
)

type GeoDataFile struct {
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Asset     string    `json:"asset"`
	Exists    bool      `json:"exists"`
	Updatable bool      `json:"updatable"`
	Size      int64     `json:"size"`
	UpdatedAt time.Time `json:"updated_at"`
}

var geoDataResources = []struct{ key, file, asset, failure, starting, invalid string }{
	{"mmdb", "Country.mmdb", "country.mmdb", "can't download MMDB:", "Can't find MMDB, start download", "MMDB invalid, remove and download"},
	{"geoip", "GeoIP.dat", "geoip.dat", "can't download GeoIP.dat:", "Can't find GeoIP.dat, start download", "GeoIP.dat invalid, remove and download"},
	{"geosite", "GeoSite.dat", "geosite.dat", "can't download GeoSite.dat:", "Can't find GeoSite.dat, start download", "GeoSite.dat invalid, remove and download"},
	{"asn", "ASN.mmdb", "GeoLite2-ASN.mmdb", "can't download ASN", "Can't find ASN.mmdb, start download", "ASN invalid, remove and download"},
}

func (s *CoreService) GeoDataFiles() ([]GeoDataFile, error) {
	cfg, err := loadCoreConfig()
	if err != nil {
		return nil, err
	}
	return managedGeoDataFiles(cfg)
}

func (s *CoreService) UpdateGeoData(ctx context.Context, key string, notify func(coreinfra.Progress)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.mu.TryLock() {
		return errors.New("another core operation is active")
	}
	defer s.mu.Unlock()
	ctx, cancel := s.operationContext(ctx)
	defer cancel()
	if err := s.lifetime.Err(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cfg, err := loadCoreConfig()
	if err != nil {
		return err
	}
	files, err := managedGeoDataFiles(cfg)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.Key != key {
			continue
		}
		if !file.Updatable {
			return errors.New("custom GEO providers cannot be updated through the official downloader")
		}
		if err := os.MkdirAll(filepath.Dir(file.Path), 0700); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		fetcher := coreFetcher(cfg, notify)
		fetcher.RunningProxy = managedDownloadProxy(cfg)
		return fetcher.DownloadGeoData(ctx, file.Asset, file.Path)
	}
	return fmt.Errorf("unknown GEO resource: %s", key)
}

func managedGeoDataFiles(cfg *config.Config) ([]GeoDataFile, error) {
	if cfg.Core.Mode != "managed" {
		return nil, errors.New("GEO file management requires managed core mode")
	}
	path, err := config.ManagedMihomoConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	files, err := geoDataTargets(data, filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	for i := range files {
		info, err := os.Stat(files[i].Path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("GEO resource is not a regular file: %s", files[i].Name)
		}
		files[i].Exists = true
		files[i].Size = info.Size()
		files[i].UpdatedAt = info.ModTime()
	}
	return files, nil
}

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
	files, err := geoDataTargets(data, dir)
	if err != nil {
		return "", "", false
	}
	for i, resource := range geoDataResources {
		output := validationErr.Error()
		if strings.Contains(output, resource.failure) || strings.Contains(output, resource.starting) || strings.Contains(output, resource.invalid) {
			return files[i].Asset, files[i].Path, files[i].Updatable
		}
	}
	return "", "", false
}

func geoDataTargets(data []byte, dir string) ([]GeoDataFile, error) {
	var cfg struct {
		URLs map[string]string `yaml:"geox-url"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	files := make([]GeoDataFile, 0, len(geoDataResources))
	for _, resource := range geoDataResources {
		asset, updatable := resource.asset, true
		if raw := cfg.URLs[resource.key]; raw != "" {
			asset, updatable = officialGeoDataAsset(raw)
		}
		file := resource.file
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() && (strings.EqualFold(name, file) || resource.key == "mmdb" && (strings.EqualFold(name, "geoip.metadb") || strings.EqualFold(name, "geoip.db"))) {
				file = name
				break
			}
		}
		files = append(files, GeoDataFile{Key: resource.key, Name: file, Path: filepath.Join(dir, file), Asset: asset, Updatable: updatable})
	}
	return files, nil
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
