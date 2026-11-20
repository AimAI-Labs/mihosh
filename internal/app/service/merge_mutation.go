package service

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"gopkg.in/yaml.v3"
)

type mergeMutation struct {
	overrides map[string]string
	rule      *profile.RuleEdit
}

type preparedMerge struct {
	data      []byte
	overrides map[string]string
	cleared   []string
	candidate []byte
	rulePath  string
	ruleEdits *profileFileRecovery
}

func (change mergeMutation) prepare(old []byte, uid, path string, managed bool) (preparedMerge, error) {
	var next preparedMerge
	if change.rule != nil {
		return change.prepareRuleEdit(old, uid, path, managed)
	}
	native, err := profile.NormalizeLegacyMerge(old, uid, func() (string, error) {
		if path == "" {
			return "", os.ErrNotExist
		}
		return path, nil
	})
	if err != nil {
		return next, err
	}
	before, err := profile.ReadMihomoOverridesFrom(native)
	if err != nil {
		return next, err
	}
	next.cleared = profile.ClearedOverrideKeys(before, change.overrides)
	next.data, err = profile.EncodeMihomoOverrides(native, change.overrides)
	if err != nil {
		return next, err
	}
	next.overrides, err = profile.ReadMihomoOverridesFrom(next.data)
	if err == nil {
		err = profile.PreserveLegacyRuleEdits(old)
	}
	return next, err
}

func (change mergeMutation) prepareRuleEdit(merge []byte, uid, path string, managed bool) (preparedMerge, error) {
	var next preparedMerge
	effective, err := readRuleEditBase(uid, path, merge, managed)
	if err != nil {
		return next, err
	}
	next.candidate, err = profile.ApplyRuleEditToYAML(effective, *change.rule)
	if err != nil {
		return next, err
	}
	edit := *change.rule
	if uid != "" && path != "" && edit.Match != "" && edit.Rule != "" {
		base, err := profile.GenerateOverrideConfigWithMerge(uid, path, merge, nil, managed)
		if err != nil {
			return next, err
		}
		if _, err := profile.ApplyRuleEditToYAML(base, edit); errors.Is(err, profile.ErrRuleEditTarget) {
			// An older runtime-only rule needs an addition to survive reactivation.
			edit.Match = ""
		} else if err != nil {
			return next, err
		}
	}
	next.rulePath, err = profile.RuleEditsPath()
	if err != nil {
		return next, err
	}
	previous, existed, err := readOptionalFile(next.rulePath)
	if err != nil {
		return next, err
	}
	edits, err := profile.DecodeRuleEdits(previous)
	if err != nil {
		return next, err
	}
	edits.Edits = append(edits.Edits, edit)
	next.data, err = yaml.Marshal(edits)
	if err != nil {
		return next, err
	}
	next.ruleEdits = &profileFileRecovery{Data: previous, Existed: existed}
	next.overrides, err = profile.ReadMihomoOverridesFrom(merge)
	return next, err
}

func (next preparedMerge) write() error {
	if next.ruleEdits != nil {
		if err := os.MkdirAll(filepath.Dir(next.rulePath), 0700); err != nil {
			return err
		}
		return privateWrite(next.rulePath, next.data)
	}
	return profile.WriteMerge(next.data)
}

func (next preparedMerge) restore(mergePath string, mergeExisted bool, oldMerge []byte) error {
	if restore := next.ruleEdits; restore != nil {
		return restoreMergeFile(next.rulePath, restore.Existed, restore.Data)
	}
	return restoreMergeFile(mergePath, mergeExisted, oldMerge)
}

func (next preparedMerge) generate(uid, path string, managed bool) ([]byte, error) {
	if next.candidate != nil {
		return next.candidate, nil
	}
	return profile.GenerateOverrideConfigWithMerge(uid, path, next.data, next.cleared, managed)
}

func readRuleEditBase(uid, path string, old []byte, managed bool) ([]byte, error) {
	if path != "" {
		// Keep runtime-only rules when editing a rule displayed by the local core.
		effective, err := os.ReadFile(path)
		if err == nil {
			return effective, nil
		}
		if !os.IsNotExist(err) || uid == "" {
			return nil, err
		}
	}
	return profile.GenerateOverrideConfigWithMerge(uid, path, old, nil, managed)
}

func (change mergeMutation) controlRequest() coreControlRequest {
	if change.rule == nil {
		return coreControlRequest{Action: "overrides", Overrides: change.overrides}
	}
	return coreControlRequest{Action: "rule-edit", RuleEdit: change.rule}
}

var ErrRemoteRuleEditRequiresSubscription = errors.New("a remote controller needs an active subscription to apply rule edits")
