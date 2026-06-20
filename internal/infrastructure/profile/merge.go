package profile

// merge.go — Clash Verge merge.rs 的 Go 移植。
//
// 输入：raw（订阅原始配置）+ merge（用户覆写 YAML），均为已解析的 yaml.Node。
// 输出：基于 raw 深拷贝后叠加 merge 的最终 yaml.Node。
//
// 算法（对齐 merge.rs::use_merge）：
//   - 六个特殊键（prepend/append × rules/proxies/proxy-groups）做列表拼接；
//   - 其余顶层键直接覆盖（shallow，不递归合并 mapping）；
//   - key 经 strings.ToLower 归一比较（对齐 merge.rs 的 use_lowercase），
//     避免 Prepend-Rules vs prepend-rules 漏匹配；
//   - 目标键不存在时创建（raw 无 rules 时 prepend 等价于赋值）。
//
// 注释安全：基于 yaml.Node 操作，序列化用 2 空格缩进（见 generate.go）。

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// 合并语义类别。
type mergeKind int

const (
	mergeKindPrepend mergeKind = iota // val 拼接到目标列表前面
	mergeKindAppend                   // val 拼接到目标列表后面
	mergeKindReplace                  // 顶层键直接覆盖
)

// classifyMergeKey 根据归一化后的 key 判定合并类别，并返回目标键名。
// prepend-rules → (Prepend, "rules")；非特殊键 → (Replace, 原键名)。
func classifyMergeKey(key string) (mergeKind, string) {
	lower := strings.ToLower(strings.TrimSpace(key))
	switch lower {
	case "prepend-rules":
		return mergeKindPrepend, "rules"
	case "append-rules":
		return mergeKindAppend, "rules"
	case "prepend-proxies":
		return mergeKindPrepend, "proxies"
	case "append-proxies":
		return mergeKindAppend, "proxies"
	case "prepend-proxy-groups":
		return mergeKindPrepend, "proxy-groups"
	case "append-proxy-groups":
		return mergeKindAppend, "proxy-groups"
	default:
		return mergeKindReplace, key
	}
}

// ApplyMerge 把 merge 叠加到 raw 上，返回最终 yaml.Node（深拷贝，不修改入参）。
//
// raw / merge 都应是 YAML 文档的顶层 mapping 节点；若传入 DocumentNode 会自动取首个 mapping。
// 两者为空时返回空 mapping。
func ApplyMerge(raw, merge *yaml.Node) (*yaml.Node, error) {
	rawRoot := topLevelMapping(raw)
	mergeRoot := topLevelMapping(merge)

	// 深拷贝 raw 作为结果基底，避免修改入参。
	result := deepCopyNode(rawRoot)
	if result == nil {
		result = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}

	if mergeRoot == nil || len(mergeRoot.Content) == 0 {
		return result, nil
	}

	// merge 的键值成对出现。
	for i := 0; i+1 < len(mergeRoot.Content); i += 2 {
		keyNode := mergeRoot.Content[i]
		valNode := mergeRoot.Content[i+1]
		if keyNode.Kind != yaml.ScalarNode {
			// 非 scalar 键（极少见）跳过，避免破坏结构。
			continue
		}
		kind, targetKey := classifyMergeKey(keyNode.Value)

		switch kind {
		case mergeKindPrepend, mergeKindAppend:
			merged, err := mergeListField(result, targetKey, valNode, kind == mergeKindPrepend)
			if err != nil {
				return nil, fmt.Errorf("合并 %s 失败: %w", keyNode.Value, err)
			}
			setMappingField(result, targetKey, merged)
		case mergeKindReplace:
			// 顶层键直接覆盖（shallow）：若已存在则替换值节点，否则追加键值对。
			setMappingField(result, targetKey, valNode)
		}
	}

	return result, nil
}

// mergeListField 把 valNode 拼接到 result 中 targetKey 对应的列表节点。
//
//   - prepend=true：result[key] = val + result[key]；
//   - prepend=false：result[key] = result[key] + val；
//   - result 无该键或非 sequence：结果即为 val 的拷贝（若 val 非 sequence 则报错）。
//
// 返回新的 sequence 节点（调用方负责写入 result）。
func mergeListField(result *yaml.Node, targetKey string, valNode *yaml.Node, prepend bool) (*yaml.Node, error) {
	if valNode == nil {
		return nil, fmt.Errorf("覆写值为空")
	}
	if valNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s 必须是列表（YAML 序列），实际为 %s", targetKey, nodeKindName(valNode))
	}

	existing := findMappingValue(result, targetKey)

	// 拷贝 val 的每个子节点，避免与原 merge 文档共享引用。
	valItems := copyNodes(valNode.Content)

	if existing == nil || existing.Kind != yaml.SequenceNode || len(existing.Content) == 0 {
		// 目标列表缺失或为空 → 结果即为 val。
		return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: valItems}, nil
	}

	existingItems := copyNodes(existing.Content)
	var combined []*yaml.Node
	if prepend {
		combined = append(combined, valItems...)
		combined = append(combined, existingItems...)
	} else {
		combined = append(combined, existingItems...)
		combined = append(combined, valItems...)
	}
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: combined}, nil
}

// setMappingField 把 value 写入 result mapping 的 targetKey：
// 存在则替换值；不存在则追加键值对。
func setMappingField(result *yaml.Node, targetKey string, value *yaml.Node) {
	for i := 0; i+1 < len(result.Content); i += 2 {
		k := result.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == targetKey {
			result.Content[i+1] = value
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: targetKey}
	result.Content = append(result.Content, keyNode, value)
}

// findMappingValue 在 mapping 节点中查找 key 对应的值节点；不存在返回 nil。
func findMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		k := mapping.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// topLevelMapping 从 yaml.Node 中取顶层 mapping：若为 DocumentNode 取首个 mapping 子节点；
// 若本身是 MappingNode 直接返回；其余返回 nil。
func topLevelMapping(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.MappingNode {
		return n
	}
	if n.Kind == yaml.DocumentNode {
		for _, c := range n.Content {
			if c.Kind == yaml.MappingNode {
				return c
			}
		}
	}
	return nil
}

// deepCopyNode 通过 marshal/unmarshal 深拷贝节点（保留注释与样式）。
// yaml.v3 的 Encode/Decode 会保留 HeadComment/LineComment/FootComment 等元信息。
func deepCopyNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	out, err := yaml.Marshal(n)
	if err != nil {
		// marshal 失败时退化为浅拷贝（极少触发）。
		c := *n
		return &c
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(out, &doc); err != nil {
		c := *n
		return &c
	}
	// Marshal 总会包一层 DocumentNode。
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return &doc
}

// copyNodes 深拷贝一组节点。
func copyNodes(nodes []*yaml.Node) []*yaml.Node {
	out := make([]*yaml.Node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, deepCopyNode(n))
	}
	return out
}

// nodeKindName 返回节点类型的中文名（用于错误信息）。
func nodeKindName(n *yaml.Node) string {
	switch n.Kind {
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.DocumentNode:
		return "document"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}
