package sync

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"gopkg.in/yaml.v3"
)

const (
	// BundleSchemaVersion is the current sync archive schema version.
	BundleSchemaVersion = 1
	// MaxArchiveBytes bounds total uncompressed archive size on import.
	MaxArchiveBytes = 16 << 20
	// MaxArchiveFileBytes bounds any single file entry inside an archive.
	MaxArchiveFileBytes = 4 << 20
)

var (
	// ErrInvalidArchive indicates a malformed or unsafe archive file.
	ErrInvalidArchive = errors.New("invalid mihosh sync archive")
	// ErrUnsupportedSchema indicates an archive schema newer than this client supports.
	ErrUnsupportedSchema = errors.New("unsupported sync archive schema version")
)

const (
	entryManifest   = "manifest.json"
	entryConfig     = "config.yaml"
	entryMerge      = "merge.yaml"
	entryRuleEdits  = "rule-edits.yaml"
	entryChainEdits = "chain-edits.yaml"
)

// Manifest describes the metadata of a portable backup/sync bundle.
type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	AppVersion    string    `json:"app_version"`
	Device        string    `json:"device"`
	ExportedAt    time.Time `json:"exported_at"`
	Redacted      bool      `json:"redacted"`
	RedactedCount int       `json:"redacted_count"`
	Files         []string  `json:"files"`
}

