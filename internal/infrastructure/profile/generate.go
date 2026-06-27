package profile

// generate.go — 生成最终配置 + 备份 + 写入。
//
// 流程（对应 spec §1 数据流的第 3~5 步）：
//   1. ApplyMerge(raw, merge) → final yaml.Node；
//   2. 序列化为 2 空格缩进 YAML；
//   3. （备份与写入由 GenerateAndWrite 完成，备份策略见 backup.go）
//
// 本文件只负责「合并 → 序列化 → 写入目标文件」。
// 备份与 rotate 在 backup.go。

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
	if !FileExists(path) {
		return managed, nil
	}

	data, err := os.ReadFile(path)
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
	return atomicWrite(targetPath, data)
}

// GenerateAndWriteForUID 针对某订阅完成「读 raw → 读 merge → 合并 → 序列化」。
// 返回最终 YAML 字节。不写入目标文件（写入与备份由 service 层编排）。
//
// raw.yaml 不存在时返回 ErrRawNotFound。
func GenerateAndWriteForUID(uid string) ([]byte, error) {
	raw, err := ReadRaw(uid)
	if err != nil {
		return nil, err
	}
	merge, err := ReadMergeNode()
	if err != nil {
		// merge 语法错误：跳过覆写，调用方决定是否警告。
		// 此处返回错误以便上层给出明确提示；上层可捕获后用 nil merge 重试。
		return nil, err
	}
	return GenerateYAML(raw, merge)
}

// GenerateAndWriteForUIDIgnoreMergeError 与 GenerateAndWriteForUID 类似，
// 但当 merge 解析失败时忽略覆写（用空 merge），返回最终字节与一个 merge 错误（可空）。
// 供 Activate 流程在 merge 损坏时降级使用。
//
// 返回约定（调用方 Activate 用 len(data)==0 判定致命失败）：
//   - raw 不存在或序列化失败：返回 (nil, _)（视为致命，data 为空）；
//   - 仅 merge 解析失败：返回 (data, mergeErr)（data 为用空 merge 生成的字节，mergeErr 用于警告）；
//   - 全部成功：返回 (data, nil)。
func GenerateAndWriteForUIDIgnoreMergeError(uid string) (data []byte, mergeErr error) {
	raw, err := ReadRaw(uid)
	if err != nil {
		return nil, nil
	}
	merge, mergeErr := ReadMergeNode()
	if mergeErr != nil {
		merge = nil // 降级：忽略覆写
	}
	data, err = GenerateYAML(raw, merge)
	if err != nil {
		return nil, mergeErr
	}
	return data, mergeErr
}

// GenerateAndWriteForUIDIgnoreMergeErrorWithPreserve 与 GenerateAndWriteForUIDIgnoreMergeError 类似，
// 但会从现有 mihomo 配置文件中保留托管字段，避免它们被订阅覆盖。
func GenerateAndWriteForUIDIgnoreMergeErrorWithPreserve(uid string, managedPath string) (data []byte, mergeErr error) {
	raw, err := ReadRaw(uid)
	if err != nil {
		return nil, nil
	}
	merge, mergeErr := ReadMergeNode()
	if mergeErr != nil {
		merge = nil // 降级：忽略覆写
	}
	data, err = GenerateYAMLWithPreserve(raw, merge, managedPath)
	if err != nil {
		return nil, mergeErr
	}
	return data, mergeErr
}

// FileExists 判断路径是否存在（文件或目录）。
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
