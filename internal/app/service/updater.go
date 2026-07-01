package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/minio/selfupdate"
)

const (
	githubAPIURL = "https://api.github.com/repos/AimAI-Labs/mihosh/releases/latest"
)

// githubRelease represents the JSON structure of a GitHub release.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckUpdate checks the GitHub API for the latest release.
// It returns an UpdateInfo if a newer version is found, or nil if no update is needed or on error.
func CheckUpdate(currentVersion string) (*model.UpdateInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", githubAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode json failed: %w", err)
	}

	// Simple version compare: if tag is different and not empty
	// (A proper semver compare could be used here, but string inequality is a good start)
	if release.TagName == "" || release.TagName == currentVersion {
		return nil, nil // No update
	}

	// Find the correct asset for the current OS and Arch
	expectedPrefix := fmt.Sprintf("mihosh-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		expectedPrefix += ".exe"
	}
	// Note: We might also want to look for .tar.gz or .zip if the binary is compressed.
	// For now, assume it's just the binary.

	var downloadURL string
	for _, asset := range release.Assets {
		if strings.HasPrefix(asset.Name, expectedPrefix) {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	// If we can't find a direct binary, we can still show the update but without the download URL,
	// or we just rely on users downloading it manually. Here we assume we must find an asset.
	if downloadURL == "" {
		// Fallback: just return the release page as download URL
		downloadURL = fmt.Sprintf("https://github.com/AimAI-Labs/mihosh/releases/tag/%s", release.TagName)
	}

	return &model.UpdateInfo{
		Version:      release.TagName,
		ReleaseNotes: release.Body,
		DownloadURL:  downloadURL,
	}, nil
}

// DownloadAndApplyUpdate downloads the binary from the given URL and replaces the current executable.
func DownloadAndApplyUpdate(url string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code during download: %d", resp.StatusCode)
	}

	err = selfupdate.Apply(resp.Body, selfupdate.Options{})
	if err != nil {
		return fmt.Errorf("apply update failed: %w", err)
	}

	return nil
}
