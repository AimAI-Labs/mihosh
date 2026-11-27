package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/fsutil"
	"golang.org/x/mod/semver"
)

const (
	githubAPIURL  = "https://api.github.com/repos/AimAI-Labs/mihosh/releases/latest"
	releaseBase   = "https://github.com/AimAI-Labs/mihosh/releases/"
	maxUpdateSize = 128 << 20
)

// githubRelease represents the JSON structure of a GitHub release.
type githubRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Digest             string `json:"digest"`
	} `json:"assets"`
}

// CheckUpdate checks the GitHub API for the latest release.
// It returns an UpdateInfo if a newer version is found, or nil if no update is needed or on error.
func CheckUpdate(ctx context.Context, currentVersion string) (*model.UpdateInfo, error) {
	client := configuredUpdateHTTPClient(10 * time.Second)
	defer client.CloseIdleConnections()
	return checkUpdate(ctx, client, githubAPIURL, currentVersion)
}

func checkUpdate(ctx context.Context, client updateRequestClient, endpoint, currentVersion string) (*model.UpdateInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", safeUpdateRequestError(err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode json failed: %w", err)
	}

	latest := "v" + strings.TrimPrefix(release.TagName, "v")
	if release.Draft || release.Prerelease || !semver.IsValid(latest) || semver.Prerelease(latest) != "" {
		return nil, nil
	}

	// 正式构建（通过 -ldflags 注入了 semver 版本号）才做版本比对；
	// dev/unknown/空 = 本地开发构建，始终展示最新 release 以便测试。
	isDev := currentVersion == "" || currentVersion == "dev" || currentVersion == "unknown"
	if !isDev {
		current := "v" + strings.TrimPrefix(currentVersion, "v")
		// 仅当远程版本严格大于本地版本时才提示更新
		if !semver.IsValid(current) || semver.Compare(current, latest) >= 0 {
			return nil, nil
		}
	}

	executable, _ := os.Executable()
	return release.updateInfo(currentVersion, runtime.GOOS, runtime.GOARCH, executable), nil
}

func (r githubRelease) updateInfo(currentVersion, goos, goarch, executable string) *model.UpdateInfo {
	tag := url.PathEscape(r.TagName)
	info := &model.UpdateInfo{
		CurrentVersion: currentVersion,
		Version:        r.TagName,
		ReleaseURL:     releaseBase + "tag/" + tag,
	}
	if goos != "linux" && goos != "darwin" {
		return info
	}
	if target, err := filepath.EvalSymlinks(executable); err == nil && isHomebrewUpdateTarget(target) {
		info.HomebrewManaged = true
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
	relative, ok := strings.CutPrefix(raw, releaseBase+"download/")
	if !ok {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") ||
		u.EscapedPath() != u.Path || path.Clean(u.Path) != u.Path {
		return false
	}
	tag, asset, ok := strings.Cut(relative, "/")
	return ok && tag != "" && asset != "" && !strings.Contains(asset, "/")
}

func isHomebrewUpdateTarget(target string) bool {
	if strings.Contains(filepath.ToSlash(target), "/Cellar/mihosh/") {
		return true
	}
	cellar := os.Getenv("HOMEBREW_CELLAR")
	return filepath.IsAbs(cellar) && strings.HasPrefix(target, filepath.Join(cellar, "mihosh")+string(filepath.Separator))
}

// DownloadAndApplyUpdate verifies the release archive before replacing the running executable.
// It returns the Linux service executable path if the update also replaced it.
func DownloadAndApplyUpdate(ctx context.Context, info model.UpdateInfo) (string, error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return "", errors.New("automatic updates are unavailable on this platform")
	}
	if !isTrustedUpdateURL(info.DownloadURL) {
		return "", errors.New("untrusted update download URL")
	}
	target, err := os.Executable()
	if err != nil {
		return "", err
	}
	client := configuredUpdateHTTPClient(60 * time.Second)
	defer client.CloseIdleConnections()
	return applyReleaseUpdate(ctx, client, info, target)
}

func applyReleaseUpdate(ctx context.Context, client updateRequestClient, info model.UpdateInfo, target string) (string, error) {
	digest := decodeUpdateDigest(info.DownloadSHA256)
	if info.DownloadURL == "" || digest == nil {
		return "", errors.New("no verified update archive available")
	}
	target, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	if isHomebrewUpdateTarget(target) {
		return "", errors.New("use brew upgrade mihosh for Homebrew installations")
	}
	staged, err := os.CreateTemp(filepath.Dir(target), ".mihosh-update-*")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return "", fmt.Errorf("cannot write to the install directory. Please rerun the installer with the required permissions: %w", err)
		}
		return "", err
	}
	defer func() { _ = os.Remove(staged.Name()) }()
	defer func() { _ = staged.Close() }()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", safeUpdateRequestError(err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code during download: %d", resp.StatusCode)
	}

	archive, err := os.CreateTemp("", "mihosh-update-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(archive.Name()) }()
	defer func() { _ = archive.Close() }()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(archive, hash), io.LimitReader(resp.Body, maxUpdateSize+1))
	if err != nil {
		return "", fmt.Errorf("read update archive: %w", err)
	}
	if size > maxUpdateSize {
		return "", errors.New("update archive exceeds size limit")
	}
	if !bytes.Equal(hash.Sum(nil), digest) {
		return "", errors.New("update archive SHA-256 mismatch")
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if err := stageUpdateExecutable(archive, staged, target); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return installUpdatedExecutables(staged, target)
}

