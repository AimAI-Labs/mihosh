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

// MihomoTarget describes one discovered controller and its local editing path.
type MihomoTarget struct {
	Endpoint   MihomoEndpoint
	ConfigPath string
	AllowLan   bool
	LogLevel   string
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
	cfg, _ := Load()
	return ResolveMihomoTarget(cfg).Endpoint
}

// ResolveMihomoTarget uses the caller's app configuration and reads the discovered
// Mihomo file once, so the endpoint and local editing path describe the same target.
func ResolveMihomoTarget(cfg *Config) MihomoTarget {
	target := MihomoTarget{Endpoint: DefaultMihomoEndpoint}
	if cfg != nil && cfg.Core.Mode != "managed" && cfg.Core.ExternalController != "" {
		port := cfg.Core.ExternalMixedPort
		if port <= 0 {
			port = DefaultMihomoEndpoint.MixedPort
		}
		target.Endpoint = MihomoEndpoint{ExternalController: cfg.Core.ExternalController, Secret: cfg.Core.ExternalSecret, MixedPort: port}
		return target
	}
	path, err := resolveMihomoConfigPath(cfg)
	if err != nil {
		target.ConfigPath, _ = GetDefaultMihomoConfigPath()
		return target
	}
	target.ConfigPath = path

	data, err := ReadMihomoYAML(path)
	if err != nil {
		return target
	}

	if v, ok := data["external-controller"].(string); ok && v != "" {
		target.Endpoint.ExternalController = v
	}
	if v, ok := data["secret"].(string); ok {
		target.Endpoint.Secret = v
	}
	target.Endpoint.MixedPort = resolveMixedPort(data, target.Endpoint.MixedPort)
	target.AllowLan, _ = data["allow-lan"].(bool)
	target.LogLevel, _ = data["log-level"].(string)
	return target
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
	release, err := lockMihomoFile(configPath)
	if err != nil {
		return err
	}
	defer release()
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

	target := configPath
	if info, err := os.Lstat(configPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(configPath)
		if err != nil {
			return err
		}
		target = resolved
	}
	return atomicWriteFile(target, out, 0644)
}
