package core

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const releaseAPI = "https://api.github.com/repos/MetaCubeX/mihomo/releases/"

var stableTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
var validTag = regexp.MustCompile(`^(v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?|(alpha|beta)-[0-9a-f]{7,40})$`)

func validReleaseTag(tag string) bool {
	return validTag.MatchString(tag) || tag == "Prerelease-Alpha" || tag == "Prerelease-Beta"
}

type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

type Release struct {
	PublishedAt time.Time `json:"published_at"`
	FetchedAt   time.Time `json:"fetched_at"`
	Tag         string    `json:"tag_name"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	Assets      []Asset   `json:"assets"`
	Route       string    `json:"route"`
	Trusted     bool      `json:"trusted"`
}

func (f *Fetcher) Release(ctx context.Context, tag string) (Release, error) {
	endpoint := releaseAPI + "latest"
	if tag != "" && tag != "latest" {
		if !validReleaseTag(tag) {
			return Release{}, errors.New("invalid release tag")
		}
		endpoint = releaseAPI + "tags/" + url.PathEscape(tag)
	}
	var release Release
	_, route, err := f.Metadata(ctx, endpoint, func(data []byte) error {
		var r Release
		if err := json.Unmarshal(data, &r); err != nil {
			return errors.New("invalid release JSON")
		}
		if !validReleaseTag(r.Tag) || r.Draft || len(r.Assets) == 0 || ((tag == "" || tag == "latest") && (r.Prerelease || !stableTag.MatchString(r.Tag))) {
			return errors.New("invalid release metadata")
		}
		if tag != "" && tag != "latest" && r.Tag != tag {
			return errors.New("release tag mismatch")
		}
		if err := validateReleaseAssets(r); err != nil {
			return err
		}
		release = r
		return nil
	})
	if err != nil {
		return Release{}, err
	}
	release.FetchedAt = time.Now().UTC()
	release.Route = route.Name
	release.Trusted = route.Trusted()
	return release, nil
}

// validateReleaseAssets is shared by single-release lookups and the release browser.
func validateReleaseAssets(r Release) error {
	seen := make(map[string]bool)
	for _, a := range r.Assets {
		if a.Name == "" || strings.ContainsAny(a.Name, "/\\") || seen[a.Name] || a.Size < 0 {
			return errors.New("invalid release asset")
		}
		if err := validateSource(a.URL); err != nil {
			return err
		}
		u, _ := url.Parse(a.URL)
		if u.Host != "github.com" || u.Path != "/MetaCubeX/mihomo/releases/download/"+r.Tag+"/"+a.Name {
			return errors.New("release asset URL mismatch")
		}
		seen[a.Name] = true
	}
	return nil
}

const ReleasePageSize = 10

// ReleasePage retains only assets for the requested platform, keeping each page bounded.
// HasNext follows the unfiltered GitHub page, so incompatible entries do not
// prematurely terminate browsing.
type ReleasePage struct {
	Page     int       `json:"page"`
	HasNext  bool      `json:"has_next"`
	Releases []Release `json:"releases"`
}

func (f *Fetcher) Releases(ctx context.Context, page int, goos, arch string) (ReleasePage, error) {
	if page < 1 {
		return ReleasePage{}, errors.New("invalid release page")
	}
	endpoint := strings.TrimSuffix(releaseAPI, "/") + "?per_page=" + strconv.Itoa(ReleasePageSize) + "&page=" + strconv.Itoa(page)
	var result ReleasePage
	_, route, err := f.Metadata(ctx, endpoint, func(data []byte) error {
		var releases []Release
		if err := json.Unmarshal(data, &releases); err != nil || releases == nil || len(releases) > ReleasePageSize {
			return errors.New("invalid release list JSON")
		}
		candidate := ReleasePage{Page: page, HasNext: len(releases) == ReleasePageSize, Releases: []Release{}}
		seen := make(map[string]bool)
		for _, r := range releases {
			if r.Draft || !validReleaseTag(r.Tag) {
				continue
			}
			if len(r.Assets) == 0 {
				continue
			}
			if err := validateReleaseAssets(r); err != nil {
				return err
			}
			if seen[r.Tag] {
				return errors.New("duplicate release tag")
			}
			seen[r.Tag] = true
			asset, err := r.SelectAsset(goos, arch)
			if err != nil {
				continue
			}
			r.Assets = []Asset{asset}
			candidate.Releases = append(candidate.Releases, r)
		}
		result = candidate
		return nil
	})
	if err != nil {
		return ReleasePage{}, err
	}
	for i := range result.Releases {
		result.Releases[i].FetchedAt = time.Now().UTC()
		result.Releases[i].Route = route.Name
		result.Releases[i].Trusted = route.Trusted()
	}
	return result, nil
}

// SelectAsset uses baseline amd64 builds, avoiding unverified CPU instructions.
// Unsupported platforms fail explicitly rather than guessing an executable.
func (r Release) SelectAsset(goos, arch string) (Asset, error) {
	if goos != "linux" && goos != "windows" {
		return Asset{}, fmt.Errorf("unsupported core platform: %s/%s", goos, arch)
	}
	platform := arch
	switch arch {
	case "amd64":
		platform = "amd64-compatible"
	case "arm64":
	default:
		return Asset{}, fmt.Errorf("unsupported core architecture: %s", arch)
	}
	ext := ".gz"
	if goos == "windows" {
		ext = ".zip"
	}
	platforms := []string{platform}
	if arch == "amd64" {
		platforms = append(platforms, "amd64-v1")
	}
	for _, platform := range platforms {
		prefix := "mihomo-" + goos + "-" + platform + "-"
		var match Asset
		for _, a := range r.Assets {
			if !strings.HasPrefix(a.Name, prefix) || !strings.HasSuffix(a.Name, ext) {
				continue
			}
			version := strings.TrimSuffix(strings.TrimPrefix(a.Name, prefix), ext)
			if !validTag.MatchString(version) {
				continue
			}
			channel := strings.TrimPrefix(r.Tag, "Prerelease-")
			if r.Tag != version && !(r.Prerelease && (channel == "Alpha" || channel == "Beta") && strings.HasPrefix(version, strings.ToLower(channel)+"-")) {
				continue
			}
			if match.Name != "" {
				return Asset{}, errors.New("ambiguous compatible release assets")
			}
			match = a
		}
		if match.Name != "" {
			return match, nil
		}
	}
	return Asset{}, fmt.Errorf("no compatible asset for %s/%s", goos, arch)
}

// Version identifies the executable build, including the commit of rolling releases.
// The upstream Prerelease-Alpha tag is reused; installation directories must not be.
func (r Release) Version() string {
	if validTag.MatchString(r.Tag) {
		return r.Tag
	}
	for _, goos := range []string{"linux", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			if asset, err := r.SelectAsset(goos, arch); err == nil {
				ext := ".gz"
				if goos == "windows" {
					ext = ".zip"
				}
				name := strings.TrimSuffix(asset.Name, ext)
				for _, platform := range []string{"amd64-compatible", "amd64-v1", "arm64"} {
					prefix := "mihomo-" + goos + "-" + platform + "-"
					if strings.HasPrefix(name, prefix) {
						return strings.TrimPrefix(name, prefix)
					}
				}
			}
		}
	}
	return r.Tag
}

func (r Release) TrustedDigest(asset Asset, pinned map[string]string) (string, error) {
	// A pin is keyed by immutable release tag and asset name, never the gateway URL.
	if digest := pinned[r.Tag+"/"+asset.Name]; digest != "" {
		if validDigest(digest) {
			return strings.TrimPrefix(digest, "sha256:"), nil
		}
		return "", errors.New("invalid pinned asset digest")
	}
	if r.Trusted && validDigest(asset.Digest) {
		return strings.TrimPrefix(asset.Digest, "sha256:"), nil
	}
	return "", errors.New("no independently trusted asset digest; use a verified SHA-256 or retry through direct/proxy access")
}

func validDigest(s string) bool {
	b, err := hex.DecodeString(strings.TrimPrefix(s, "sha256:"))
	return err == nil && len(b) == 32
}

type Status struct {
	BinaryPath       string   `json:"binary_path"`
	ConfigPath       string   `json:"config_path"`
	Controller       string   `json:"controller"`
	MixedPort        int      `json:"mixed_port"`
	Runtime          string   `json:"runtime"`
	ServiceInstalled bool     `json:"service_installed"`
	AutoStart        bool     `json:"auto_start"`
	SystemProxy      bool     `json:"system_proxy"`
	TUN              bool     `json:"tun"`
	RecoveryPending  bool     `json:"recovery_pending"`
	Mode             string   `json:"mode"`
	Version          string   `json:"version"`
	Installed        []string `json:"installed"`
	Running          bool     `json:"running"`
	HasNodes         bool     `json:"has_nodes"`
}