// PortablePreferences captures cross-device Mihosh preferences while excluding
// host-specific paths, core binaries, and controller/database secrets.
type PortablePreferences struct {
	TestURL             string                   `yaml:"test_url,omitempty" json:"test_url,omitempty"`
	Timeout             int                      `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Language            string                   `yaml:"language,omitempty" json:"language,omitempty"`
	AutoRefreshInterval int                      `yaml:"auto_refresh_interval" json:"auto_refresh_interval"`
	Theme               string                   `yaml:"theme,omitempty" json:"theme,omitempty"`
	BorderStyle         string                   `yaml:"border_style,omitempty" json:"border_style,omitempty"`
	ShellProxy          bool                     `yaml:"shell_proxy,omitempty" json:"shell_proxy,omitempty"`
	FileDiffOptions     *profile.FileDiffOptions `yaml:"file_diff_options,omitempty" json:"file_diff_options,omitempty"`
}

// BundleConfig stores portable preferences and subscription metadata inside the archive.
type BundleConfig struct {
	Preferences PortablePreferences `yaml:"preferences"`
	Subs        []profile.Profile   `yaml:"subs,omitempty"`
	ActiveSub   string              `yaml:"active_sub,omitempty"`
}

// Bundle represents an in-memory portable snapshot of Mihosh settings, subscriptions,
// override YAML, custom rule edits, and proxy chain edits.
type Bundle struct {
	Manifest   Manifest
	Config     BundleConfig
	MergeYAML  []byte
	RuleEdits  []byte
	ChainEdits []byte
}

// BuildBundle constructs a portable Bundle from local Mihosh configuration and overlay files.
// When redact is true, sensitive credentials in subscription URLs and merge.yaml are replaced
// with RedactedPlaceholder.
func BuildBundle(cfg *config.Config, mergeYAML, ruleEdits, chainEdits []byte, redact bool) (Bundle, error) {
	if cfg == nil {
		c := config.DefaultConfig
		cfg = &c
	}

	hostname, _ := os.Hostname()
	redactedCount := 0

	prefs := PortablePreferences{
		TestURL:             cfg.TestURL,
		Timeout:             cfg.Timeout,
		Language:            cfg.Language,
		AutoRefreshInterval: cfg.AutoRefreshInterval,
		Theme:               cfg.Theme,
		BorderStyle:         cfg.BorderStyle,
		ShellProxy:          cfg.ShellProxy,
	}
	if cfg.FileDiffOptions != nil {
		opts := *cfg.FileDiffOptions
		prefs.FileDiffOptions = &opts
	}

	subs := make([]profile.Profile, 0, len(cfg.Subs))
	for _, sub := range cfg.Subs {
		cloned := sub
		if sub.Usage != nil {
			u := *sub.Usage
			cloned.Usage = &u
		}
		if redact {
			if cloned.Source.URL != "" {
				if cleaned, did := RedactURL(cloned.Source.URL); did {
					cloned.Source.URL = cleaned
					redactedCount++
				}
			}
			// Do not leak host-specific local absolute path outside the machine when redacting.
			if cloned.Source.Kind == profile.SourceLocal && filepath.IsAbs(cloned.Source.Path) {
				cloned.Source.Path = filepath.Base(cloned.Source.Path)
			}
		}
		subs = append(subs, cloned)
	}

	var outMerge []byte
	if len(bytes.TrimSpace(mergeYAML)) > 0 {
		if redact {
			cleaned, n, err := RedactYAML(mergeYAML)
			if err != nil {
				return Bundle{}, fmt.Errorf("redact merge.yaml: %w", err)
			}
			outMerge = cleaned
			redactedCount += n
		} else {
			outMerge = append([]byte(nil), mergeYAML...)
		}
	}

	files := []string{entryManifest, entryConfig}
	if len(bytes.TrimSpace(outMerge)) > 0 {
		files = append(files, entryMerge)
	}
	var outRules []byte
	if len(bytes.TrimSpace(ruleEdits)) > 0 {
		outRules = append([]byte(nil), ruleEdits...)
		files = append(files, entryRuleEdits)
	}
	var outChains []byte
	if len(bytes.TrimSpace(chainEdits)) > 0 {
		outChains = append([]byte(nil), chainEdits...)
		files = append(files, entryChainEdits)
	}

	return Bundle{
		Manifest: Manifest{
			SchemaVersion: BundleSchemaVersion,
			AppVersion:    model.Version,
			Device:        hostname,
			ExportedAt:    time.Now().UTC().Truncate(time.Second),
			Redacted:      redact,
			RedactedCount: redactedCount,
			Files:         files,
		},
		Config: BundleConfig{
			Preferences: prefs,
			Subs:        subs,
			ActiveSub:   cfg.ActiveSub,
		},
		MergeYAML:  outMerge,
		RuleEdits:  outRules,
		ChainEdits: outChains,
	}, nil
}

// ContentSHA256 computes a deterministic SHA-256 digest of the bundle's semantic contents
// (excluding volatile ExportedAt/Device metadata so identical state yields identical hash).
func ContentSHA256(b Bundle) (string, error) {
	cfgBytes, err := marshalBundleYAML(b.Config)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	writeSection := func(name string, data []byte) {
		_, _ = h.Write([]byte(name))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(data)
		_, _ = h.Write([]byte{0})
	}
	writeSection(entryConfig, cfgBytes)
	writeSection(entryMerge, b.MergeYAML)
	writeSection(entryRuleEdits, b.RuleEdits)
	writeSection(entryChainEdits, b.ChainEdits)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func marshalBundleYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type archiveEntry struct {
	Name string
	Data []byte
}

func bundleEntries(b Bundle) ([]archiveEntry, error) {
	cfgBytes, err := marshalBundleYAML(b.Config)
	if err != nil {
		return nil, fmt.Errorf("encode bundle config: %w", err)
	}
	manifest := b.Manifest
	if manifest.SchemaVersion == 0 {
		manifest.SchemaVersion = BundleSchemaVersion
	}
	files := []string{entryManifest, entryConfig}
	entries := []archiveEntry{{Name: entryConfig, Data: cfgBytes}}
	if len(bytes.TrimSpace(b.MergeYAML)) > 0 {
		files = append(files, entryMerge)
		entries = append(entries, archiveEntry{Name: entryMerge, Data: b.MergeYAML})
	}
	if len(bytes.TrimSpace(b.RuleEdits)) > 0 {
		files = append(files, entryRuleEdits)
		entries = append(entries, archiveEntry{Name: entryRuleEdits, Data: b.RuleEdits})
	}
	if len(bytes.TrimSpace(b.ChainEdits)) > 0 {
		files = append(files, entryChainEdits)
		entries = append(entries, archiveEntry{Name: entryChainEdits, Data: b.ChainEdits})
	}
	manifest.Files = files
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	return append([]archiveEntry{{Name: entryManifest, Data: manifestBytes}}, entries...), nil
}

// EncodeTarGz serializes a Bundle into a .tar.gz archive byte slice.
func EncodeTarGz(b Bundle) ([]byte, error) {
	entries, err := bundleEntries(b)
	if err != nil {
		return nil, err
	}
	modTime := b.Manifest.ExportedAt
	if modTime.IsZero() {
		modTime = time.Now().UTC()
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:    e.Name,
			Mode:    0600,
			Size:    int64(len(e.Data)),
			ModTime: modTime,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return nil, err
		}
		if _, err := tw.Write(e.Data); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		_ = gz.Close()
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EncodeZip serializes a Bundle into a .zip archive byte slice.
func EncodeZip(b Bundle) ([]byte, error) {
	entries, err := bundleEntries(b)
	if err != nil {
		return nil, err
	}
	modTime := b.Manifest.ExportedAt
	if modTime.IsZero() {
		modTime = time.Now().UTC()
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		hdr := &zip.FileHeader{
			Name:     e.Name,
			Method:   zip.Deflate,
			Modified: modTime,
		}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if _, err := w.Write(e.Data); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func validateArchiveEntryName(raw string) (string, error) {
	if raw == "" || strings.ContainsRune(raw, 0) || strings.Contains(raw, "\\") {
		return "", fmt.Errorf("%w: unsafe entry path %q", ErrInvalidArchive, raw)
	}
	if strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("%w: absolute entry path %q", ErrInvalidArchive, raw)
	}
	cleaned := path.Clean(raw)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/") {
		return "", fmt.Errorf("%w: invalid nested or traversal path %q", ErrInvalidArchive, raw)
	}
	switch cleaned {
	case entryManifest, entryConfig, entryMerge, entryRuleEdits, entryChainEdits:
		return cleaned, nil
	default:
		return "", fmt.Errorf("%w: unexpected archive entry %q", ErrInvalidArchive, raw)
	}
}

func assembleDecodedBundle(files map[string][]byte) (Bundle, error) {
	manifestData, ok := files[entryManifest]
	if !ok {
		return Bundle{}, fmt.Errorf("%w: missing %s", ErrInvalidArchive, entryManifest)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return Bundle{}, fmt.Errorf("%w: parse manifest: %v", ErrInvalidArchive, err)
	}
	if manifest.SchemaVersion <= 0 || manifest.SchemaVersion > BundleSchemaVersion {
		return Bundle{}, fmt.Errorf("%w: %d", ErrUnsupportedSchema, manifest.SchemaVersion)
	}

	cfgData, ok := files[entryConfig]
	if !ok {
		return Bundle{}, fmt.Errorf("%w: missing %s", ErrInvalidArchive, entryConfig)
	}
	var cfg BundleConfig
	if err := yaml.Unmarshal(cfgData, &cfg); err != nil {
		return Bundle{}, fmt.Errorf("%w: parse config.yaml: %v", ErrInvalidArchive, err)
	}

	return Bundle{
		Manifest:   manifest,
		Config:     cfg,
		MergeYAML:  files[entryMerge],
		RuleEdits:  files[entryRuleEdits],
		ChainEdits: files[entryChainEdits],
	}, nil
}

// DecodeTarGz safely parses and validates a .tar.gz archive into a Bundle.
func DecodeTarGz(data []byte) (Bundle, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return Bundle{}, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	files := make(map[string][]byte)
	var total int64

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Bundle{}, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			return Bundle{}, fmt.Errorf("%w: unsupported tar entry type %d for %q", ErrInvalidArchive, hdr.Typeflag, hdr.Name)
		}
		name, err := validateArchiveEntryName(hdr.Name)
		if err != nil {
			return Bundle{}, err
		}
		if hdr.Size < 0 || hdr.Size > MaxArchiveFileBytes {
			return Bundle{}, fmt.Errorf("%w: entry %q exceeds size limit", ErrInvalidArchive, name)
		}
		limited := io.LimitReader(tr, MaxArchiveFileBytes+1)
		content, err := io.ReadAll(limited)
		if err != nil {
			return Bundle{}, fmt.Errorf("%w: read %q: %v", ErrInvalidArchive, name, err)
		}
		if int64(len(content)) > MaxArchiveFileBytes {
			return Bundle{}, fmt.Errorf("%w: entry %q exceeds size limit", ErrInvalidArchive, name)
		}
		total += int64(len(content))
		if total > MaxArchiveBytes {
			return Bundle{}, fmt.Errorf("%w: archive exceeds total size limit", ErrInvalidArchive)
		}
		files[name] = content
	}
	return assembleDecodedBundle(files)
}

// DecodeZip safely parses and validates a .zip archive into a Bundle.
func DecodeZip(data []byte) (Bundle, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Bundle{}, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	files := make(map[string][]byte)
	var total int64

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return Bundle{}, fmt.Errorf("%w: symlink entry %q is not allowed", ErrInvalidArchive, f.Name)
		}
		name, err := validateArchiveEntryName(f.Name)
		if err != nil {
			return Bundle{}, err
		}
		rc, err := f.Open()
		if err != nil {
			return Bundle{}, fmt.Errorf("%w: open %q: %v", ErrInvalidArchive, name, err)
		}
		content, err := io.ReadAll(io.LimitReader(rc, MaxArchiveFileBytes+1))
		_ = rc.Close()
		if err != nil {
			return Bundle{}, fmt.Errorf("%w: read %q: %v", ErrInvalidArchive, name, err)
		}
		if int64(len(content)) > MaxArchiveFileBytes {
			return Bundle{}, fmt.Errorf("%w: entry %q exceeds size limit", ErrInvalidArchive, name)
		}
		total += int64(len(content))
		if total > MaxArchiveBytes {
			return Bundle{}, fmt.Errorf("%w: archive exceeds total size limit", ErrInvalidArchive)
		}
		files[name] = content
	}
	return assembleDecodedBundle(files)
}

// WriteArchiveFile encodes bundle as .zip (when path ends with .zip) or .tar.gz and writes atomically.
func WriteArchiveFile(targetPath string, b Bundle) (int, error) {
	if strings.TrimSpace(targetPath) == "" {
		return 0, errors.New("archive output path is required")
	}
	var data []byte
	var err error
	if strings.EqualFold(filepath.Ext(targetPath), ".zip") {
		data, err = EncodeZip(b)
	} else {
		data, err = EncodeTarGz(b)
	}
	if err != nil {
		return 0, err
	}
	dir := filepath.Dir(targetPath)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(dir, ".mihosh-export-*")
	if err != nil {
		return 0, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return 0, err
	}
	if err = tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return 0, err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return 0, err
	}
	if err = tmp.Close(); err != nil {
		return 0, err
	}
	if err = os.Rename(tmp.Name(), targetPath); err != nil {
		return 0, err
	}
	return len(data), nil
}

// ReadArchiveFile reads a .tar.gz or .zip archive (sniffing magic bytes) into a validated Bundle.
func ReadArchiveFile(sourcePath string) (Bundle, error) {
	if strings.TrimSpace(sourcePath) == "" {
		return Bundle{}, errors.New("archive input path is required")
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return Bundle{}, err
	}
	if info.Size() > MaxArchiveBytes {
		return Bundle{}, fmt.Errorf("%w: file exceeds %d bytes limit", ErrInvalidArchive, MaxArchiveBytes)
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return Bundle{}, err
	}
	return DecodeArchive(data)
}

// DecodeArchive auto-detects .zip vs .tar.gz by magic header bytes.
func DecodeArchive(data []byte) (Bundle, error) {
	if len(data) < 4 {
		return Bundle{}, fmt.Errorf("%w: file too small", ErrInvalidArchive)
	}
	// ZIP magic: PK\x03\x04
	if data[0] == 0x50 && data[1] == 0x4b && data[2] == 0x03 && data[3] == 0x04 {
		return DecodeZip(data)
	}
	// GZIP magic: \x1f\x8b
	if data[0] == 0x1f && data[1] == 0x8b {
		return DecodeTarGz(data)
	}
	return Bundle{}, fmt.Errorf("%w: unsupported archive format (expected .tar.gz or .zip)", ErrInvalidArchive)
}
