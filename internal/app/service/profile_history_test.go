package service

import (
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
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
