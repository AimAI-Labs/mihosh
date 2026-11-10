package profile

// generate.go — 订阅配置生成与原子写入；备份和激活由调用方编排。

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// 托管字段列表：这些字段从现有 mihomo 配置文件保留，不被订阅覆盖
var managedFields = []string{
	"external-controller",
	"secret",
	"mixed-port",
	"allow-lan",
	"log-level",
}

func managedFieldKeysInNode(n *yaml.Node) []string {
	mapping := topLevelMapping(n)
	if mapping == nil {
		return nil
	}
	keys := make([]string, 0, len(managedFields))
	for _, field := range managedFields {
		if findMappingValue(mapping, field) != nil {
			keys = append(keys, field)
		}
	}
	return keys
}

// MergeManagedFieldOverrideKeys 返回 merge.yaml 中出现的托管字段。
// 这些字段由设置页托管，不支持在覆写配置中设置。
func MergeManagedFieldOverrideKeys() ([]string, error) {
	merge, err := ReadMergeNode()
	if err != nil {
		return nil, err
	}
	return managedFieldKeysInNode(merge), nil
}

// readManagedFieldsFromFile 从现有配置文件读取托管字段
func readManagedFieldsFromFile(path string) (map[string]*yaml.Node, error) {
	managed := make(map[string]*yaml.Node)
	if path == "" {
		return managed, nil
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return managed, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取现有 mihomo 配置失败: %w", err)
	}
	if len(trimSpaceBytes(data)) == 0 {
		return managed, nil
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("解析现有 mihomo 配置失败: %w", err)
	}

	mapping := topLevelMapping(&root)
	if mapping == nil {
		return managed, nil
	}

	for _, field := range managedFields {
		if val := findMappingValue(mapping, field); val != nil {
			managed[field] = deepCopyNode(val)
		}
	}

	return managed, nil
}

// injectManagedFields 向最终配置写回托管字段。
// 这些字段由设置页托管，只要当前 O 中存在，就应始终覆盖 S/M 生成结果。
func injectManagedFields(final *yaml.Node, managed map[string]*yaml.Node) {
	for _, field := range managedFields {
		if val, ok := managed[field]; ok {
			setMappingField(final, field, val)
		}
	}
}

// Generate 合并 raw 与 merge，返回最终 yaml.Node。
// merge 为 nil 时直接返回 raw（深拷贝）。
func Generate(raw, merge *yaml.Node) (*yaml.Node, error) {
	return ApplyMerge(raw, merge)
}

// GenerateWithPreserve 合并 raw 与 merge，同时保留现有配置文件中的托管字段。
// managedPath 为现有 mihomo 配置文件路径。
func GenerateWithPreserve(raw, merge *yaml.Node, managedPath string) (*yaml.Node, error) {
	final, err := ApplyMerge(raw, merge)
	if err != nil {
		return nil, err
	}

	managed, err := readManagedFieldsFromFile(managedPath)
	if err != nil {
		return nil, err
	}
	if len(managed) > 0 {
		injectManagedFields(final, managed)
	}

	return final, nil
}

// GenerateYAML 合并并序列化为 2 空格缩进的 YAML 字节。
func GenerateYAML(raw, merge *yaml.Node) ([]byte, error) {
	final, err := Generate(raw, merge)
	if err != nil {
		return nil, err
	}
	return marshalNode(final)
}

// GenerateYAMLWithPreserve 合并并序列化为 2 空格缩进的 YAML 字节，同时保留现有配置中的托管字段。
func GenerateYAMLWithPreserve(raw, merge *yaml.Node, managedPath string) ([]byte, error) {
	final, err := GenerateWithPreserve(raw, merge, managedPath)
	if err != nil {
		return nil, err
	}
	return marshalNode(final)
}

// marshalNode 以 2 空格缩进序列化 yaml.Node（与 config.marshalYAML 一致）。
func marshalNode(n *yaml.Node) ([]byte, error) {
	// 包一层 DocumentNode 以保证 encoder 正确输出顶层。
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	if n != nil {
		doc.Content = []*yaml.Node{n}
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}
	return buf.Bytes(), nil
}

// WriteFinal 原子写入最终配置到 targetPath（覆盖）。
func WriteFinal(targetPath string, data []byte) error {
	if targetPath == "" {
		return fmt.Errorf("目标配置路径为空")
	}
	return atomicWrite(targetPath, data, 0644)
}

// GeneratedConfig 包含有效配置和已降级忽略的覆写警告。
type GeneratedConfig struct {
	Data     []byte
	MergeErr error
}

// GenerateForUID 读取并生成订阅配置，不写入目标文件。
// managedPath 为空时不保留当前配置字段；致命错误与覆写警告分别返回。
func GenerateForUID(uid, managedPath string) (GeneratedConfig, error) {
	raw, err := ReadRaw(uid)
	if err != nil {
		return GeneratedConfig{}, err
	}
	merge, mergeErr := ReadMergeNode()
	if mergeErr != nil {
		merge = nil // 降级：忽略覆写
	}
	data, err := GenerateYAMLWithPreserve(raw, merge, managedPath)
	if err != nil {
		return GeneratedConfig{}, err
	}
	return GeneratedConfig{Data: data, MergeErr: mergeErr}, nil
}
