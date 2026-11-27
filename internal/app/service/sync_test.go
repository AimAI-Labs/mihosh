package service

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	syncinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/sync"
	"github.com/stretchr/testify/require"
)

type memorySyncStore struct {
	mu    sync.Mutex
	snaps map[string]syncinfra.Snapshot
}

func newMemorySyncStore() *memorySyncStore {
	return &memorySyncStore{snaps: make(map[string]syncinfra.Snapshot)}
}

func (m *memorySyncStore) Push(_ context.Context, snap syncinfra.Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := syncinfra.NormalizeProfileKey(snap.Meta.ProfileKey)
	snap.Meta.ProfileKey = key
	m.snaps[key] = snap
	return nil
}

func (m *memorySyncStore) Pull(_ context.Context, profileKey string) (syncinfra.Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := syncinfra.NormalizeProfileKey(profileKey)
	snap, ok := m.snaps[key]
	if !ok {
		return syncinfra.Snapshot{}, syncinfra.ErrSnapshotNotFound
	}
	return snap, nil
}

func (m *memorySyncStore) Status(_ context.Context, profileKey string) (syncinfra.SnapshotMeta, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := syncinfra.NormalizeProfileKey(profileKey)
	snap, ok := m.snaps[key]
	if !ok {
		return syncinfra.SnapshotMeta{}, syncinfra.ErrSnapshotNotFound
	}
	return snap.Meta, nil
}

func (m *memorySyncStore) Close() error { return nil }

func TestRestoreBundleRejectsInvalidRulesWithoutReplacingAnyFile(t *testing.T) {
	root := t.TempDir()
	seedSyncDevice(t, root)
	svc := NewSyncService()
	beforeConfig, beforeFiles, err := config.ReadProfileFiles()
	require.NoError(t, err)
	bundle, err := svc.CollectLocalBundle(false)
	require.NoError(t, err)
	bundle.MergeYAML = []byte("log-level: debug\n")
	bundle.RuleEdits = []byte("legacy:\n  mixed-port: 7890\n")
	bundle.Config.Preferences.Language = "en-US"
	archive := filepath.Join(t.TempDir(), "invalid-rules.tar.gz")
	_, err = syncinfra.WriteArchiveFile(archive, bundle)
	require.NoError(t, err)
	result, err := svc.ImportArchive(archive)
	require.ErrorContains(t, err, "validate rule-edits.yaml")
	require.False(t, result.RestoredMerge)
	afterConfig, afterFiles, err := config.ReadProfileFiles()
	require.NoError(t, err)
	require.Equal(t, beforeConfig, afterConfig)
	require.Equal(t, beforeFiles, afterFiles)
}

func seedSyncDevice(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("MIHOSH_CONFIG_DIR", dir)
	cfg := config.DefaultConfig
	cfg.TestURL = "https://cp.cloudflare.com/generate_204"
	cfg.Timeout = 3500
	cfg.Language = "zh-CN"
	cfg.Theme = "nord"
	cfg.BorderStyle = "square"
	cfg.AutoRefreshInterval = 12
	cfg.Core.ExternalSecret = "local-core-secret"
	cfg.Sync = config.SyncConfig{
		DSN:        "postgres://u:supersecret@db.internal:5432/mihosh",
		ProfileKey: "workstation",
	}
	cfg.ActiveSub = "uid-main"
	cfg.Subs = []profile.Profile{{
		UID:  "uid-main",
		Name: "Main Airport",
		Source: profile.SubSource{
			Kind:           profile.SourceRemote,
			URL:            "https://sub.example.com/get?token=myrealsecret&flag=meta",
			UseProxy:       true,
			UserAgent:      "mihosh/test",
			UpdateInterval: 360,
		},
	}}
	require.NoError(t, config.Save(&cfg))
	require.NoError(t, profile.WriteMerge([]byte("mixed-port: 7890\nsecret: my-controller-secret\nlog-level: warning\n")))

	rulePath, err := profile.RuleEditsPath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(rulePath, []byte("edits:\n  - rule: DOMAIN,example.org,DIRECT\n"), 0600))

	require.NoError(t, profile.WriteChainEdits(profile.ChainEdits{
		Chains: []profile.ProxyChain{{
			Group:   "PROXY",
			Nodes:   []string{"HK-1", "US-2"},
			Enabled: true,
		}},
	}))
}

