package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"golang.org/x/mod/semver"
)

const (
	githubAPIURL  = "https://api.github.com/repos/AimAI-Labs/mihosh/releases/latest"
	releaseBase   = "https://github.com/AimAI-Labs/mihosh/releases/"
	maxUpdateSize = 128 << 20
)

// githubRelease represents the JSON structure of a GitHub release.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Digest             string `json:"digest"`
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode json failed: %w", err)
	}

	if release.TagName == "" {
		return nil, nil
	}

	// 正式构建（通过 -ldflags 注入了 semver 版本号）才做版本比对；
	// dev/unknown/空 = 本地开发构建，始终展示最新 release 以便测试。
	isDev := currentVersion == "" || currentVersion == "dev" || currentVersion == "unknown"
	if !isDev {
		v1 := currentVersion
		if !strings.HasPrefix(v1, "v") {
			v1 = "v" + v1
		}
		v2 := release.TagName
		if !strings.HasPrefix(v2, "v") {
			v2 = "v" + v2
		}
		// 仅当远程版本严格大于本地版本时才提示更新
		if !semver.IsValid(v1) || !semver.IsValid(v2) || semver.Compare(v1, v2) >= 0 {
			return nil, nil
		}
	}

	return release.updateInfo(currentVersion, runtime.GOOS, runtime.GOARCH), nil
}

func (r githubRelease) updateInfo(currentVersion, goos, goarch string) *model.UpdateInfo {
	tag := url.PathEscape(r.TagName)
	info := &model.UpdateInfo{
		CurrentVersion: currentVersion,
		Version:        r.TagName,
		ReleaseURL:     releaseBase + "tag/" + tag,
	}
	if goos != "linux" && goos != "darwin" {
		return info
	}
	// 与 release.yml 的 tar 打包步骤使用相同的文件名和归档成员名。
	name := fmt.Sprintf("mihosh-%s-%s.tar.gz", goos, goarch)
	downloadURL := releaseBase + "download/" + tag + "/" + name
	for _, asset := range r.Assets {
		digest, ok := strings.CutPrefix(asset.Digest, "sha256:")
		if asset.Name == name && asset.BrowserDownloadURL == downloadURL && ok && len(decodeUpdateDigest(digest)) == sha256.Size {
			info.DownloadURL = downloadURL
			info.DownloadSHA256 = digest
			break
		}
	}
	return info
}

func decodeUpdateDigest(digest string) []byte {
	if len(digest) != sha256.Size*2 {
		return nil
	}
	decoded, err := hex.DecodeString(digest)
	if err != nil {
		return nil
	}
	return decoded
}

func isTrustedUpdateURL(raw string) bool {
	if !strings.HasPrefix(raw, releaseBase+"download/") {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "github.com" && u.User == nil
}

// DownloadAndApplyUpdate verifies the release archive before replacing the running executable.
func DownloadAndApplyUpdate(info model.UpdateInfo) error {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return errors.New("automatic updates are unavailable on this platform")
	}
	if !isTrustedUpdateURL(info.DownloadURL) {
		return errors.New("untrusted update download URL")
	}
	target, err := os.Executable()
	if err != nil {
		return err
	}
	target, err = filepath.EvalSymlinks(target)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	return applyReleaseUpdate(client, info, target)
}

func applyReleaseUpdate(client *http.Client, info model.UpdateInfo, target string) error {
	digest := decodeUpdateDigest(info.DownloadSHA256)
	if info.DownloadURL == "" || digest == nil {
		return errors.New("no verified update archive available")
	}

	resp, err := client.Get(info.DownloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code during download: %d", resp.StatusCode)
	}

	archive, err := os.CreateTemp("", "mihosh-update-*.tar.gz")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(archive.Name()) }()
	defer func() { _ = archive.Close() }()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(archive, hash), io.LimitReader(resp.Body, maxUpdateSize+1))
	if err != nil {
		return fmt.Errorf("read update archive: %w", err)
	}
	if size > maxUpdateSize {
		return errors.New("update archive exceeds size limit")
	}
	if !bytes.Equal(hash.Sum(nil), digest) {
		return errors.New("update archive SHA-256 mismatch")
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return replaceFromUpdateArchive(archive, target)
}

func replaceFromUpdateArchive(archive io.Reader, target string) error {
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("open update archive: %w", err)
	}
	defer func() { _ = gz.Close() }()
	uncompressed := &io.LimitedReader{R: gz, N: maxUpdateSize + (64 << 10)}
	contents := tar.NewReader(uncompressed)
	header, err := contents.Next()
	if err != nil || header.Name != "mihosh" || !header.FileInfo().Mode().IsRegular() || header.Size <= 0 || header.Size > maxUpdateSize {
		return errors.New("update archive must contain the mihosh executable")
	}
	current, err := os.Stat(target)
	if err != nil {
		return err
	}
	staged, err := os.CreateTemp(filepath.Dir(target), ".mihosh-update-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(staged.Name()) }()
	defer func() { _ = staged.Close() }()
	if _, err := io.Copy(staged, contents); err != nil {
		return fmt.Errorf("extract update executable: %w", err)
	}
	if _, err := contents.Next(); err != io.EOF {
		return errors.New("unexpected data in update archive")
	}
	// Read through the gzip trailer before committing the replacement.
	if _, err := io.Copy(io.Discard, uncompressed); err != nil || uncompressed.N == 0 {
		return errors.New("incomplete or oversized gzip stream in update archive")
	}
	if err := staged.Chmod(current.Mode().Perm()); err != nil {
		return err
	}
	if err := staged.Sync(); err != nil {
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	return os.Rename(staged.Name(), target)
}
