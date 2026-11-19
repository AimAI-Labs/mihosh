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
}

func (change mergeMutation) prepare(old []byte, uid, path string, managed bool) (preparedMerge, error) {
	var next preparedMerge
	var err error
	if change.rule != nil {
		persisted := *change.rule
		if uid != "" && path != "" && persisted.Match != "" && persisted.Rule != "" {
			// Older versions only wrote custom rules to the runtime file. If the
			// old rule isn't reproducible from the subscription, persist its edit as an addition.
			base, genErr := profile.GenerateOverrideConfigWithMerge(uid, path, old, nil, managed)
			if genErr != nil {
				return next, genErr
			}
			if _, editErr := profile.ApplyRuleEditToYAML(base, persisted); errors.Is(editErr, profile.ErrRuleEditTarget) {
				persisted.Match = ""
			} else if editErr != nil {
				return next, editErr
			}
		}
		next.data, err = profile.AppendRuleEdit(old, persisted)
	} else {
		before, err := profile.ReadMihomoOverridesFrom(old)
		if err != nil {
			return next, err
		}
		next.cleared = profile.ClearedOverrideKeys(before, change.overrides)
		next.data, err = profile.EncodeMihomoOverrides(old, change.overrides)
		if err != nil {
			return next, err
		}
	}
	if err != nil {
		return next, err
	}
	next.overrides, err = profile.ReadMihomoOverridesFrom(next.data)
	return next, err
}

func (change mergeMutation) candidate(uid, path string, old []byte, next preparedMerge, managed bool) ([]byte, error) {
	if change.rule == nil {
		return profile.GenerateOverrideConfigWithMerge(uid, path, next.data, next.cleared, managed)
	}
	if path != "" {
		// Keep the rest of the live rule list intact when changing one rule,
		// including legacy local rules and other runtime-only edits.
		effective, err := os.ReadFile(path)
		if err == nil {
			return profile.ApplyRuleEditToYAML(effective, *change.rule)
		}
		if !os.IsNotExist(err) || uid == "" {
			return nil, err
		}
	}
	effective, err := profile.GenerateOverrideConfigWithMerge(uid, path, old, nil, managed)
	if err != nil {
		return nil, err
	}
	return profile.ApplyRuleEditToYAML(effective, *change.rule)
}

func (change mergeMutation) controlRequest() coreControlRequest {
	if change.rule == nil {
		return coreControlRequest{Action: "overrides", Overrides: change.overrides}
	}
	return coreControlRequest{Action: "rule-edit", RuleEdit: change.rule}
}

var ErrRemoteRuleEditRequiresSubscription = errors.New("a remote controller needs an active subscription to apply rule edits")
