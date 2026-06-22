package profile

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// parseMapping 解析 YAML 字符串为顶层 mapping 节点。
func parseMapping(t *testing.T, s string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(s), &doc))
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return &doc
}

// parseDoc 解析 YAML 字符串为 DocumentNode（含文档层）。
func parseDoc(t *testing.T, s string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(s), &doc))
	return &doc
}

// mappingToMap 把 mapping 节点的顶层键值转成 map[string]string（仅用于断言标量顶层）。
// sequence 值会序列化回字符串便于比较。
func mappingToMap(t *testing.T, n *yaml.Node) map[string]string {
	t.Helper()
	root := topLevelMapping(n)
	out := map[string]string{}
	if root == nil {
		return out
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		k := root.Content[i].Value
		v := root.Content[i+1]
		if v.Kind == yaml.ScalarNode {
			out[k] = v.Value
		} else {
			b, _ := yaml.Marshal(v)
			out[k] = strings.TrimSpace(string(b))
		}
	}
	return out
}

// seqValues 取某键对应的 sequence 的所有标量值。
func seqValues(t *testing.T, n *yaml.Node, key string) []string {
	t.Helper()
	root := topLevelMapping(n)
	require.NotNil(t, root)
	val := findMappingValue(root, key)
	require.NotNil(t, val, "key %s 不存在", key)
	require.Equal(t, yaml.SequenceNode, val.Kind, "key %s 不是 sequence", key)
	var out []string
	for _, c := range val.Content {
		out = append(out, c.Value)
	}
	return out
}

func TestClassifyMergeKey(t *testing.T) {
	cases := []struct {
		in       string
		wantKind mergeKind
		wantKey  string
	}{
		{"prepend-rules", mergeKindPrepend, "rules"},
		{"Prepend-Rules", mergeKindPrepend, "rules"}, // 大小写归一
		{"APPEND-PROXIES", mergeKindAppend, "proxies"},
		{"prepend-proxy-groups", mergeKindPrepend, "proxy-groups"},
		{"append-proxy-groups", mergeKindAppend, "proxy-groups"},
		{"dns", mergeKindReplace, "dns"},
		{"mode", mergeKindReplace, "mode"},
	}
	for _, c := range cases {
		k, key := classifyMergeKey(c.in)
		assert.Equal(t, c.wantKind, k, "key=%s kind", c.in)
		if c.wantKind == mergeKindReplace {
			assert.Equal(t, c.wantKey, key, "key=%s target", c.in)
		} else {
			assert.Equal(t, c.wantKey, key, "key=%s target", c.in)
		}
	}
}

func TestApplyMerge_PrependRules(t *testing.T) {
	raw := parseDoc(t, `rules:
  - DOMAIN,old.com,DIRECT
  - MATCH,REJECT
`)
	merge := parseDoc(t, `prepend-rules:
  - DOMAIN,new.com,DIRECT
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"DOMAIN,new.com,DIRECT",
		"DOMAIN,old.com,DIRECT",
		"MATCH,REJECT",
	}, seqValues(t, final, "rules"))
}

func TestApplyMerge_AppendRules(t *testing.T) {
	raw := parseDoc(t, `rules:
  - DOMAIN,old.com,DIRECT
`)
	merge := parseDoc(t, `append-rules:
  - MATCH,DIRECT
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"DOMAIN,old.com,DIRECT",
		"MATCH,DIRECT",
	}, seqValues(t, final, "rules"))
}

