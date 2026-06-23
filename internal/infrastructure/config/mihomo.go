package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// ReadMihomoYAML reads the mihomo config file into a map, primarily to fetch missing fields like secret
func ReadMihomoYAML(configPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// WriteMihomoField modifies a specific field in the mihomo config file while preserving comments and structure
func WriteMihomoField(configPath, key string, value interface{}) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("")
		} else {
			return err
		}
	}

	var root yaml.Node
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &root); err != nil {
			return err
		}
	}

	if len(root.Content) == 0 {
		root = yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{
				{Kind: yaml.MappingNode},
			},
		}
	}

	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		mapping = &yaml.Node{Kind: yaml.MappingNode}
		root.Content[0] = mapping
	}

	found := false
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			// Update existing
			node := yaml.Node{}
			if err := node.Encode(value); err != nil {
				return err
			}
			mapping.Content[i+1] = &node
			found = true
			break
		}
	}

	if !found {
		// Append new key-value pair
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
		valNode := &yaml.Node{}
		if err := valNode.Encode(value); err != nil {
			return err
		}
		mapping.Content = append(mapping.Content, keyNode, valNode)
	}

	out, err := yaml.Marshal(&root)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, out, 0644)
}
