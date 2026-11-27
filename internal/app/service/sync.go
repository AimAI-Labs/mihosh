package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	syncinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/sync"
)

const (
	// EnvSyncPGDSN allows supplying PostgreSQL DSN via environment variable.
	EnvSyncPGDSN = "MIHOSH_SYNC_PG_DSN"
	// EnvSyncKey allows supplying sync profile key via environment variable.
	EnvSyncKey = "MIHOSH_SYNC_KEY"
)

// SyncExportResult summarizes a local archive export operation.
type SyncExportResult struct {
	Path          string    `json:"path"`
	SizeBytes     int       `json:"size_bytes"`
	ContentSHA256 string    `json:"content_sha256"`
	Redacted      bool      `json:"redacted"`
	RedactedCount int       `json:"redacted_count"`
	SubsCount     int       `json:"subs_count"`
	Files         []string  `json:"files"`
	ExportedAt    time.Time `json:"exported_at"`
}

// SyncRestoreResult summarizes restoring a bundle (from local archive or PG cloud pull).
type SyncRestoreResult struct {
	Source           string `json:"source"`
	ProfileKey       string `json:"profile_key,omitempty"`
	Device           string `json:"device,omitempty"`
	SubsCount        int    `json:"subs_count"`
	RestoredMerge    bool   `json:"restored_merge"`
	RestoredRules    bool   `json:"restored_rules"`
	RestoredChains   bool   `json:"restored_chains"`
	PreservedSecrets int    `json:"preserved_secrets"`
	Redacted         bool   `json:"redacted"`
}

