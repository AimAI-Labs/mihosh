package service

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

func TestUpdateSelectsExactVerifiedReleaseAsset(t *testing.T) {
	digest := strings.Repeat("a", 64)
	const asset = releaseBase + "download/v2/mihosh-linux-amd64.tar.gz"
	page := releaseBase + "tag/v2"
	for _, tc := range []struct {
		name, filename, url, digest, download string
	}{
		{"verified", "mihosh-linux-amd64.tar.gz", asset, "sha256:" + digest, asset},
		{"wrong platform", "mihosh-linux-arm64.tar.gz", asset, "sha256:" + digest, ""},
		{"prefix match", "mihosh-linux-amd64.tar.gz.bak", asset, "sha256:" + digest, ""},
		{"missing digest", "mihosh-linux-amd64.tar.gz", asset, "", ""},
		{"release page", "mihosh-linux-amd64.tar.gz", page, "sha256:" + digest, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var release githubRelease
			if err := json.Unmarshal(fmt.Appendf(nil, `{"tag_name":"v2","assets":[{"name":%q,"browser_download_url":%q,"digest":%q}]}`, tc.filename, tc.url, tc.digest), &release); err != nil {
				t.Fatal(err)
			}
			info := release.updateInfo("v1", "linux", "amd64", "")
			if info.ReleaseURL != page || info.DownloadURL != tc.download {
				t.Fatalf("unexpected update info: %+v", info)
			}
			if tc.download != "" && info.DownloadSHA256 != digest {
				t.Fatalf("unexpected update digest: %s", info.DownloadSHA256)
			}
		})
	}
}

func TestApplyReleaseUpdatePreservesRunnableTargetOnFailure(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("release workflow packages Unix binaries")
	}
	dir := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(dir, "mihosh")
	wantVersion := "new version"
	if path := os.Getenv("MIHOSH_UPDATE_TEST_BINARY"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		write(binary, string(data))
		out, err := exec.Command(binary, "version").CombinedOutput()
		if err != nil || len(out) == 0 {
			t.Fatalf("new executable version: %s (%v)", out, err)
		}
		wantVersion = strings.TrimSpace(string(out))
	} else {
		write(binary, "#!/bin/sh\necho new version\n")
	}
	archivePath := filepath.Join(dir, "mihosh.tar.gz")
	// The release workflow packages the root-level mihosh executable with tar.
	pack := exec.Command("tar", "-czf", archivePath, "mihosh")
	pack.Dir = dir
	if out, err := pack.CombinedOutput(); err != nil {
		t.Fatalf("package binary: %s (%v)", out, err)
	}
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	truncated := archive[:len(archive)-4]
	archiveDigest := fmt.Sprintf("%x", sha256.Sum256(archive))
	for _, tc := range []struct {
		name, digest, version, wantError string
		archive                          []byte
		skipService                      bool
	}{
		{"success", archiveDigest, wantVersion, "", archive, false},
		{"success without service", archiveDigest, wantVersion, "", archive, true},
		{"missing asset", "", "old version", "no verified update archive", nil, false},
		{"wrong digest", strings.Repeat("0", 64), "old version", "SHA-256 mismatch", archive, false},
		{"truncated gzip", fmt.Sprintf("%x", sha256.Sum256(truncated)), "old version", "incomplete or oversized gzip", truncated, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("MIHOSH_CONFIG_DIR", root)
			target := filepath.Join(root, "installed-mihosh")
			write(target, "#!/bin/sh\necho old version\n")
			serviceBinary := filepath.Join(root, "bin", "mihosh")
			hasService := runtime.GOOS == "linux" && !tc.skipService
			if hasService {
				if err := os.Mkdir(filepath.Dir(serviceBinary), 0700); err != nil {
					t.Fatal(err)
				}
				write(serviceBinary, "#!/bin/sh\necho old version\n")
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.archive == nil {
					t.Error("manual update must not download a release page")
				}
				_, _ = w.Write(tc.archive)
			}))
			defer srv.Close()
			info := model.UpdateInfo{ReleaseURL: srv.URL, DownloadSHA256: tc.digest}
			if tc.archive != nil {
				info.DownloadURL = srv.URL
			}
			updatedService, err := applyReleaseUpdate(srv.Client(), info, target)
			if tc.wantError == "" && err != nil || tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("apply error = %v, want %q", err, tc.wantError)
			}
			wantService := ""
			if hasService && tc.wantError == "" {
				wantService = serviceBinary
			}
			if updatedService != wantService {
				t.Fatalf("updated service path = %q, want %q", updatedService, wantService)
			}
			out, err := exec.Command(target, "version").CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != tc.version {
				t.Fatalf("installed executable version: %s (%v), want %q", out, err, tc.version)
			}
			if hasService {
				out, err := exec.Command(serviceBinary, "version").CombinedOutput()
				if err != nil || strings.TrimSpace(string(out)) != tc.version {
					t.Fatalf("service executable version: %s (%v), want %q", out, err, tc.version)
				}
				if tc.wantError == "" {
					stat, err := os.Stat(serviceBinary)
					if err != nil || stat.Mode().Perm() != 0700 {
						t.Fatalf("service executable mode: %v (%v), want 0700", stat, err)
					}
				}
			}
		})
	}
}

