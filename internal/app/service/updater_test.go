package service

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
			info := release.updateInfo("v1", "linux", "amd64")
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
	}{
		{"success", archiveDigest, wantVersion, "", archive},
		{"missing asset", "", "old version", "no verified update archive", nil},
		{"wrong digest", strings.Repeat("0", 64), "old version", "SHA-256 mismatch", archive},
		{"truncated gzip", fmt.Sprintf("%x", sha256.Sum256(truncated)), "old version", "incomplete or oversized gzip", truncated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "installed-mihosh")
			write(target, "#!/bin/sh\necho old version\n")
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
			err := applyReleaseUpdate(srv.Client(), info, target)
			if tc.wantError == "" && err != nil || tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("apply error = %v, want %q", err, tc.wantError)
			}
			out, err := exec.Command(target, "version").CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != tc.version {
				t.Fatalf("installed executable version: %s (%v), want %q", out, err, tc.version)
			}
		})
	}
}
