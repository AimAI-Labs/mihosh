package service

import (
	"context"
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
// the active configuration when affected.
func (s *ProfileService) RollbackSnapshot(kind profile.SnapshotKind, uid, snapshotID string) (*ActivateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch kind {
	case profile.SnapshotRaw:
		return s.rollbackRawSnapshot(uid, snapshotID)
	case profile.SnapshotMerge:
		cfg, err := loadCoreConfig()
		if err != nil {
			return nil, err
		}
		active := activeProfile(cfg).UID != ""
		if active && cfg.Core.Mode != "managed" && s.client == nil {
			return nil, errors.New(i18n.T("sub.err.activation_no_client"))
		}
		res := NewConfigService().applyMergeMutation(context.Background(), s.core, s.client, mergeMutation{snapshotID: &snapshotID})
		return &ActivateResult{
			BackupName:    res.BackupName,
			Restarted:     res.Restarted,
			NeedsRestart:  res.NeedsRestart,
			Remote:        res.Remote,
			ConfigWritten: res.WriteOK && !res.Remote,
			CoreApplied:   res.Applied,
			ActiveSaved:   active && res.Err == nil,
		}, res.Err
	default:
		return nil, fmt.Errorf("unsupported snapshot kind: %q", kind)
	}
}

func (s *ProfileService) rollbackRawSnapshot(uid, snapshotID string) (*ActivateResult, error) {
	var before profile.Profile
	var restore func() error
	var active bool
	err := config.WithLockedConfig(func(cfg *config.Config) error {
		var err error
		before, err = profileByUID(cfg, uid)
		if err != nil {
			return err
		}
		active = cfg.ActiveSub == uid
		restore, err = profile.RestoreRawSnapshot(before, snapshotID)
		return err
	})
	if errors.Is(err, profile.ErrStaleRaw) {
		return nil, errors.New(i18n.T("sub.err.snapshot_source_changed"))
	}
	if err != nil {
		return nil, err
	}
	if !active {
		return &ActivateResult{}, nil
	}
	result, err := s.activateLocked(uid, nil)
	if err != nil {
		restoreErr := config.WithLockedConfig(func(cfg *config.Config) error {
			current, readErr := profileByUID(cfg, uid)
			if readErr != nil {
				return readErr
			}
			if !current.Source.SameOrigin(before.Source) {
				return profile.ErrStaleRaw
			}
			return restore()
		})
		return &result, errors.Join(err, restoreErr)
	}
	return &result, nil
}
