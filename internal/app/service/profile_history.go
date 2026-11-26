package service

import (
	"errors"
	"fmt"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

// ListRawSnapshots returns saved historical snapshots of a subscription's raw.yaml (newest first).
func (s *ProfileService) ListRawSnapshots(uid string) ([]profile.Snapshot, error) {
	return profile.ListSnapshots(profile.SnapshotRaw, uid)
}

// ListMergeSnapshots returns saved historical snapshots of global merge.yaml (newest first).
func (s *ProfileService) ListMergeSnapshots() ([]profile.Snapshot, error) {
	return profile.ListSnapshots(profile.SnapshotMerge, "")
}

// DiffSnapshotWithCurrent computes a line-level diff between a historical snapshot and the current file.
func (s *ProfileService) DiffSnapshotWithCurrent(kind profile.SnapshotKind, uid, snapshotID string, opts profile.FileDiffOptions) ([]profile.FileDiffRow, error) {
	before, err := profile.ReadSnapshot(kind, uid, snapshotID)
	if err != nil {
		return nil, err
	}
	var after []byte
	switch kind {
	case profile.SnapshotRaw:
		after, err = profile.ReadRawFile(uid)
		if err != nil {
			return nil, err
		}
	case profile.SnapshotMerge:
		after, err = profile.ReadMerge()
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported snapshot kind: %q", kind)
	}
	return profile.DiffFiles(string(before), string(after), opts), nil
}

// RollbackSnapshot restores a historical raw.yaml or merge.yaml snapshot and re-applies
// the active subscription configuration when affected.
func (s *ProfileService) RollbackSnapshot(kind profile.SnapshotKind, uid, snapshotID string) (*ActivateResult, error) {
	_, activeUID, err := s.ListProfiles()
	if err != nil {
		return nil, err
	}
	switch kind {
	case profile.SnapshotRaw:
		if err := config.Update(func(cfg *config.Config) error {
			p, err := profileByUID(cfg, uid)
			if err != nil {
				return err
			}
			return profile.RestoreRawSnapshot(p, snapshotID)
		}); err != nil {
			if errors.Is(err, profile.ErrStaleRaw) {
				return nil, errors.New(i18n.T("sub.err.snapshot_source_changed"))
			}
			return nil, err
		}
		if activeUID != "" && activeUID == uid {
			res, actErr := s.Activate(uid)
			if actErr != nil {
				return &res, actErr
			}
			return &res, nil
		}
		return &ActivateResult{}, nil
	case profile.SnapshotMerge:
		data, err := profile.ReadSnapshot(kind, uid, snapshotID)
		if err != nil {
			return nil, err
		}
		mergePath, err := profile.MergePath()
		if err != nil {
			return nil, err
		}
		before, beforeExisted, err := readOptionalFile(mergePath)
		if err != nil {
			return nil, err
		}
		beforeOverrides, _ := profile.ReadMihomoOverridesFrom(before)
		afterOverrides, _ := profile.ReadMihomoOverridesFrom(data)
		cleared := profile.ClearedOverrideKeys(beforeOverrides, afterOverrides)
		if err := profile.WriteMerge(data); err != nil {
			return nil, err
		}
		if activeUID != "" {
			res, applyErr := s.ApplyActiveMerge(activeUID, cleared)
			if applyErr != nil {
				restoreErr := restoreMergeFile(mergePath, beforeExisted, before)
				return &res, errors.Join(applyErr, restoreErr)
			}
			return &res, nil
		}
		return &ActivateResult{}, nil
	default:
		return nil, fmt.Errorf("unsupported snapshot kind: %q", kind)
	}
}