func stageUpdateExecutable(archive io.Reader, staged *os.File, target string) error {
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
	return staged.Sync()
}

func installUpdatedExecutables(staged *os.File, target string) (string, error) {
	replacePrimary := func() error {
		if err := staged.Close(); err != nil {
			return err
		}
		return os.Rename(staged.Name(), target)
	}
	if runtime.GOOS != "linux" {
		return "", replacePrimary()
	}
	root, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	serviceBinary := filepath.Join(root, "bin", "mihosh")
	if _, err := os.Lstat(serviceBinary); os.IsNotExist(err) {
		return "", replacePrimary()
	} else if err != nil {
		return "", err
	}
	runtimeDir := filepath.Join(root, "runtime")
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		return "", err
	}
	unlock, err := coreinfra.LockRuntime(filepath.Join(runtimeDir, "config.lock"))
	if err != nil {
		return "", err
	}
	defer unlock()
	resolvedService, err := filepath.EvalSymlinks(serviceBinary)
	if err != nil {
		return "", err
	}
	if resolvedService == target {
		if err := replacePrimary(); err != nil {
			return "", err
		}
		return serviceBinary, nil
	}
	if _, err := staged.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	// Keep the old inode, including any file capabilities, until both paths are replaced.
	backupDir, err := os.MkdirTemp(filepath.Dir(serviceBinary), ".mihosh-update-backup-*")
	if err != nil {
		return "", err
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			_ = os.RemoveAll(backupDir)
		}
	}()
	backup := filepath.Join(backupDir, "mihosh")
	if err := os.Link(serviceBinary, backup); err != nil {
		return "", err
	}
	if err := fsutil.WriteStreamAtomic(serviceBinary, fsutil.WriteOptions{Perm: 0700}, func(w io.Writer) error {
		_, err := io.Copy(w, staged)
		return err
	}); err != nil {
		return "", fmt.Errorf("update service executable: %w", err)
	}
	if err := replacePrimary(); err != nil {
		if restoreErr := os.Rename(backup, serviceBinary); restoreErr != nil {
			keepBackup = true
			return "", errors.Join(err, fmt.Errorf("restore service executable from %s: %w", backup, restoreErr))
		}
		return "", err
	}
	return serviceBinary, nil
}
