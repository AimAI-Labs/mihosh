package profile

// generate.go — 订阅配置生成与原子写入；备份和激活由调用方编排。

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// 托管字段列表：merge.yaml 中显式覆写时优先使用覆写值，未覆写时保留现有 mihomo 配置中的对应字段
var managedFields = []string{
	"external-controller",
	"secret",
	"mixed-port",
	"allow-lan",
	"log-level",
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

// GenerateWithPreserve 合并 raw 与 merge，同时保留现有配置文件中未被 merge 显式覆写的托管字段。
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
	for _, field := range explicitManagedFields(merge) {
		delete(managed, field)
	}
	if len(managed) > 0 {
		injectManagedFields(final, managed)
	}

	return final, nil
}

func explicitManagedFields(merge *yaml.Node) []string {
	if merge == nil {
		return nil
	}
	root := topLevelMapping(merge)
	if root == nil {
		return nil
	}
	var fields []string
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Kind != yaml.ScalarNode || root.Content[i+1].ShortTag() == "!!null" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(root.Content[i].Value))
		for _, field := range managedFields {
			if key == field {
				fields = append(fields, field)
			}
		}
	}
	return fields
}

// GenerateOverrideConfig uses the selected source and falls back to the current
// file for the controller identity. Cleared fields fall through to the source.
func GenerateOverrideConfig(uid, path string, cleared []string, managed bool) ([]byte, error) {
	mergeBytes, err := ReadMerge()
	if err != nil {
		return nil, err
	}
	return GenerateOverrideConfigWithMerge(uid, path, mergeBytes, cleared, managed)
}

// GenerateOverrideConfigWithMerge generates candidate config bytes using mergeBytes
// without requiring merge.yaml to be written first.
func GenerateOverrideConfigWithMerge(uid, path string, mergeBytes []byte, cleared []string, managed bool) ([]byte, error) {
	var source []byte
	var err error
	if uid != "" {
		source, err = ReadRawFile(uid)
	} else {
		source, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	var raw yaml.Node
	if len(trimSpaceBytes(source)) > 0 {
		if err = yaml.Unmarshal(source, &raw); err != nil {
			return nil, fmt.Errorf("解析配置源文件失败: %w", err)
		}
	}
	if uid == "" && len(cleared) > 0 {
		if root := topLevelMapping(&raw); root != nil {
			for i := len(root.Content) - 2; i >= 0; i -= 2 {
				for _, key := range cleared {
					if strings.EqualFold(root.Content[i].Value, key) {
						root.Content = append(root.Content[:i], root.Content[i+2:]...)
						break
					}
				}
			}
		}
	}
	var merge *yaml.Node
	if len(trimSpaceBytes(mergeBytes)) > 0 {
		var mergeDoc yaml.Node
		if err = yaml.Unmarshal(mergeBytes, &mergeDoc); err != nil {
			return nil, fmt.Errorf("解析覆写配置失败: %w", err)
		}
		merge = &mergeDoc
	}
	final, err := ApplyMerge(&raw, merge)
	if err != nil {
		return nil, err
	}
	fields, err := readManagedFieldsFromFile(path)
	if err != nil {
		return nil, err
	}
	delete(fields, "log-level")
	delete(fields, "allow-lan")
	if managed && uid != "" {
		if fields == nil {
			fields = make(map[string]*yaml.Node)
		}
		fields["allow-lan"] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
	}
	for _, field := range explicitManagedFields(merge) {
		delete(fields, field)
	}
	injectManagedFields(final, fields)
	return marshalNode(final)
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

// WriteFinal 原子写入最终配置到 targetPath（覆盖，保留符号链接目标）。
func WriteFinal(targetPath string, data []byte) error {
	if targetPath == "" {
		return fmt.Errorf("目标配置路径为空")
	}
	target := targetPath
	if info, err := os.Lstat(targetPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(targetPath)
		if err != nil {
			return err
		}
		target = resolved
	}
	return atomicWrite(target, data, 0644)
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
	return generateFromRaw(raw, managedPath, nil, false)
}

// GenerateForUIDWithCleared 在 GenerateForUID 基础上处理被从 merge.yaml 清空的字段。
func GenerateForUIDWithCleared(uid, managedPath string, cleared []string, managed bool) (GeneratedConfig, error) {
	raw, err := ReadRaw(uid)
	if err != nil {
		return GeneratedConfig{}, err
	}
	return generateFromRaw(raw, managedPath, cleared, managed)
}

// GenerateSourcePreview merges the exact source bytes shown in the file viewer.
func GenerateSourcePreview(source []byte) (GeneratedConfig, error) {
	var raw yaml.Node
	if err := yaml.Unmarshal(source, &raw); err != nil {
		return GeneratedConfig{}, err
	}
	return generateFromRaw(&raw, "", nil, false)
}

func generateFromRaw(raw *yaml.Node, managedPath string, cleared []string, managed bool) (GeneratedConfig, error) {
	merge, mergeErr := ReadMergeNode()
	if mergeErr != nil {
		merge = nil // 降级：忽略覆写
	}
	final, err := ApplyMerge(raw, merge)
	if err != nil {
		return GeneratedConfig{}, err
	}
	fields, err := readManagedFieldsFromFile(managedPath)
	if err != nil {
		return GeneratedConfig{}, err
	}
	for _, key := range cleared {
		if key == "allow-lan" || key == "log-level" {
			delete(fields, key)
		}
	}
	if managed && slices.Contains(cleared, "allow-lan") {
		if fields == nil {
			fields = make(map[string]*yaml.Node)
		}
		fields["allow-lan"] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
	}
	for _, field := range explicitManagedFields(merge) {
		delete(fields, field)
	}
	if len(fields) > 0 {
		injectManagedFields(final, fields)
	}
	data, err := marshalNode(final)
	if err != nil {
		return GeneratedConfig{}, err
	}
	return GeneratedConfig{Data: data, MergeErr: mergeErr}, nil
}