func TestApplyMerge_PrependProxies(t *testing.T) {
	raw := parseDoc(t, `proxies:
  - name: existing
    type: ss
`)
	merge := parseDoc(t, `prepend-proxies:
  - name: custom
    type: ss
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	seq := findMappingValue(topLevelMapping(final), "proxies")
	require.NotNil(t, seq)
	require.Len(t, seq.Content, 2)
	// 第一个应为 custom（前置）
	firstName := findMappingValue(seq.Content[0], "name")
	assert.Equal(t, "custom", firstName.Value)
	secondName := findMappingValue(seq.Content[1], "name")
	assert.Equal(t, "existing", secondName.Value)
}

func TestApplyMerge_PrependProxyGroups(t *testing.T) {
	raw := parseDoc(t, `proxy-groups:
  - name: G1
`)
	merge := parseDoc(t, `prepend-proxy-groups:
  - name: G0
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	seq := findMappingValue(topLevelMapping(final), "proxy-groups")
	require.Len(t, seq.Content, 2)
	assert.Equal(t, "G0", findMappingValue(seq.Content[0], "name").Value)
	assert.Equal(t, "G1", findMappingValue(seq.Content[1], "name").Value)
}

func TestApplyMerge_AppendProxyGroups(t *testing.T) {
	raw := parseDoc(t, `proxy-groups:
  - name: G1
`)
	merge := parseDoc(t, `append-proxy-groups:
  - name: G2
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	seq := findMappingValue(topLevelMapping(final), "proxy-groups")
	require.Len(t, seq.Content, 2)
	assert.Equal(t, "G1", findMappingValue(seq.Content[0], "name").Value)
	assert.Equal(t, "G2", findMappingValue(seq.Content[1], "name").Value)
}

func TestApplyMerge_TopLevelReplace(t *testing.T) {
	raw := parseDoc(t, `mode: global
dns:
  enable: false
`)
	merge := parseDoc(t, `mode: rule
tun:
  enable: true
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	m := mappingToMap(t, final)
	assert.Equal(t, "rule", m["mode"], "顶层标量应被覆盖")
	assert.Contains(t, m["tun"], "enable: true", "merge 新增的顶层键应出现")
	assert.Contains(t, m["dns"], "enable: false", "raw 原有顶层键应保留")
}

func TestApplyMerge_TopLevelReplaceMapping_Deep(t *testing.T) {
	// 对齐 merge.rs::deep_merge：双方都为 mapping 时递归合并子键。
	// raw 的 dns.nameserver 应保留，merge 的 dns.enable 覆盖 raw。
	raw := parseDoc(t, `dns:
  enable: true
  nameserver:
    - 8.8.8.8
`)
	merge := parseDoc(t, `dns:
  enable: false
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	dns := findMappingValue(topLevelMapping(final), "dns")
	require.NotNil(t, dns)
	// deep：merge 的 enable 覆盖 raw，且 raw 的 nameserver 应保留
	assert.Equal(t, "false", findMappingValue(dns, "enable").Value, "同名 scalar 子键应被 merge 覆盖")
	ns := findMappingValue(dns, "nameserver")
	require.NotNil(t, ns, "递归合并应保留 raw 的子键 nameserver")
	require.Equal(t, yaml.SequenceNode, ns.Kind)
	require.Len(t, ns.Content, 1)
	assert.Equal(t, "8.8.8.8", ns.Content[0].Value)
}

func TestApplyMerge_DeepMergeNestedMapping(t *testing.T) {
	// 多级嵌套 mapping 递归合并：raw 有 dns.nameserver-policy，merge 加 dns.enable
	// 与 dns.nameserver-policy 的同名子键之外的键，两侧都应保留。
	raw := parseDoc(t, `dns:
  enable: true
  nameserver-policy:
    "geosite:cn":
      - 223.5.5.5
`)
	merge := parseDoc(t, `dns:
  enhanced-mode: fake-ip
  nameserver-policy:
    "geosite:geolocation-!cn":
      - https://1.1.1.1/dns-query
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	dns := findMappingValue(topLevelMapping(final), "dns")
	require.NotNil(t, dns)
	assert.Equal(t, "true", findMappingValue(dns, "enable").Value, "raw 独有子键保留")
	assert.Equal(t, "fake-ip", findMappingValue(dns, "enhanced-mode").Value, "merge 新增子键写入")
	np := findMappingValue(dns, "nameserver-policy")
	require.NotNil(t, np)
	require.Len(t, np.Content, 4) // 两个子键，每个 key+value 各 2 节点
	assert.NotNil(t, findMappingValue(np, "geosite:cn"))
	assert.NotNil(t, findMappingValue(np, "geosite:geolocation-!cn"))
}

func TestApplyMerge_DeepMergeSequenceReplaced(t *testing.T) {
	// 对齐 deep_merge 的 (a,b)=>*a=b：非 prepend/append 的同名 sequence 直接替换，不拼接。
	raw := parseDoc(t, `rules:
  - DOMAIN,old,DIRECT
`)
	merge := parseDoc(t, `rules:
  - DOMAIN,new,DIRECT
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	assert.Equal(t, []string{"DOMAIN,new,DIRECT"}, seqValues(t, final, "rules"))
}

func TestApplyMerge_DeepMergeDoesNotMutateInput(t *testing.T) {
	// mapping 深合并不应修改 raw 入参。
	raw := parseDoc(t, `dns:
  enable: true
  nameserver:
    - 8.8.8.8
`)
	merge := parseDoc(t, `dns:
  enable: false
`)
	_, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	rawDNS := findMappingValue(topLevelMapping(raw), "dns")
	require.NotNil(t, rawDNS)
	assert.Equal(t, "true", findMappingValue(rawDNS, "enable").Value, "raw 不应被修改")
	ns := findMappingValue(rawDNS, "nameserver")
	require.Len(t, ns.Content, 1)
	assert.Equal(t, "8.8.8.8", ns.Content[0].Value)
}

func TestApplyMerge_PrependToMissingKey(t *testing.T) {
	// raw 无 rules → prepend 等价于赋值。
	raw := parseDoc(t, `mode: rule
`)
	merge := parseDoc(t, `prepend-rules:
  - DOMAIN,only.com,DIRECT
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	assert.Equal(t, []string{"DOMAIN,only.com,DIRECT"}, seqValues(t, final, "rules"))
}

func TestApplyMerge_CaseInsensitiveKeys(t *testing.T) {
	raw := parseDoc(t, `rules:
  - DOMAIN,old,DIRECT
`)
	merge := parseDoc(t, `Prepend-Rules:
  - DOMAIN,new,DIRECT
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"DOMAIN,new,DIRECT",
		"DOMAIN,old,DIRECT",
	}, seqValues(t, final, "rules"))
}

func TestApplyMerge_NilMerge(t *testing.T) {
	raw := parseDoc(t, `mode: rule
rules:
  - MATCH,DIRECT
`)
	final, err := ApplyMerge(raw, nil)
	require.NoError(t, err)
	// 无 merge 时应原样返回（raw 内容完整保留）
	assert.Equal(t, []string{"MATCH,DIRECT"}, seqValues(t, final, "rules"))
	assert.Equal(t, "rule", mappingToMap(t, final)["mode"])
}

func TestApplyMerge_DoesNotMutateInput(t *testing.T) {
	raw := parseDoc(t, `rules:
  - DOMAIN,old,DIRECT
`)
	merge := parseDoc(t, `prepend-rules:
  - DOMAIN,new,DIRECT
`)
	_, err := ApplyMerge(raw, merge)
	require.NoError(t, err)
	// raw 不应被修改
	root := topLevelMapping(raw)
	seq := findMappingValue(root, "rules")
	require.Len(t, seq.Content, 1)
	assert.Equal(t, "DOMAIN,old,DIRECT", seq.Content[0].Value)
}

func TestApplyMerge_PrependNonSequenceReturnsError(t *testing.T) {
	raw := parseDoc(t, `rules:
  - DOMAIN,old,DIRECT
`)
	merge := parseDoc(t, `prepend-rules: not-a-list
`)
	_, err := ApplyMerge(raw, merge)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "必须是列表")
}

func TestApplyMerge_CombinedAll(t *testing.T) {
	// 综合：六键 + 顶层覆盖同时存在。
	raw := parseDoc(t, `mode: global
rules:
  - MATCH,REJECT
proxies:
  - name: p1
proxy-groups:
  - name: g1
`)
	merge := parseDoc(t, `mode: rule
prepend-rules:
  - DOMAIN,a.com,DIRECT
append-rules:
  - DOMAIN,b.com,DIRECT
prepend-proxies:
  - name: p0
append-proxies:
  - name: p2
prepend-proxy-groups:
  - name: g0
append-proxy-groups:
  - name: g2
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)

	assert.Equal(t, []string{"DOMAIN,a.com,DIRECT", "MATCH,REJECT", "DOMAIN,b.com,DIRECT"},
		seqValues(t, final, "rules"))

	pg := findMappingValue(topLevelMapping(final), "proxies")
	require.Len(t, pg.Content, 3)
	assert.Equal(t, "p0", findMappingValue(pg.Content[0], "name").Value)
	assert.Equal(t, "p1", findMappingValue(pg.Content[1], "name").Value)
	assert.Equal(t, "p2", findMappingValue(pg.Content[2], "name").Value)

	grps := findMappingValue(topLevelMapping(final), "proxy-groups")
	require.Len(t, grps.Content, 3)
	assert.Equal(t, "g0", findMappingValue(grps.Content[0], "name").Value)
	assert.Equal(t, "g1", findMappingValue(grps.Content[1], "name").Value)
	assert.Equal(t, "g2", findMappingValue(grps.Content[2], "name").Value)

	assert.Equal(t, "rule", mappingToMap(t, final)["mode"])
}

func TestApplyMerge_CommentPreservation(t *testing.T) {
	// raw 带注释，合并后注释应保留。
	raw := parseDoc(t, `# 顶部注释
rules:
  # 规则注释
  - DOMAIN,old,DIRECT
`)
	merge := parseDoc(t, `prepend-rules:
  - DOMAIN,new,DIRECT
`)
	final, err := ApplyMerge(raw, merge)
	require.NoError(t, err)

	out, err := marshalNode(final)
	require.NoError(t, err)
	outStr := string(out)
	assert.Contains(t, outStr, "顶部注释", "raw 顶部注释应保留")
	assert.Contains(t, outStr, "规则注释", "raw 规则注释应保留")
}