// SyncPushResult summarizes a PostgreSQL cloud push operation.
type SyncPushResult struct {
	ProfileKey    string    `json:"profile_key"`
	DSN           string    `json:"dsn"`
	SizeBytes     int       `json:"size_bytes"`
	PayloadSHA256 string    `json:"payload_sha256"`
	ContentSHA256 string    `json:"content_sha256"`
	Redacted      bool      `json:"redacted"`
	RedactedCount int       `json:"redacted_count"`
	SubsCount     int       `json:"subs_count"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SyncStatusResult summarizes local vs remote PostgreSQL sync state.
type SyncStatusResult struct {
	ProfileKey     string    `json:"profile_key"`
	DSN            string    `json:"dsn"`
	State          string    `json:"state"` // "in_sync", "diverged", "remote_empty"
	LocalSHA256    string    `json:"local_sha256"`
	RemoteSHA256   string    `json:"remote_sha256,omitempty"`
	RemoteDevice   string    `json:"remote_device,omitempty"`
	RemoteVersion  string    `json:"remote_version,omitempty"`
	RemoteSize     int       `json:"remote_size_bytes,omitempty"`
	RemoteRedacted bool      `json:"remote_redacted,omitempty"`
	RemoteUpdated  time.Time `json:"remote_updated_at,omitempty"`
}

// SyncService orchestrates local archive backup/restore and PostgreSQL cloud sync.
type SyncService struct {
	openStore func(ctx context.Context, dsn string) (syncinfra.Store, error)
}

// NewSyncService creates a SyncService backed by PostgreSQL.
func NewSyncService() *SyncService {
	return &SyncService{
		openStore: syncinfra.OpenPostgres,
	}
}

// DefaultArchivePath returns a timestamped .tar.gz path under ~/.mihosh/backups/.
func (s *SyncService) DefaultArchivePath() (string, error) {
	dir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("mihosh-backup-%s.tar.gz", time.Now().Format("20060102-150405"))
	return filepath.Join(dir, "backups", name), nil
}

// ResolveSettings resolves PostgreSQL DSN and profile key using priority:
// explicit override -> environment variable -> saved config.yaml -> default key.
func (s *SyncService) ResolveSettings(dsnOverride, keyOverride string) (dsn, profileKey string) {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		c := config.DefaultConfig
		cfg = &c
	}

	dsn = strings.TrimSpace(dsnOverride)
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv(EnvSyncPGDSN))
	}
	if dsn == "" {
		dsn = strings.TrimSpace(cfg.Sync.DSN)
	}

	profileKey = strings.TrimSpace(keyOverride)
	if profileKey == "" {
		profileKey = strings.TrimSpace(os.Getenv(EnvSyncKey))
	}
	if profileKey == "" {
		profileKey = strings.TrimSpace(cfg.Sync.ProfileKey)
	}
	profileKey = syncinfra.NormalizeProfileKey(profileKey)
	return dsn, profileKey
}

// SaveSettings persists PostgreSQL DSN and profile key into Mihosh's local config.yaml.
func (s *SyncService) SaveSettings(dsn, profileKey string) error {
	dsn = strings.TrimSpace(dsn)
	profileKey = syncinfra.NormalizeProfileKey(profileKey)
	return config.Update(func(cfg *config.Config) error {
		cfg.Sync.DSN = dsn
		cfg.Sync.ProfileKey = profileKey
		return nil
	})
}

// CollectLocalBundle gathers local preferences, subscription metadata, merge.yaml,
// rule-edits.yaml, and chain-edits.yaml into a portable Bundle.
func (s *SyncService) CollectLocalBundle(redact bool) (syncinfra.Bundle, error) {
	cfg, files, err := config.ReadProfileFiles()
	if err != nil {
		return syncinfra.Bundle{}, err
	}
	return syncinfra.BuildBundle(cfg, files.Merge, files.Rules, files.Chains, redact)
}

func readOptionalSyncFile(pathFn func() (string, error)) ([]byte, error) {
	p, err := pathFn()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ExportArchive writes a local .tar.gz (or .zip) backup archive atomically.
func (s *SyncService) ExportArchive(targetPath string, redact bool) (SyncExportResult, error) {
	if strings.TrimSpace(targetPath) == "" {
		var err error
		targetPath, err = s.DefaultArchivePath()
		if err != nil {
			return SyncExportResult{}, err
		}
	}
	bundle, err := s.CollectLocalBundle(redact)
	if err != nil {
		return SyncExportResult{}, err
	}
	contentSHA, err := syncinfra.ContentSHA256(bundle)
	if err != nil {
		return SyncExportResult{}, err
	}
	size, err := syncinfra.WriteArchiveFile(targetPath, bundle)
	if err != nil {
		return SyncExportResult{}, err
	}
	return SyncExportResult{
		Path:          targetPath,
		SizeBytes:     size,
		ContentSHA256: contentSHA,
		Redacted:      bundle.Manifest.Redacted,
		RedactedCount: bundle.Manifest.RedactedCount,
		SubsCount:     len(bundle.Config.Subs),
		Files:         bundle.Manifest.Files,
		ExportedAt:    bundle.Manifest.ExportedAt,
	}, nil
}

// ImportArchive validates and atomically restores a local .tar.gz or .zip archive.
func (s *SyncService) ImportArchive(sourcePath string) (SyncRestoreResult, error) {
	bundle, err := syncinfra.ReadArchiveFile(sourcePath)
	if err != nil {
		return SyncRestoreResult{}, err
	}
	return s.RestoreBundle(bundle, sourcePath, "")
}

// RestoreBundle applies a decoded Bundle to the local Mihosh configuration directory
// using atomic writes and preserving local secrets whenever incoming fields are redacted.
func (s *SyncService) RestoreBundle(bundle syncinfra.Bundle, source, profileKey string) (SyncRestoreResult, error) {
	res := SyncRestoreResult{
		Source:     source,
		ProfileKey: profileKey,
		Device:     bundle.Manifest.Device,
		SubsCount:  len(bundle.Config.Subs),
		Redacted:   bundle.Manifest.Redacted,
	}

	var files config.ProfileFiles
	err := config.RestoreProfileFiles(func(cfg *config.Config) (config.ProfileFiles, error) {
		var err error
		files, res.PreservedSecrets, err = prepareSyncProfileFiles(bundle)
		if err != nil {
			return config.ProfileFiles{}, err
		}
		// Host-specific core and sync settings stay local.
		prefs := bundle.Config.Preferences
		if prefs.Language != "" && prefs.Language != "auto" && prefs.Language != "zh-CN" && prefs.Language != "en-US" {
			return config.ProfileFiles{}, fmt.Errorf("invalid language preference: %q", prefs.Language)
		}
		if prefs.Theme != "" && !model.IsValidTheme(prefs.Theme) {
			return config.ProfileFiles{}, fmt.Errorf("invalid theme preference: %q", prefs.Theme)
		}
		if prefs.BorderStyle != "" && prefs.BorderStyle != "rounded" && prefs.BorderStyle != "square" {
			return config.ProfileFiles{}, fmt.Errorf("invalid border_style preference: %q", prefs.BorderStyle)
		}
		if prefs.TestURL != "" {
			cfg.TestURL = prefs.TestURL
		}
		if prefs.Timeout > 0 {
			cfg.Timeout = prefs.Timeout
		}
		if prefs.Language != "" {
			cfg.Language = prefs.Language
		}
		if prefs.AutoRefreshInterval >= 0 {
			cfg.AutoRefreshInterval = prefs.AutoRefreshInterval
		}
		if prefs.Theme != "" {
			cfg.Theme = prefs.Theme
		}
		if prefs.BorderStyle != "" {
			cfg.BorderStyle = prefs.BorderStyle
		}
		cfg.ShellProxy = prefs.ShellProxy
		if prefs.FileDiffOptions != nil {
			opts := *prefs.FileDiffOptions
			cfg.FileDiffOptions = &opts
		}

		// Index existing local subs by UID so redacted remote URLs & local absolute paths are preserved.
		existingByUID := make(map[string]profile.Profile, len(cfg.Subs))
		for _, sub := range cfg.Subs {
			existingByUID[sub.UID] = sub
		}

		restoredSubs := make([]profile.Profile, 0, len(bundle.Config.Subs))
		validUIDs := make(map[string]struct{}, len(bundle.Config.Subs))
		for _, incoming := range bundle.Config.Subs {
			if err := validateSyncSubUID(incoming.UID); err != nil {
				return config.ProfileFiles{}, err
			}
			if _, dup := validUIDs[incoming.UID]; dup {
				return config.ProfileFiles{}, fmt.Errorf("duplicate subscription UID in bundle: %q", incoming.UID)
			}
			mergedSub := incoming
			if local, ok := existingByUID[incoming.UID]; ok {
				if mergedSub.Source.URL != "" && syncinfra.ContainsRedacted(mergedSub.Source.URL) &&
					local.Source.URL != "" && !syncinfra.ContainsRedacted(local.Source.URL) {
					mergedSub.Source.URL = local.Source.URL
					res.PreservedSecrets++
				}
				if mergedSub.Source.Kind == profile.SourceLocal && !filepath.IsAbs(mergedSub.Source.Path) && filepath.IsAbs(local.Source.Path) {
					mergedSub.Source.Path = local.Source.Path
				}
				if !mergedSub.Source.SameOrigin(local.Source) {
					if err := profile.BindLegacyRawSource(local); err != nil {
						return config.ProfileFiles{}, err
					}
					mergedSub.UpdatedAt = 0
				} else if mergedSub.Usage == nil && local.Usage != nil {
					u := *local.Usage
					mergedSub.Usage = &u
				}
			}
			restoredSubs = append(restoredSubs, mergedSub)
			validUIDs[mergedSub.UID] = struct{}{}
		}
		cfg.Subs = restoredSubs

		if _, exists := validUIDs[bundle.Config.ActiveSub]; bundle.Config.ActiveSub != "" && exists {
			cfg.ActiveSub = bundle.Config.ActiveSub
		}
		if _, exists := validUIDs[cfg.ActiveSub]; !exists {
			cfg.ActiveSub = ""
		}
		return files, nil
	})
	if err != nil {
		res.PreservedSecrets = 0
		return res, fmt.Errorf("restore configuration bundle: %w", err)
	}
	res.RestoredMerge = files.Merge != nil
	res.RestoredRules = files.Rules != nil
	res.RestoredChains = files.Chains != nil
	return res, nil
}

func validateSyncSubUID(uid string) error {
	if uid == "" || uid == "." || uid == ".." || strings.ContainsAny(uid, `/\`) || !filepath.IsLocal(uid) {
		return fmt.Errorf("invalid subscription UID in bundle: %q", uid)
	}
	return nil
}

func prepareSyncProfileFiles(bundle syncinfra.Bundle) (config.ProfileFiles, int, error) {
	var files config.ProfileFiles
	if len(bytes.TrimSpace(bundle.RuleEdits)) > 0 {
		if _, err := profile.DecodeRuleEdits(bundle.RuleEdits); err != nil {
			return files, 0, fmt.Errorf("validate rule-edits.yaml: %w", err)
		}
		files.Rules = bundle.RuleEdits
	} else {
		files.RemoveRules = true
	}
	if len(bytes.TrimSpace(bundle.ChainEdits)) > 0 {
		if _, err := profile.DecodeChainEdits(bundle.ChainEdits); err != nil {
			return files, 0, fmt.Errorf("validate chain-edits.yaml: %w", err)
		}
		files.Chains = bundle.ChainEdits
	} else {
		files.RemoveChains = true
	}
	if len(bytes.TrimSpace(bundle.MergeYAML)) == 0 {
		files.RemoveMerge = true
		return files, 0, nil
	}
	previous, err := readOptionalSyncFile(profile.MergePath)
	if err != nil {
		return files, 0, err
	}
	merged, preserved, err := syncinfra.RestoreRedactedYAML(bundle.MergeYAML, previous)
	if err != nil {
		return files, 0, fmt.Errorf("restore merge.yaml: %w", err)
	}
	if err := profile.ValidateMergeWrite(merged); err != nil {
		return files, 0, fmt.Errorf("validate merge.yaml: %w", err)
	}
	_, migratedRules, err := profile.PrepareLegacyRuleEdits(previous)
	if err != nil {
		return files, 0, err
	}
	if files.Rules == nil && migratedRules != nil {
		files.Rules = migratedRules
		files.RemoveRules = false
	}
	if err := profile.BackupMergeBeforeRestore(previous, merged); err != nil {
		return files, 0, err
	}
	files.Merge = merged
	return files, preserved, nil
}

// Push uploads a compressed (and by default redacted) local snapshot to PostgreSQL.
func (s *SyncService) Push(ctx context.Context, dsnOverride, keyOverride string, redact bool) (SyncPushResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	dsn, key := s.ResolveSettings(dsnOverride, keyOverride)
	if dsn == "" {
		return SyncPushResult{}, errors.New("PostgreSQL DSN is not configured (pass --dsn, set MIHOSH_SYNC_PG_DSN, or configure in Settings)")
	}
	bundle, err := s.CollectLocalBundle(redact)
	if err != nil {
		return SyncPushResult{}, err
	}
	contentSHA, err := syncinfra.ContentSHA256(bundle)
	if err != nil {
		return SyncPushResult{}, err
	}
	payload, err := syncinfra.EncodeTarGz(bundle)
	if err != nil {
		return SyncPushResult{}, err
	}

	store, err := s.openStore(ctx, dsn)
	if err != nil {
		return SyncPushResult{}, err
	}
	defer func() { _ = store.Close() }()

	now := time.Now().UTC().Truncate(time.Second)
	snap := syncinfra.Snapshot{
		Meta: syncinfra.SnapshotMeta{
			ProfileKey: key,
			SHA256:     syncinfra.PayloadSHA256(payload),
			ContentSHA: contentSHA,
			Device:     bundle.Manifest.Device,
			AppVersion: bundle.Manifest.AppVersion,
			Redacted:   bundle.Manifest.Redacted,
			SizeBytes:  len(payload),
			UpdatedAt:  now,
		},
		Payload: payload,
	}
	if err := store.Push(ctx, snap); err != nil {
		return SyncPushResult{}, err
	}
	_ = config.Update(func(cfg *config.Config) error {
		cfg.Sync.LastSyncAt = now.Format(time.RFC3339)
		return nil
	})
	return SyncPushResult{
		ProfileKey:    key,
		DSN:           syncinfra.RedactDSN(dsn),
		SizeBytes:     len(payload),
		PayloadSHA256: snap.Meta.SHA256,
		ContentSHA256: contentSHA,
		Redacted:      bundle.Manifest.Redacted,
		RedactedCount: bundle.Manifest.RedactedCount,
		SubsCount:     len(bundle.Config.Subs),
		UpdatedAt:     now,
	}, nil
}

// Pull downloads the latest snapshot for profileKey from PostgreSQL and restores it locally.
func (s *SyncService) Pull(ctx context.Context, dsnOverride, keyOverride string) (SyncRestoreResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	dsn, key := s.ResolveSettings(dsnOverride, keyOverride)
	if dsn == "" {
		return SyncRestoreResult{}, errors.New("PostgreSQL DSN is not configured (pass --dsn, set MIHOSH_SYNC_PG_DSN, or configure in Settings)")
	}
	store, err := s.openStore(ctx, dsn)
	if err != nil {
		return SyncRestoreResult{}, err
	}
	defer func() { _ = store.Close() }()

	snap, err := store.Pull(ctx, key)
	if err != nil {
		return SyncRestoreResult{}, err
	}
	bundle, err := syncinfra.DecodeArchive(snap.Payload)
	if err != nil {
		return SyncRestoreResult{}, err
	}
	res, err := s.RestoreBundle(bundle, syncinfra.RedactDSN(dsn), key)
	if err != nil {
		return res, err
	}
	_ = config.Update(func(cfg *config.Config) error {
		cfg.Sync.LastSyncAt = time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
		return nil
	})
	return res, nil
}

// Status compares the local bundle content hash against the remote PostgreSQL snapshot.
func (s *SyncService) Status(ctx context.Context, dsnOverride, keyOverride string) (SyncStatusResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	dsn, key := s.ResolveSettings(dsnOverride, keyOverride)
	if dsn == "" {
		return SyncStatusResult{}, errors.New("PostgreSQL DSN is not configured (pass --dsn, set MIHOSH_SYNC_PG_DSN, or configure in Settings)")
	}
	store, err := s.openStore(ctx, dsn)
	if err != nil {
		return SyncStatusResult{}, err
	}
	defer func() { _ = store.Close() }()

	meta, err := store.Status(ctx, key)
	if errors.Is(err, syncinfra.ErrSnapshotNotFound) {
		localBundle, bErr := s.CollectLocalBundle(true)
		if bErr != nil {
			return SyncStatusResult{}, bErr
		}
		localSHA, hErr := syncinfra.ContentSHA256(localBundle)
		if hErr != nil {
			return SyncStatusResult{}, hErr
		}
		return SyncStatusResult{
			ProfileKey:  key,
			DSN:         syncinfra.RedactDSN(dsn),
			State:       "remote_empty",
			LocalSHA256: localSHA,
		}, nil
	}
	if err != nil {
		return SyncStatusResult{}, err
	}

	localBundle, err := s.CollectLocalBundle(meta.Redacted)
	if err != nil {
		return SyncStatusResult{}, err
	}
	localSHA, err := syncinfra.ContentSHA256(localBundle)
	if err != nil {
		return SyncStatusResult{}, err
	}

	remoteHash := meta.ContentSHA
	if remoteHash == "" {
		remoteHash = meta.SHA256
	}
	state := "diverged"
	if meta.ContentSHA != "" && meta.ContentSHA == localSHA {
		state = "in_sync"
	}
	return SyncStatusResult{
		ProfileKey:     key,
		DSN:            syncinfra.RedactDSN(dsn),
		State:          state,
		LocalSHA256:    localSHA,
		RemoteSHA256:   remoteHash,
		RemoteDevice:   meta.Device,
		RemoteVersion:  meta.AppVersion,
		RemoteSize:     meta.SizeBytes,
		RemoteRedacted: meta.Redacted,
		RemoteUpdated:  meta.UpdatedAt,
	}, nil
}
