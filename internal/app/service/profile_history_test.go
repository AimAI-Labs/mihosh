package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

func TestProfileServiceHistoryDiffAndRollback(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	if err := config.Save(&config.DefaultConfig); err != nil {
		t.Fatal(err)
	}
	svc := NewProfileService(nil)
	p, err := svc.AddProfile("demo", profile.SubSource{Kind: profile.SourceRemote, URL: "https://example.org/sub.yaml"})
	if err != nil {
		t.Fatal(err)
	}

	// Write v1 then v2 for raw.yaml -> v1 is snapshotted automatically.
	v1 := []byte("mixed-port: 7890\nproxies:\n  - name: v1-node\n")
	v2 := []byte("mixed-port: 7890\nproxies:\n  - name: v2-node\n")
	if err := profile.WriteRaw(p.UID, v1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if err := profile.WriteRaw(p.UID, v2); err != nil {
		t.Fatal(err)
	}

	snaps, err := svc.ListRawSnapshots(p.UID)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("expected 1 raw snapshot, got %d err=%v", len(snaps), err)
	}
	diff, err := svc.DiffSnapshotWithCurrent(profile.SnapshotRaw, p.UID, snaps[0].ID, profile.DefaultFileDiffOptions)
	if err != nil || len(diff) == 0 {
		t.Fatalf("expected non-empty diff, got len=%d err=%v", len(diff), err)
	}

	// Rollback raw.yaml to v1 (inactive profile -> no core reload required).
	if _, err := svc.RollbackSnapshot(profile.SnapshotRaw, p.UID, snaps[0].ID); err != nil {
		t.Fatalf("rollback raw failed: %v", err)
	}
	cur, err := profile.ReadRawFile(p.UID)
	if err != nil || string(cur) != string(v1) {
		t.Fatalf("expected raw.yaml restored to v1, got %q err=%v", string(cur), err)
	}
	// Rollback itself saved v2 as a snapshot, so rolling back is reversible!
	snapsAfter, err := svc.ListRawSnapshots(p.UID)
	if err != nil || len(snapsAfter) != 2 {
		t.Fatalf("expected 2 snapshots after reversible rollback, got %d err=%v", len(snapsAfter), err)
	}

	// Merge snapshot & rollback
	if err := profile.WriteMerge([]byte("log-level: info\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if err := profile.WriteMerge([]byte("log-level: debug\n")); err != nil {
		t.Fatal(err)
	}
	mSnaps, err := svc.ListMergeSnapshots()
	if err != nil || len(mSnaps) != 1 {
		t.Fatalf("expected 1 merge snapshot, got %d err=%v", len(mSnaps), err)
	}
	if _, err := svc.RollbackSnapshot(profile.SnapshotMerge, "", mSnaps[0].ID); err != nil {
		t.Fatalf("rollback merge failed: %v", err)
	}
	curMerge, err := profile.ReadMerge()
	if err != nil || string(curMerge) != "log-level: info\n" {
		t.Fatalf("expected merge.yaml restored to info, got %q err=%v", string(curMerge), err)
	}
}

func TestRawHistoryRollbackRestoresCurrentFilesWhenActivationFails(t *testing.T) {
	core, uid, _ := prepareManagedProfile(t)
	cfg, err := config.Load()
	require.NoError(t, err)
	p, err := profileByUID(cfg, uid)
	require.NoError(t, err)
	require.NoError(t, config.Update(func(cfg *config.Config) error {
		cfg.ActiveSub = uid
		return nil
	}))
	current := []byte("mixed-port: 7890\nproxies: []\n")
	require.NoError(t, profile.WriteRawForSource(p, []byte("invalid YAML: [\n")))
	require.NoError(t, profile.WriteRawForSource(p, current))
	rawPath, err := profile.RawPath(uid)
	require.NoError(t, err)
	bindingPath := rawPath + ".source.json"
	beforeBinding, err := os.ReadFile(bindingPath)
	require.NoError(t, err)
	snapshots, err := profile.ListSnapshots(profile.SnapshotRaw, uid)
	require.NoError(t, err)
	require.NotEmpty(t, snapshots)

	_, err = NewProfileService(nil, core).RollbackSnapshot(profile.SnapshotRaw, uid, snapshots[0].ID)
	require.Error(t, err)
	raw, err := profile.ReadRawFile(uid)
	require.NoError(t, err)
	require.Equal(t, current, raw)
	binding, err := os.ReadFile(bindingPath)
	require.NoError(t, err)
	require.Equal(t, beforeBinding, binding)
}

func TestMergeHistoryRollbackUsesManagedRecoveryWithoutActiveSubscription(t *testing.T) {
	core, _, _ := prepareManagedProfile(t)
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	require.NoError(t, config.Update(func(cfg *config.Config) error {
		cfg.ActiveSub = ""
		return nil
	}))
	journal := filepath.Join(root, "runtime", "profile-recovery.json")
	var seen atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/configs":
			data, err := os.ReadFile(journal)
			if err != nil || !strings.Contains(string(data), `"has_merge":true`) {
				t.Error("merge recovery journal missing when applying the snapshot")
			}
			seen.Store(true)
			w.WriteHeader(http.StatusNoContent)
		case "/version":
			_, _ = w.Write([]byte(`{"version":"fixture"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	require.NoError(t, config.WriteMihomoField(path, "external-controller", strings.TrimPrefix(server.URL, "http://")))
	snapshot, err := profile.SaveMergeSnapshot([]byte("log-level: debug\n"), 10)
	require.NoError(t, err)
	require.NoError(t, profile.WriteMerge([]byte("log-level: info\n")))
	core.process = &exec.Cmd{}
	defer func() { core.process = nil }()
	require.NoError(t, core.startControl(root))
	caller := NewCoreService()
	defer caller.Close()

	result, err := NewProfileService(nil, caller).RollbackSnapshot(profile.SnapshotMerge, "", snapshot.ID)
	require.NoError(t, err)
	require.True(t, seen.Load(), "rollback did not apply the snapshot to the running core")
	require.True(t, result.CoreApplied)
	require.NotEmpty(t, result.BackupName)
	runtimeConfig, err := config.ReadMihomoYAML(path)
	require.NoError(t, err)
	require.Equal(t, "debug", runtimeConfig["log-level"])
	_, err = os.Stat(journal)
	require.ErrorIs(t, err, os.ErrNotExist)
}
