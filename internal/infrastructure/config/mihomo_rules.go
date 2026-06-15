package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// mihomoRulesKey 是 Mihomo 配置中规则列表的顶层键名。
const mihomoRulesKey = "rules"

// InsertRule 将一条规则字符串（格式如 "DOMAIN-SUFFIX,example.com,DIRECT"）
// 按 index（1-based）插入到 configPath 指向的 Mihomo 配置文件的 rules 列表中。
//
// 行为约定：
//   - index < 1 视为 1（插入到列表顶部，最高优先级）；
//   - index 超出当前规则数量时追加到列表末尾；
//   - 若配置不存在 rules 键，则在顶层 mapping 末尾创建该键。
//
// 注释安全：使用 yaml.Node 而非 struct marshal，保留原有注释、键顺序与缩进。
// 写入采用「临时文件 + Rename」原子替换，避免半写损坏用户配置。
func InsertRule(configPath, rule string, index int) error {
	if rule == "" {
		return fmt.Errorf("规则内容不能为空")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var doc yaml.Node
	// 空文件：构造一个空 mapping 文档
	if len(bytesTrimSpace(data)) == 0 {
		doc = yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{
				{Kind: yaml.MappingNode, Tag: mapTagPlain},
			},
		}
	} else if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	root := mappingRoot(&doc)
	if root == nil {
		return fmt.Errorf("配置文件顶层不是 mapping，无法写入 rules")
	}

	seq := findRulesSequence(root)
	if seq == nil {
		// 创建 rules 键（追加到顶层 mapping 末尾）
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: strTagPlain, Value: mihomoRulesKey}
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: seqTagPlain}
		root.Content = append(root.Content, keyNode, seq)
	}

	insertAt := normalizeInsertIndex(index, len(seq.Content))
	newItem := &yaml.Node{Kind: yaml.ScalarNode, Tag: strTagPlain, Value: rule}

	// 在 seq.Content 的 insertAt 位置插入 newItem
	seq.Content = append(seq.Content, nil)
	copy(seq.Content[insertAt+1:], seq.Content[insertAt:])
	seq.Content[insertAt] = newItem

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
	}

	return atomicWriteFile(configPath, out, 0644)
}

// CountRules 返回配置文件中 rules 列表的条目数；rules 键不存在时返回 0。
func CountRules(configPath string) (int, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0, fmt.Errorf("读取配置文件失败: %w", err)
	}
	if len(bytesTrimSpace(data)) == 0 {
		return 0, nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return 0, fmt.Errorf("解析配置文件失败: %w", err)
	}
	root := mappingRoot(&doc)
	if root == nil {
		return 0, nil
	}
	seq := findRulesSequence(root)
	if seq == nil {
		return 0, nil
	}
	return len(seq.Content), nil
}

// builtinPolicies 是 Mihomo 始终可用的内置策略，列在提取结果最前。
var builtinPolicies = []string{"DIRECT", "REJECT"}

// ExtractPolicies 从配置文件中提取可用的策略/代理名称列表。
//
// 收集顺序（去重）：
//  1. 内置策略 DIRECT、REJECT（始终在前）；
//  2. proxy-groups 下每个组的 name；
//  3. proxies 下每个代理的 name。
//
// 配置文件不存在 / 解析失败 / 无 proxy-groups 与 proxies 时，仅返回内置策略。
func ExtractPolicies(configPath string) ([]string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// 文件不可读时降级为仅内置策略，保证调用方总有可用列表
		return append([]string(nil), builtinPolicies...), nil
	}

	result := append([]string(nil), builtinPolicies...)
	seen := make(map[string]struct{}, len(result))
	for _, p := range result {
		seen[p] = struct{}{}
	}

	if len(bytesTrimSpace(data)) == 0 {
		return result, nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		// 解析失败：仅内置策略，不报错（避免阻塞表单）
		return result, nil
	}
	root := mappingRoot(&doc)
	if root == nil {
		return result, nil
	}

	// proxy-groups 优先（用户在规则中通常引用组）
	appendUnique(findNamedSequence(root, "proxy-groups"), &result, &seen)
	appendUnique(findNamedSequence(root, "proxies"), &result, &seen)

	return result, nil
}