func TestSyncServiceExportImportAndPreserveRedactedSecrets(t *testing.T) {
	deviceA := t.TempDir()
	deviceB := t.TempDir()
	archivePath := filepath.Join(t.TempDir(), "mihosh-sync.tar.gz")

	seedSyncDevice(t, deviceA)
	svc := NewSyncService()

	// Export from Device A with automatic redaction.
	exp, err := svc.ExportArchive(archivePath, true)
	require.NoError(t, err)
	require.True(t, exp.Redacted)
	require.GreaterOrEqual(t, exp.RedactedCount, 2)
	require.Equal(t, 1, exp.SubsCount)

	// Import onto fresh Device B.
	t.Setenv("MIHOSH_CONFIG_DIR", deviceB)
	restoredB, err := svc.ImportArchive(archivePath)
	require.NoError(t, err)
	require.True(t, restoredB.RestoredMerge)
	require.True(t, restoredB.RestoredRules)
	require.True(t, restoredB.RestoredChains)
	require.Equal(t, 1, restoredB.SubsCount)

	cfgB, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "nord", cfgB.Theme)
	require.Equal(t, "square", cfgB.BorderStyle)
	require.Equal(t, 3500, cfgB.Timeout)
	require.Equal(t, "uid-main", cfgB.ActiveSub)
	require.Empty(t, cfgB.Core.ExternalSecret)
	require.Empty(t, cfgB.Sync.DSN)
	require.Contains(t, cfgB.Subs[0].Source.URL, syncinfra.RedactedPlaceholder)
	require.NotContains(t, cfgB.Subs[0].Source.URL, "myrealsecret")

	mergeB, err := profile.ReadMerge()
	require.NoError(t, err)
	require.Contains(t, string(mergeB), syncinfra.RedactedPlaceholder)
	require.NotContains(t, string(mergeB), "my-controller-secret")

	chainsB, err := profile.ReadChainEdits()
	require.NoError(t, err)
	ch, ok := chainsB.GroupChain("PROXY")
	require.True(t, ok)
	require.Equal(t, []string{"HK-1", "US-2"}, ch.Nodes)

	// Re-importing the redacted archive back onto Device A must preserve Device A's real secrets.
	t.Setenv("MIHOSH_CONFIG_DIR", deviceA)
	restoredA, err := svc.ImportArchive(archivePath)
	require.NoError(t, err)
	require.GreaterOrEqual(t, restoredA.PreservedSecrets, 2)

	cfgA, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "https://sub.example.com/get?token=myrealsecret&flag=meta", cfgA.Subs[0].Source.URL)
	require.Equal(t, "local-core-secret", cfgA.Core.ExternalSecret)
	require.Equal(t, "postgres://u:supersecret@db.internal:5432/mihosh", cfgA.Sync.DSN)

	mergeA, err := profile.ReadMerge()
	require.NoError(t, err)
	require.Contains(t, string(mergeA), "my-controller-secret")
}

func TestSyncServicePostgresPushStatusPull(t *testing.T) {
	store := newMemorySyncStore()
	svc := &SyncService{
		openStore: func(context.Context, string) (syncinfra.Store, error) {
			return store, nil
		},
	}

	deviceA := t.TempDir()
	deviceB := t.TempDir()
	seedSyncDevice(t, deviceA)

	// Initially remote is empty.
	stEmpty, err := svc.Status(context.Background(), "", "")
	require.NoError(t, err)
	require.Equal(t, "remote_empty", stEmpty.State)
	require.Equal(t, "workstation", stEmpty.ProfileKey)
	require.NotContains(t, stEmpty.DSN, "supersecret")

	// Push from Device A.
	pushed, err := svc.Push(context.Background(), "", "", true)
	require.NoError(t, err)
	require.True(t, pushed.Redacted)
	require.Equal(t, "workstation", pushed.ProfileKey)
	require.NotContains(t, pushed.DSN, "supersecret")

	// Status immediately after push is in_sync.
	stInSync, err := svc.Status(context.Background(), "", "")
	require.NoError(t, err)
	require.Equal(t, "in_sync", stInSync.State)

	// Mutating local theme makes status diverged.
	require.NoError(t, config.Update(func(cfg *config.Config) error {
		cfg.Theme = "gruvbox"
		return nil
	}))
	stDiv, err := svc.Status(context.Background(), "", "")
	require.NoError(t, err)
	require.Equal(t, "diverged", stDiv.State)

	// Pull on Device B restores everything from PostgreSQL snapshot.
	t.Setenv("MIHOSH_CONFIG_DIR", deviceB)
	pulled, err := svc.Pull(context.Background(), "postgres://u:supersecret@db.internal:5432/mihosh", "workstation")
	require.NoError(t, err)
	require.Equal(t, "workstation", pulled.ProfileKey)
	require.True(t, pulled.RestoredMerge)
	require.True(t, pulled.RestoredRules)
	require.True(t, pulled.RestoredChains)

	cfgB, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "nord", cfgB.Theme)
	require.NotEmpty(t, cfgB.Sync.LastSyncAt)
}

