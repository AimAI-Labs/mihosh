package profile

import (
	"fmt"
	"testing"
	"time"
)

func TestSnapshotsAutoSaveRetentionDedupAndCleanup(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())

	// 1. WriteRaw auto-snapshots previous content and enforces DefaultSnapshotRetention.
	for i := 0; i < DefaultSnapshotRetention+4; i++ {
		body := []byte(fmt.Sprintf("proxies:\n  - name: node-%d\n", i))
		if err := WriteRaw("sub1", body); err != nil {
			t.Fatalf("WriteRaw #%d: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}

	// Writing identical content must not create a duplicate snapshot.
	lastBody := []byte(fmt.Sprintf("proxies:\n  - name: node-%d\n", DefaultSnapshotRetention+3))
	if err := WriteRaw("sub1", lastBody); err != nil {
		t.Fatalf("duplicate WriteRaw: %v", err)
	}

	snaps, err := ListSnapshots(SnapshotRaw, "sub1")
	if err != nil {
		t.Fatalf("ListSnapshots raw: %v", err)
	}
	if len(snaps) != DefaultSnapshotRetention {
		t.Fatalf("expected %d retained raw snapshots, got %d", DefaultSnapshotRetention, len(snaps))
	}
	// Newest snapshot is the second-to-last write (node-(N+2)).
	data, err := ReadSnapshot(SnapshotRaw, "sub1", snaps[0].ID)
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}
	wantNewest := fmt.Sprintf("proxies:\n  - name: node-%d\n", DefaultSnapshotRetention+2)
	if string(data) != wantNewest {
		t.Fatalf("newest snapshot mismatch: got %q, want %q", string(data), wantNewest)
	}

	// Path traversal rejected
	if _, err := ReadSnapshot(SnapshotRaw, "sub1", "../escape.yaml"); err == nil {
		t.Fatal("expected path traversal snapshot ID to fail")
	}

	// DeleteProfileDir removes raw snapshots too.
	if err := DeleteProfileDir("sub1"); err != nil {
		t.Fatalf("DeleteProfileDir: %v", err)
	}
	afterDel, err := ListSnapshots(SnapshotRaw, "sub1")
	if err != nil || len(afterDel) != 0 {
		t.Fatalf("expected raw snapshots cleaned up after DeleteProfileDir, got %d err=%v", len(afterDel), err)
	}

	// 2. WriteMerge auto-snapshots previous merge.yaml.
	if err := WriteMerge([]byte("mixed-port: 7890\n")); err != nil {
		t.Fatalf("WriteMerge 1: %v", err)
	}
	time.Sleep(time.Millisecond)
	if err := WriteMerge([]byte("mixed-port: 7891\n")); err != nil {
		t.Fatalf("WriteMerge 2: %v", err)
	}
	mergeSnaps, err := ListSnapshots(SnapshotMerge, "")
	if err != nil || len(mergeSnaps) != 1 {
		t.Fatalf("expected 1 merge snapshot, got %d err=%v", len(mergeSnaps), err)
	}
	mergeData, err := ReadSnapshot(SnapshotMerge, "", mergeSnaps[0].ID)
	if err != nil || string(mergeData) != "mixed-port: 7890\n" {
		t.Fatalf("unexpected merge snapshot: %q err=%v", string(mergeData), err)
	}
}
