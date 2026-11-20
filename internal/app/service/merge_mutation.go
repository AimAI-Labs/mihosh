package service

import (
	"errors"
	"os"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
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
}

func (change mergeMutation) prepare(old []byte, uid, path string, managed bool) (preparedMerge, error) {
	var next preparedMerge
	native, err := profile.NormalizeLegacyMerge(old, uid, func() (string, error) {
		if path == "" {
			return "", os.ErrNotExist
		}
		return path, nil
	})
	if err != nil {
		return next, err
	}
	if change.rule != nil {
		effective, err := readRuleEditBase(uid, path, old, managed)
		if err != nil {
			return next, err
		}
		next.candidate, err = profile.ApplyRuleEditToYAML(effective, *change.rule)
		if err != nil {
			return next, err
		}
		next.data, err = profile.ReplaceMergeRules(native, next.candidate)
		if err != nil {
			return next, err
		}
	} else {
		before, err := profile.ReadMihomoOverridesFrom(native)
		if err != nil {
			return next, err
		}
		next.cleared = profile.ClearedOverrideKeys(before, change.overrides)
		next.data, err = profile.EncodeMihomoOverrides(native, change.overrides)
		if err != nil {
			return next, err
		}
	}
	next.overrides, err = profile.ReadMihomoOverridesFrom(next.data)
	return next, err
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
