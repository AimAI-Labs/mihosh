package core

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractGzipAndZip(t *testing.T) {
	for _, platform := range []string{"linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			var b bytes.Buffer
			if platform == "linux" {
				w := gzip.NewWriter(&b)
				_, _ = w.Write([]byte("executable"))
				_ = w.Close()
			} else {
				w := zip.NewWriter(&b)
				entry, _ := w.Create("mihomo.exe")
				_, _ = entry.Write([]byte("executable"))
				_ = w.Close()
			}
			dir := t.TempDir()
			archive := filepath.Join(dir, "archive")
			if err := os.WriteFile(archive, b.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(dir, BinaryName(platform))
			if err := extractBinary(context.Background(), archive, destination, platform); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(destination)
			if string(got) != "executable" {
				t.Fatalf("got %q", got)
			}
		})
	}
}
func TestExtractRejectsZipTraversalAndMultipleExecutables(t *testing.T) {
	for _, names := range [][]string{{"../mihomo.exe"}, {"a.exe", "b.exe"}, {"link/mihomo.exe"}, {"page.html"}} {
		var b bytes.Buffer
		w := zip.NewWriter(&b)
		for _, name := range names {
			entry, _ := w.Create(name)
			_, _ = entry.Write([]byte("bad"))
		}
		_ = w.Close()
		dir := t.TempDir()
		archive := filepath.Join(dir, "archive.zip")
		if err := os.WriteFile(archive, b.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		if err := extractBinary(context.Background(), archive, filepath.Join(dir, "core.exe"), "windows"); err == nil {
			t.Fatalf("accepted %v", names)
		}
	}
}
func TestImportRequiresChecksumAndKeepsExistingVersion(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(t.TempDir(), "mihomo")
	if err := os.WriteFile(src, []byte("local executable"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(context.Background(), root, src, "../../bad", strings.Repeat("0", 64), "linux"); err == nil {
		t.Fatal("accepted traversal tag")
	}
	if _, err := Import(context.Background(), root, src, "v1.2.3", strings.Repeat("0", 64), "linux"); err == nil {
		t.Fatal("accepted wrong checksum")
	}
	hash := sha256.Sum256([]byte("local executable"))
	digest := hex.EncodeToString(hash[:])
	if _, err := Import(context.Background(), root, src, "v1.2.3", digest, "linux"); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(context.Background(), root, src, "v1.2.3", digest, "linux"); err == nil {
		t.Fatal("replaced installed version")
	}
}
func TestLogBufferIsBounded(t *testing.T) {
	var b LogBuffer
	_, _ = b.Write(bytes.Repeat([]byte("a"), 1<<20))
	_, _ = b.Write([]byte("end"))
	if len(b.String()) != 64<<10 || !strings.HasSuffix(b.String(), "end") {
		t.Fatal("unbounded or missing tail")
	}
}
func TestPinnedReleaseHasBaselineAssetsAndDigests(t *testing.T) {
	r := PinnedRelease()
	for _, os := range []string{"linux", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			a, err := r.SelectAsset(os, arch)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = r.TrustedDigest(a, PinnedDigests()); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestInstallIsIdempotentButRejectsTamperedExecutable(t *testing.T) {
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	_, _ = gz.Write([]byte("verified executable"))
	_ = gz.Close()
	hash := sha256.Sum256(archive.Bytes())
	digest := hex.EncodeToString(hash[:])
	asset := Asset{Name: "mihomo-linux-amd64-compatible-v1.2.3.gz", URL: "https://github.com/MetaCubeX/mihomo/releases/download/v1.2.3/mihomo-linux-amd64-compatible-v1.2.3.gz", Digest: "sha256:" + digest}
	r := Release{Tag: "v1.2.3", Assets: []Asset{asset}, Trusted: true}
	calls := 0
	f := testFetcher(t, func(Route, *http.Request) (int, string, error) { calls++; return 200, archive.String(), nil })
	root := t.TempDir()
	first, err := f.Install(context.Background(), root, r, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, "core", ".install-interrupted")
	if err := os.MkdirAll(stale, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "asset.part"), []byte("incomplete"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := f.Install(context.Background(), root, r, "linux", "amd64")
	if _, e := os.Stat(stale); !os.IsNotExist(e) {
		t.Fatal("interrupted staging directory was retained")
	}
	if err != nil || first.BinarySHA256 != second.BinarySHA256 || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	binary, _ := BinaryPath(root, r.Tag, "linux")
	if err := os.WriteFile(binary, []byte("tampered"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Install(context.Background(), root, r, "linux", "amd64"); err == nil {
		t.Fatal("tampered installed executable accepted")
	}
	data, _ := os.ReadFile(binary)
	if string(data) != "tampered" || calls != 1 {
		t.Fatal("existing executable was silently replaced")
	}
}
func TestRuntimeLockReleasesWithoutDeletingLockFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "process.lock")
	release, err := LockRuntime(path)
	if err != nil {
		t.Fatal(err)
	}
	if release2, err := LockRuntime(path); err == nil {
		release2()
		release()
		t.Fatal("second owner acquired lock")
	}
	release()
	release, err = LockRuntime(path)
	if err != nil {
		t.Fatal(err)
	}
	release()
}
