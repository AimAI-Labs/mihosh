package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultSnapshotRetention is the default number of historical snapshots kept per target.
const DefaultSnapshotRetention = 10

// SnapshotKind identifies whether a snapshot belongs to a subscription raw file or global merge.yaml.
type SnapshotKind string

const (
	SnapshotRaw   SnapshotKind = "raw"
	SnapshotMerge SnapshotKind = "merge"
)

// Snapshot describes a single historical version saved on disk.
type Snapshot struct {
	ID        string       `json:"id"`
	Kind      SnapshotKind `json:"kind"`
	UID       string       `json:"uid,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	Size      int64        `json:"size"`
	SHA256    string       `json:"sha256"`
}

func snapshotDir(kind SnapshotKind, uid string) (string, error) {
	root, err := mihoshConfigDir()
	if err != nil {
		return "", err
	}
	switch kind {
	case SnapshotRaw:
		// filepath.IsLocal also anchors the value for CodeQL's path-injection query,
		// which does not model the equivalent filepath.Base check.
		if !filepath.IsLocal(uid) {
			return "", errors.New("invalid profile ID")
		}
		if uid == "." || strings.ContainsAny(uid, `/\`) {
			return "", errors.New("invalid profile ID")
		}
		return filepath.Join(root, "history", "raw", uid), nil
	case SnapshotMerge:
		return filepath.Join(root, "history", "merge"), nil
	default:
		return "", fmt.Errorf("unsupported snapshot kind: %q", kind)
	}
}

func validateSnapshotID(id string) error {
	if id == "" || id == "." || id == ".." || filepath.Base(id) != id || !strings.HasSuffix(id, ".yaml") {
		return errors.New("invalid snapshot ID")
	}
	return nil
}

// SaveRawSnapshot archives a pre-update snapshot of a subscription's raw.yaml, keeping at most maxKeep entries.
func SaveRawSnapshot(uid string, data []byte, maxKeep int) (Snapshot, error) {
	return saveSnapshot(SnapshotRaw, uid, data, maxKeep, "")
}

// SaveMergeSnapshot archives a pre-edit snapshot of global merge.yaml, keeping at most maxKeep entries.
func SaveMergeSnapshot(data []byte, maxKeep int) (Snapshot, error) {
	return saveSnapshot(SnapshotMerge, "", data, maxKeep, "")
}

func saveSnapshot(kind SnapshotKind, uid string, data []byte, maxKeep int, origin string) (Snapshot, error) {
	if len(trimSpaceBytes(data)) == 0 {
		return Snapshot{}, nil
	}
	if maxKeep <= 0 {
		maxKeep = DefaultSnapshotRetention
	}
	dir, err := snapshotDir(kind, uid)
	if err != nil {
		return Snapshot{}, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return Snapshot{}, fmt.Errorf("create snapshot directory: %w", err)
	}

	sum := sha256.Sum256(data)
	sumHex := hex.EncodeToString(sum[:])

	existing, err := ListSnapshots(kind, uid)
	if err == nil && len(existing) > 0 && existing[0].SHA256 == sumHex[:8] {
		// Latest snapshot already has identical content; avoid redundant duplicates.
		if prevData, rErr := ReadSnapshot(kind, uid, existing[0].ID); rErr == nil {
			prevSum := sha256.Sum256(prevData)
			prevOrigin, err := readRawOrigin(filepath.Join(dir, existing[0].ID), prevData)
			if prevSum == sum && err == nil && prevOrigin == origin {
				return existing[0], nil
			}
		}
	}

	now := time.Now().UTC()
	id := fmt.Sprintf("%s-%s.yaml", now.Format("20060102-150405.000000000"), sumHex[:8])
	path := filepath.Join(dir, id)
	if origin != "" {
		if err := writeRawBindings(path, rawBinding(origin, data)); err != nil {
			return Snapshot{}, err
		}
	}
	if err := atomicWrite(path, data, 0600); err != nil {
		_ = os.Remove(path + ".source.json")
		return Snapshot{}, err
	}

	if err := pruneSnapshots(dir, maxKeep); err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		ID:        id,
		Kind:      kind,
		UID:       uid,
		CreatedAt: now,
		Size:      int64(len(data)),
		SHA256:    sumHex[:8],
	}, nil
}

func pruneSnapshots(dir string, maxKeep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for i := maxKeep; i < len(names); i++ {
		path := filepath.Join(dir, names[i])
		if err := os.Remove(path); err == nil {
			_ = os.Remove(path + ".source.json")
		}
	}
	return nil
}

// ListSnapshots returns available snapshots for the given target ordered newest first.
func ListSnapshots(kind SnapshotKind, uid string) ([]Snapshot, error) {
	dir, err := snapshotDir(kind, uid)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Snapshot{}, nil
		}
		return nil, err
	}
	var snaps []Snapshot
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		name := entry.Name()
		createdAt, shortHash := parseSnapshotFilename(name, info.ModTime())
		snaps = append(snaps, Snapshot{
			ID:        name,
			Kind:      kind,
			UID:       uid,
			CreatedAt: createdAt,
			Size:      info.Size(),
			SHA256:    shortHash,
		})
	}
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].ID > snaps[j].ID
	})
	return snaps, nil
}

func parseSnapshotFilename(name string, fallback time.Time) (time.Time, string) {
	trimmed := strings.TrimSuffix(name, ".yaml")
	lastDash := strings.LastIndex(trimmed, "-")
	if lastDash <= 0 || lastDash >= len(trimmed)-1 {
		return fallback, ""
	}
	tsPart := trimmed[:lastDash]
	hashPart := trimmed[lastDash+1:]
	if ts, err := time.Parse("20060102-150405.000000000", tsPart); err == nil {
		return ts, hashPart
	}
	if ts, err := time.Parse("20060102-150405", tsPart); err == nil {
		return ts, hashPart
	}
	return fallback, hashPart
}

// ReadSnapshot loads the raw bytes of a historical snapshot.
func ReadSnapshot(kind SnapshotKind, uid, id string) ([]byte, error) {
	if err := validateSnapshotID(id); err != nil {
		return nil, err
	}
	dir, err := snapshotDir(kind, uid)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, id))
	if err != nil {
		return nil, fmt.Errorf("read snapshot %s: %w", id, err)
	}
	return data, nil
}

// DeleteRawSnapshots removes all historical snapshots belonging to a deleted profile.
func DeleteRawSnapshots(uid string) error {
	dir, err := snapshotDir(SnapshotRaw, uid)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove raw snapshots: %w", err)
	}
	return nil
}
