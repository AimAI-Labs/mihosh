package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// MihomoEndpoint 从 mihomo 配置文件解析出的连接信息（原值，无 scheme）。
// external-controller/secret 与 mihomo 配置文件保持一致；proxy 地址由 MixedPort 派生。
type MihomoEndpoint struct {
	ExternalController string
	Secret             string
	MixedPort          int
}

// DefaultMihomoEndpoint 内置默认（自动发现失败时回退）。
var DefaultMihomoEndpoint = MihomoEndpoint{
	ExternalController: "127.0.0.1:9090",
	MixedPort:          7890,
}

// ResolveMihomoEndpoint 自动发现 mihomo 配置文件并解析连接信息。
// 发现或解析失败时返回 DefaultMihomoEndpoint（不返回 error，保证启动零门槛）。
// secret 统一在此收敛 fallback 逻辑：API 可能省略 secret，从 YAML 读取补全。
func ResolveMihomoEndpoint() MihomoEndpoint {
	path, err := GetMihomoConfigPath()
	if err != nil {
		return DefaultMihomoEndpoint
	}

	data, err := ReadMihomoYAML(path)
	if err != nil {
		return DefaultMihomoEndpoint
	}

	endpoint := DefaultMihomoEndpoint
	if v, ok := data["external-controller"].(string); ok && v != "" {
		endpoint.ExternalController = v
	}
	if v, ok := data["secret"].(string); ok {
		endpoint.Secret = v
	}
	endpoint.MixedPort = resolveMixedPort(data, endpoint.MixedPort)
	return endpoint
}

// resolveMixedPort 从 YAML map 解析 mixed-port，兼容 int/int64/float64 等 YAML 解析类型。
func resolveMixedPort(data map[string]interface{}, fallback int) int {
	switch v := data["mixed-port"].(type) {
	case int:
		if v > 0 {
			return v
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case float64:
		if v > 0 {
			return int(v)
		}
	}
	switch v := data["port"].(type) {
	case int:
		if v > 0 {
			return v
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case float64:
		if v > 0 {
			return int(v)
		}
	}
	return fallback
}

// MixedPortToProxyURL 由 mixed-port 派生 HTTP 代理地址（带 scheme 是 HTTP 协议要求）。
func MixedPortToProxyURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

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

	out, err := marshalYAML(&root)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(configPath, out, 0644)
}