func TestIsTrustedUpdateURL(t *testing.T) {
	if !isTrustedUpdateURL(releaseBase + "download/v2.0.0/mihosh-linux-amd64.tar.gz") {
		t.Fatal("expected official release download URL to be trusted")
	}
	for _, bad := range []string{
		"",
		"http://github.com/AimAI-Labs/mihosh/releases/download/v2/mihosh.tar.gz",
		"https://evil.example/AimAI-Labs/mihosh/releases/download/v2/mihosh.tar.gz",
		releaseBase + "tag/v2.0.0",
	} {
		if isTrustedUpdateURL(bad) {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}

func TestApplyReleaseUpdateRejectsUnavailableTargetsBeforeDownload(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("release workflow packages Unix binaries")
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer server.Close()
	info := model.UpdateInfo{
		DownloadURL:    server.URL,
		DownloadSHA256: strings.Repeat("a", 64),
	}
	t.Run("Homebrew symlink", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "Cellar", "mihosh", "2.0.0", "bin")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dir, "mihosh")
		if err := os.WriteFile(target, []byte("current"), 0755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "mihosh")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		var release githubRelease
		const filename = "mihosh-linux-amd64.tar.gz"
		data := fmt.Appendf(nil, `{"tag_name":"v2","assets":[{"name":%q,"browser_download_url":%q,"digest":%q}]}`,
			filename, releaseBase+"download/v2/"+filename, "sha256:"+info.DownloadSHA256)
		if err := json.Unmarshal(data, &release); err != nil {
			t.Fatal(err)
		}
		update := release.updateInfo("v1", "linux", "amd64", link)
		if !update.HomebrewManaged || update.DownloadURL != "" || update.DownloadSHA256 != "" || update.ReleaseURL != releaseBase+"tag/v2" {
			t.Fatalf("expected manual Homebrew update info, got %+v", update)
		}
		before := requests.Load()
		_, err := applyReleaseUpdate(server.Client(), info, link)
		if err == nil || !strings.Contains(err.Error(), "brew upgrade mihosh") || requests.Load() != before {
			t.Fatalf("expected Homebrew guidance before download, got %v and %d requests", err, requests.Load()-before)
		}
	})
	t.Run("read-only directory", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write into directories without write permission")
		}
		dir := t.TempDir()
		target := filepath.Join(dir, "mihosh")
		if err := os.WriteFile(target, []byte("current"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(dir, 0700); err != nil {
				t.Error(err)
			}
		})
		before := requests.Load()
		_, err := applyReleaseUpdate(server.Client(), info, target)
		if !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "rerun the installer") {
			t.Fatalf("expected install permissions error, got %v", err)
		}
		if requests.Load() != before {
			t.Fatal("read-only target must be rejected before download")
		}
	})
}

func TestInstallUpdatedExecutablesRestoresServiceOnPrimaryFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("background service is only supported on Linux")
	}
	root := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	serviceBinary := filepath.Join(root, "bin", "mihosh")
	if err := os.Mkdir(filepath.Dir(serviceBinary), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serviceBinary, []byte("old"), 0700); err != nil {
		t.Fatal(err)
	}
	old, err := os.Stat(serviceBinary)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := os.CreateTemp(root, "staged-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = staged.Close() }()
	if _, err := staged.WriteString("new"); err != nil {
		t.Fatal(err)
	}
	path, err := installUpdatedExecutables(staged, filepath.Join(root, "missing", "mihosh"))
	if path != "" || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected primary install failure, got %q and %v", path, err)
	}
	current, err := os.Stat(serviceBinary)
	if err != nil || !os.SameFile(old, current) {
		t.Fatalf("expected original service file after rollback, got %v and %v", current, err)
	}
}
