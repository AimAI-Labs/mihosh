package core

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Retrieved over direct TLS from the official GitHub API on 2026-11-08.
// Update only from an independently authenticated official source, never a gateway.
//
//go:embed pinned-release.json
var pinnedReleaseJSON []byte

func PinnedRelease() Release {
	var r Release
	if err := json.Unmarshal(pinnedReleaseJSON, &r); err != nil {
		panic(err)
	}
	r.Trusted = true
	r.Route = "builtin"
	return r
}
func PinnedDigests() map[string]string {
	r := PinnedRelease()
	pins := make(map[string]string, len(r.Assets))
	for _, a := range r.Assets {
		pins[r.Tag+"/"+a.Name] = a.Digest
	}
	return pins
}

type Installation struct {
	BinarySHA256 string `json:"binary_sha256"`
	Version      string `json:"version"`
	Asset        string `json:"asset"`
	Source       string `json:"source"`
	Route        string `json:"route"`
	SHA256       string `json:"sha256"`
}

func BinaryName(goos string) string {
	if goos == "windows" {
		return "mihomo.exe"
	}
	return "mihomo"
}
func BinaryPath(root, tag, goos string) (string, error) {
	if !validTag.MatchString(tag) {
		return "", errors.New("invalid core version")
	}
	return filepath.Join(root, "core", "versions", tag, BinaryName(goos)), nil
}

// Install never executes the downloaded binary. Validation and activation belong
// to the service layer, after the verified archive is installed in its own directory.
func (f *Fetcher) Install(ctx context.Context, root string, r Release, goos, arch string) (Installation, error) {
	asset, err := r.SelectAsset(goos, arch)
	if err != nil {
		return Installation{}, err
	}
	digest, err := r.TrustedDigest(asset, PinnedDigests())
	if err != nil {
		return Installation{}, err
	}
	coreRoot := filepath.Join(root, "core")
	if err = os.MkdirAll(coreRoot, 0700); err != nil {
		return Installation{}, err
	}
	unlock, err := LockRuntime(filepath.Join(coreRoot, "install.lock"))
	if err != nil {
		return Installation{}, err
	}
	defer unlock()
	if err = cleanupInstallStages(coreRoot); err != nil {
		return Installation{}, err
	}
	target, err := BinaryPath(root, r.Version(), goos)
	if err != nil {
		return Installation{}, err
	}
	if _, err = os.Stat(filepath.Dir(target)); err == nil {
		data, e := os.ReadFile(filepath.Join(filepath.Dir(target), "installation.json"))
		var existing Installation
		if e == nil && json.Unmarshal(data, &existing) == nil && existing.Version == r.Version() && existing.Source == asset.URL && existing.SHA256 == digest && validDigest(existing.BinarySHA256) {
			got, e := fileDigest(target)
			if e == nil && got == existing.BinarySHA256 {
				return existing, nil
			}
		}
		return Installation{}, errors.New("installed version could not be verified; it was not replaced")
	}
	stage, err := os.MkdirTemp(coreRoot, ".install-")
	if err != nil {
		return Installation{}, err
	}
	defer func() { _ = os.RemoveAll(stage) }() // Stale stages are retried by cleanupInstallStages on the next operation.
	archive := filepath.Join(stage, asset.Name)
	route, err := f.Download(ctx, asset.URL, archive, digest)
	if err != nil {
		return Installation{}, err
	}
	if err = ctx.Err(); err != nil {
		return Installation{}, err
	}
	out := filepath.Join(stage, BinaryName(goos))
	if err = extractBinary(ctx, archive, out, goos); err != nil {
		return Installation{}, err
	}
	if err = writeUpstreamNotice(stage, r.Tag); err != nil {
		return Installation{}, err
	}
	if err = os.Remove(archive); err != nil {
		return Installation{}, err
	}
	binaryDigest, err := fileDigest(out)
	if err != nil {
		return Installation{}, err
	}
	result := Installation{Version: r.Version(), Asset: asset.Name, Source: asset.URL, Route: route.Name, SHA256: digest, BinarySHA256: binaryDigest}
	data, _ := json.MarshalIndent(result, "", "  ")
	if err = os.WriteFile(filepath.Join(stage, "installation.json"), data, 0600); err != nil {
		return Installation{}, err
	}
	if err = os.MkdirAll(filepath.Dir(filepath.Dir(target)), 0700); err != nil {
		return Installation{}, err
	}
	if err = ctx.Err(); err != nil {
		return Installation{}, err
	}
	if err = os.Rename(stage, filepath.Dir(target)); err != nil {
		return Installation{}, err
	}
	return result, nil
}

