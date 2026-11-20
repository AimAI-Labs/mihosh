package profile

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RuleEdits contains Mihosh's rule changes independently of native merge.yaml.
type RuleEdits struct {
	Legacy yaml.Node  `yaml:"legacy,omitempty"`
	Edits  []RuleEdit `yaml:"edits,omitempty"`
}

func RuleEditsPath() (string, error) {
	root, err := mihoshConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "rule-edits.yaml"), nil
}

func DecodeRuleEdits(data []byte) (RuleEdits, error) {
	var edits RuleEdits
	if err := yaml.Unmarshal(data, &edits); err != nil {
		return edits, fmt.Errorf("read rule-edits.yaml: %w", err)
	}
	if edits.Legacy.Kind != 0 {
		if edits.Legacy.Kind != yaml.MappingNode {
			return edits, errors.New("legacy rule edits must be a YAML mapping")
		}
		for i := 0; i < len(edits.Legacy.Content); i += 2 {
			if legacyMergeTarget(edits.Legacy.Content[i].Value) != "rules" {
				return edits, errors.New("legacy rule edits may contain only rule directives")
			}
		}
	}
	return edits, nil
}

func legacyRuleOverrides(root *yaml.Node) (bool, *yaml.Node) {
	overlay := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if root == nil {
		return false, overlay
	}
	// A native rules field in the original file superseded earlier list directives.
	start := 0
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "rules" {
			start = i
		}
	}
	found := false
	for i := 0; i < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		if legacyMergeTarget(key.Value) == "rules" {
			found = true
			kind, _ := classifyMergeKey(key.Value)
			if kind == mergeKindReplace || i >= start {
				overlay.Content = append(overlay.Content, key, value)
			}
		}
	}
	return found, overlay
}

// PreserveLegacyRuleEdits stages old rule directives before merge.yaml is
// converted. They are replayed from this file once the old fields are removed.
func PreserveLegacyRuleEdits(previous []byte) error {
	doc, err := parseOverrideDocument(previous)
	if err != nil {
		return nil // An invalid draft can still be replaced with valid YAML.
	}
	found, legacy := legacyRuleOverrides(topLevelMapping(doc))
	if !found {
		return nil
	}
	path, err := RuleEditsPath()
	if err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	edits, err := DecodeRuleEdits(current)
	if err != nil {
		return err
	}
	edits.Legacy = *legacy
	data, err := marshalYAML2Spaces(edits)
	if err != nil {
		return err
	}
	if bytes.Equal(current, data) {
		return nil
	}
	return atomicWrite(path, data, 0600)
}

func applyProfileMerge(raw, merge *yaml.Node, replay bool) (*yaml.Node, error) {
	final, err := applyMerge(raw, merge, replay)
	if err != nil || !replay {
		return final, err
	}
	path, err := RuleEditsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return final, nil
	}
	if err != nil {
		return nil, err
	}
	edits, err := DecodeRuleEdits(data)
	if err != nil {
		return nil, err
	}
	if legacy, _ := legacyRuleOverrides(topLevelMapping(merge)); !legacy && edits.Legacy.Kind != 0 {
		final, err = ApplyMerge(final, &edits.Legacy)
		if err != nil {
			return nil, err
		}
	}
	for _, edit := range edits.Edits {
		if err := applyRuleEdit(final, edit); err != nil && !errors.Is(err, ErrRuleEditTarget) {
			return nil, err
		}
	}
	return final, nil
}