func TestSyncStatusMatchesUnredactedRemoteAndValidatesBundle(t *testing.T) {
	store := newMemorySyncStore()
	svc := &SyncService{
		openStore: func(context.Context, string) (syncinfra.Store, error) {
			return store, nil
		},
	}

	deviceA := t.TempDir()
	seedSyncDevice(t, deviceA)

	// Push unredacted snapshot; Status must compare against unredacted local bundle.
	pushed, err := svc.Push(context.Background(), "", "", false)
	require.NoError(t, err)
	require.False(t, pushed.Redacted)

	st, err := svc.Status(context.Background(), "", "")
	require.NoError(t, err)
	require.Equal(t, "in_sync", st.State)

	// RestoreBundle must reject path-traversal UIDs and invalid preference enums.
	badUIDBundle, err := svc.CollectLocalBundle(true)
	require.NoError(t, err)
	badUIDBundle.Config.Subs[0].UID = "../escape"
	_, err = svc.RestoreBundle(badUIDBundle, "test", "")
	require.ErrorContains(t, err, "invalid subscription UID")

	badThemeBundle, err := svc.CollectLocalBundle(true)
	require.NoError(t, err)
	badThemeBundle.Config.Preferences.Theme = "nonexistent-theme"
	_, err = svc.RestoreBundle(badThemeBundle, "test", "")
	require.ErrorContains(t, err, "invalid theme")

	// Restoring a bundle without merge/rule-edits/chain-edits clears stale local overlays.
	cleanBundle, err := svc.CollectLocalBundle(true)
	require.NoError(t, err)
	cleanBundle.MergeYAML = nil
	cleanBundle.RuleEdits = nil
	cleanBundle.ChainEdits = nil
	_, err = svc.RestoreBundle(cleanBundle, "test", "")
	require.NoError(t, err)
	mergePath, _ := profile.MergePath()
	_, statErr := os.Stat(mergePath)
	require.True(t, os.IsNotExist(statErr), "omitted merge.yaml should be removed on restore")
}

func TestRestoreBundleClearsPortableStateAndKeepsLocalShellProxy(t *testing.T) {
	root := t.TempDir()
	seedSyncDevice(t, root)
	options := profile.DiffIgnoreWhitespace
	require.NoError(t, config.Update(func(cfg *config.Config) error {
		cfg.FileDiffOptions = &options
		cfg.ShellProxy = true
		return nil
	}))
	mergePath, err := profile.MergePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(mergePath, []byte("rules:\n  - MATCH,DIRECT\n"), 0600))
	rulePath, err := profile.RuleEditsPath()
	require.NoError(t, err)

	svc := NewSyncService()
	incoming, err := svc.CollectLocalBundle(false)
	require.NoError(t, err)
	exportedShellProxy := incoming.Config.Preferences.ShellProxy
	incoming.Config.ActiveSub = ""
	incoming.Config.Preferences.FileDiffOptions = nil
	incoming.Config.Preferences.ShellProxy = false
	incoming.MergeYAML = []byte("log-level: warning\n")
	incoming.RuleEdits = nil
	result, err := svc.RestoreBundle(incoming, "test", "")
	require.NoError(t, err)

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.ActiveSub)
	require.Nil(t, cfg.FileDiffOptions)
	require.True(t, cfg.ShellProxy)
	require.False(t, exportedShellProxy)
	require.False(t, result.RestoredRules)
	_, err = os.Stat(rulePath)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestSyncStatusMatchesLegacyShellProxyFlag(t *testing.T) {
	seedSyncDevice(t, t.TempDir())
	store := newMemorySyncStore()
	svc := &SyncService{openStore: func(context.Context, string) (syncinfra.Store, error) {
		return store, nil
	}}
	legacy, err := svc.CollectLocalBundle(true)
	require.NoError(t, err)
	legacy.Config.Preferences.ShellProxy = true
	hash, err := syncinfra.ContentSHA256(legacy)
	require.NoError(t, err)
	require.NoError(t, store.Push(context.Background(), syncinfra.Snapshot{Meta: syncinfra.SnapshotMeta{
		ProfileKey: "workstation", ContentSHA: hash, Redacted: true,
	}}))

	status, err := svc.Status(context.Background(), "", "")
	require.NoError(t, err)
	require.Equal(t, "in_sync", status.State)
}
