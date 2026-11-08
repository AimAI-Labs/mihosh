package core

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const releaseAPI = "https://api.github.com/repos/MetaCubeX/mihomo/releases/"

var validTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

type Release struct {
	FetchedAt  time.Time `json:"fetched_at"`
	Tag        string    `json:"tag_name"`
	Prerelease bool      `json:"prerelease"`
	Draft      bool      `json:"draft"`
	Assets     []Asset   `json:"assets"`
	Route      string    `json:"route"`
	Trusted    bool      `json:"trusted"`
}

func (f *Fetcher) Release(ctx context.Context, tag string) (Release, error) {
	endpoint := releaseAPI + "latest"
	if tag != "" && tag != "latest" {
		if !validTag.MatchString(tag) {
			return Release{}, errors.New("invalid stable release tag")
		}
		endpoint = releaseAPI + "tags/" + url.PathEscape(tag)
	}
	var release Release
	_, route, err := f.Metadata(ctx, endpoint, func(data []byte) error {
		var r Release
		if err := json.Unmarshal(data, &r); err != nil {
			return errors.New("invalid release JSON")
		}
		if !validTag.MatchString(r.Tag) || r.Draft || r.Prerelease || len(r.Assets) == 0 {
			return errors.New("invalid stable release metadata")
		}
		if tag != "" && tag != "latest" && r.Tag != tag {
			return errors.New("release tag mismatch")
		}
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
	name := "mihomo-" + goos + "-" + platform + "-" + r.Tag + ext
	for _, a := range r.Assets {
		if a.Name == name {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("no compatible asset for %s/%s", goos, arch)
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
