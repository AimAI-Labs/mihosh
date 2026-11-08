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
				w.Write([]byte("executable"))
				w.Close()
			} else {
				w := zip.NewWriter(&b)
				entry, _ := w.Create("mihomo.exe")
				entry.Write([]byte("executable"))
				w.Close()
			}
			dir := t.TempDir()
			archive := filepath.Join(dir, "archive")
			os.WriteFile(archive, b.Bytes(), 0600)
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
			entry.Write([]byte("bad"))
		}
		w.Close()
		dir := t.TempDir()
		archive := filepath.Join(dir, "archive.zip")
		os.WriteFile(archive, b.Bytes(), 0600)
		if err := extractBinary(context.Background(), archive, filepath.Join(dir, "core.exe"), "windows"); err == nil {
			t.Fatalf("accepted %v", names)
		}
	}
}
func TestImportRequiresChecksumAndKeepsExistingVersion(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(t.TempDir(), "mihomo")
	os.WriteFile(src, []byte("local executable"), 0700)
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
	b.Write(bytes.Repeat([]byte("a"), 1<<20))
	b.Write([]byte("end"))
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
	gz.Write([]byte("verified executable"))
	gz.Close()
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
	os.MkdirAll(stale, 0700)
	os.WriteFile(filepath.Join(stale, "asset.part"), []byte("incomplete"), 0600)
	second, err := f.Install(context.Background(), root, r, "linux", "amd64")
	if _, e := os.Stat(stale); !os.IsNotExist(e) {
		t.Fatal("interrupted staging directory was retained")
	}
	if err != nil || first.BinarySHA256 != second.BinarySHA256 || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	binary, _ := BinaryPath(root, r.Tag, "linux")
	os.WriteFile(binary, []byte("tampered"), 0700)
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