func extractBinary(ctx context.Context, archive, destination, goos string) error {
	var reader io.ReadCloser
	if goos == "windows" {
		z, err := zip.OpenReader(archive)
		if err != nil {
			return errors.New("invalid core ZIP archive")
		}
		defer func() { _ = z.Close() }()
		for _, entry := range z.File {
			if entry.FileInfo().IsDir() {
				continue
			}
			if entry.Mode()&os.ModeSymlink != 0 || strings.ContainsAny(entry.Name, "/\\") || !strings.HasSuffix(strings.ToLower(entry.Name), ".exe") || reader != nil {
				if reader != nil {
					_ = reader.Close()
				}
				return errors.New("unexpected ZIP archive entry")
			}
			if entry.UncompressedSize64 > uint64(MaxAssetBytes) {
				return errors.New("core executable exceeds size limit")
			}
			reader, err = entry.Open()
			if err != nil {
				return err
			}
		}
		if reader == nil {
			return errors.New("core executable missing from ZIP")
		}
	} else {
		file, err := os.Open(archive)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		gz, err := gzip.NewReader(file)
		if err != nil {
			return errors.New("invalid core GZIP archive")
		}
		reader = gz
	}
	defer func() { _ = reader.Close() }()
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }() // The successful output close is checked below.
	n, err := io.Copy(file, io.LimitReader(&contextReader{ctx, reader}, MaxAssetBytes+1))
	if err != nil {
		return errors.New("core extraction interrupted")
	}
	if n == 0 || n > MaxAssetBytes {
		return errors.New("invalid core executable size")
	}
	return file.Close()
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(b)
}

// Import copies a local executable only after matching a caller-provided checksum.
// The caller owns the decision about the independently verified checksum's source.
func Import(ctx context.Context, root, source, tag, expected, goos string) (Installation, error) {
	if strings.Contains(source, "://") {
		return Installation{}, errors.New("choose a local core executable, not a subscription or download URL")
	}
	if !validDigest(expected) {
		return Installation{}, errors.New("a verified SHA-256 is required for local import")
	}
	target, err := BinaryPath(root, tag, goos)
	if err != nil {
		return Installation{}, err
	}
	if err = os.MkdirAll(filepath.Dir(filepath.Dir(target)), 0700); err != nil {
		return Installation{}, err
	}
	unlock, err := LockRuntime(filepath.Join(root, "core", "install.lock"))
	if err != nil {
		return Installation{}, err
	}
	defer unlock()
	if err = cleanupInstallStages(filepath.Join(root, "core")); err != nil {
		return Installation{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(filepath.Dir(target)), ".import-")
	if err != nil {
		return Installation{}, err
	}
	defer func() { _ = os.RemoveAll(stage) }() // Stale stages are retried by cleanupInstallStages on the next operation.
	src, err := os.Open(source)
	if err != nil {
		return Installation{}, err
	}
	defer func() { _ = src.Close() }()
	out, err := os.OpenFile(filepath.Join(stage, BinaryName(goos)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return Installation{}, err
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(&contextReader{ctx, src}, MaxAssetBytes+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || n == 0 || n > MaxAssetBytes {
		return Installation{}, errors.New("local core copy failed")
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if digest != normalizeDigest(expected) {
		return Installation{}, errors.New("local core SHA-256 mismatch")
	}
	result := Installation{Version: tag, Asset: filepath.Base(source), Source: "local", Route: "local", SHA256: digest, BinarySHA256: digest}
	data, _ := json.MarshalIndent(result, "", "  ")
	if err = os.WriteFile(filepath.Join(stage, "installation.json"), data, 0600); err != nil {
		return Installation{}, err
	}
	if _, err = os.Stat(filepath.Dir(target)); err == nil {
		return Installation{}, errors.New("core version already installed")
	}
	if err = writeUpstreamNotice(stage, tag); err != nil {
		return Installation{}, err
	}
	if err = os.Rename(stage, filepath.Dir(target)); err != nil {
		return Installation{}, fmt.Errorf("commit local core: %w", err)
	}
	return result, nil
}

func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(file, MaxAssetBytes+1))
	if err != nil {
		return "", err
	}
	if n == 0 || n > MaxAssetBytes {
		return "", errors.New("invalid executable size")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeUpstreamNotice(directory, version string) error {
	notice := fmt.Sprintf("Mihomo upstream version: %s\nOfficial releases: https://github.com/MetaCubeX/mihomo/releases/tag/%s\nCorresponding upstream source: https://github.com/MetaCubeX/mihomo/tree/%s\nLicense: https://github.com/MetaCubeX/mihomo/blob/%s/LICENSE\nInstallation source and SHA-256 values are recorded in installation.json.\n", version, version, version, version)
	return os.WriteFile(filepath.Join(directory, "UPSTREAM.txt"), []byte(notice), 0600)
}

// Only reserved staging directories are removed, while the installation lock is held.
func cleanupInstallStages(coreRoot string) error {
	for _, dir := range []string{coreRoot, filepath.Join(coreRoot, "versions")} {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() && (strings.HasPrefix(entry.Name(), ".install-") || strings.HasPrefix(entry.Name(), ".import-")) {
				if err = os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