// findNamedSequence 在 mapping 根节点中查找名为 key 的 SequenceNode。
// 返回其中每个子项（应为 MappingNode）的 name 字段值。
func findNamedSequence(root *yaml.Node, key string) []*yaml.Node {
	if root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		k := root.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			val := root.Content[i+1]
			if val.Kind == yaml.SequenceNode {
				return val.Content
			}
			return nil
		}
	}
	return nil
}

// appendUnique 把 items 中每个子项的 name 字段（去重）追加到 result。
// items 为 proxy-groups/proxies 序列的子节点（每个是含 name 键的 MappingNode）。
func appendUnique(items []*yaml.Node, result *[]string, seen *map[string]struct{}) {
	for _, item := range items {
		if item.Kind != yaml.MappingNode {
			continue
		}
		name := mapStringField(item, "name")
		if name == "" {
			continue
		}
		if _, ok := (*seen)[name]; ok {
			continue
		}
		(*seen)[name] = struct{}{}
		*result = append(*result, name)
	}
}

// mapStringField 在 mapping 节点中查找指定字符串键的标量值。
func mapStringField(node *yaml.Node, key string) string {
	if node.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			v := node.Content[i+1]
			if v.Kind == yaml.ScalarNode {
				return v.Value
			}
			return ""
		}
	}
	return ""
}

// findRulesSequence 在 mapping 根节点中查找名为 rules 的 SequenceNode；找不到返回 nil。
func findRulesSequence(root *yaml.Node) *yaml.Node {
	if root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		if key.Kind == yaml.ScalarNode && key.Value == mihomoRulesKey {
			val := root.Content[i+1]
			if val.Kind == yaml.SequenceNode {
				return val
			}
			if val.Kind == yaml.ScalarNode && val.Value == "" && val.Tag == nullTagPlain {
				// 规则为空值（rules:），提升为空序列以便插入
				val.Kind = yaml.SequenceNode
				val.Tag = seqTagPlain
				val.Value = ""
				return val
			}
			return nil
		}
	}
	return nil
}

// mappingRoot 取文档节点的首个 mapping 子节点（Mihomo 配置的顶层）。
func mappingRoot(doc *yaml.Node) *yaml.Node {
	if doc == nil {
		return nil
	}
	if doc.Kind == yaml.MappingNode {
		return doc
	}
	if doc.Kind == yaml.DocumentNode {
		for _, c := range doc.Content {
			if c.Kind == yaml.MappingNode {
				return c
			}
		}
	}
	return nil
}

// normalizeInsertIndex 把 1-based index 规范化为 0-based 切片插入位置。
func normalizeInsertIndex(index, length int) int {
	if index < 1 {
		return 0
	}
	if index > length {
		return length
	}
	return index - 1
}

// atomicWriteFile 先写入同目录临时文件再 Rename 覆盖目标，降低半写损坏风险。
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".mihosh-config-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // Rename 成功后已不存在，忽略错误

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("同步临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("设置临时文件权限失败: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("替换配置文件失败: %w", err)
	}
	return nil
}

// 下面是少量内部常量与工具，避免在调用点散布 yaml 内部细节。
const (
	strTagPlain  = "!!str"
	seqTagPlain  = "!!seq"
	mapTagPlain  = "!!map"
	nullTagPlain = "!!null"
)

func bytesTrimSpace(b []byte) []byte {
	for len(b) > 0 && isSpaceByte(b[0]) {
		b = b[1:]
	}
	for len(b) > 0 && isSpaceByte(b[len(b)-1]) {
		b = b[:len(b)-1]
	}
	return b
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
