package profile

import (
	"errors"
	"fmt"
	"slices"

	domainrules "github.com/AimAI-Labs/mihosh/internal/domain/rules"
	"gopkg.in/yaml.v3"
)

const ruleEditsKey = "rule-edits"

var ErrRuleEditTarget = errors.New("rule was not found in the effective subscription")

// RuleEdit is applied after the standard rule overrides. Match identifies the
// first matching rule. Index is one-based, with zero retaining an edited rule's position.
type RuleEdit = domainrules.Edit

func ApplyRuleEditToYAML(data []byte, edit RuleEdit) ([]byte, error) {
	doc, err := parseOverrideDocument(data)
	if err != nil {
		return nil, err
	}
	if err = applyRuleEdit(topLevelMapping(doc), edit); err != nil {
		return nil, err
	}
	return marshalYAML2Spaces(doc)
}

func applyRuleEdit(root *yaml.Node, edit RuleEdit) error {
	if (edit.Match == "" && edit.Rule == "") || edit.Index < 0 {
		return fmt.Errorf("invalid %s entry", ruleEditsKey)
	}
	list := findMappingValue(root, "rules")
	if list != nil && list.Kind == yaml.AliasNode {
		list = deepCopyNode(list.Alias)
		list.Anchor = ""
		setMappingField(root, "rules", list)
	}
	if list == nil || list.ShortTag() == "!!null" {
		if edit.Match != "" {
			return ErrRuleEditTarget
		}
		list = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		setMappingField(root, "rules", list)
	}
	if list.Kind != yaml.SequenceNode {
		return errors.New("rules must be a YAML sequence")
	}

	index := max(0, edit.Index-1)
	if edit.Match != "" {
		match := canonicalRule(edit.Match)
		matchIndex := slices.IndexFunc(list.Content, func(node *yaml.Node) bool {
			var rule string
			return node.Decode(&rule) == nil && canonicalRule(rule) == match
		})
		if matchIndex < 0 {
			return ErrRuleEditTarget
		}
		list.Content = slices.Delete(list.Content, matchIndex, matchIndex+1)
		if edit.Index == 0 {
			index = matchIndex
		}
	}
	if edit.Rule != "" {
		node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: edit.Rule}
		list.Content = slices.Insert(list.Content, min(index, len(list.Content)), node)
	}
	return nil
}

func canonicalRule(rule string) string {
	return domainrules.Canonicalize(rule)
}
