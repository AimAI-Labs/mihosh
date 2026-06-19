package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// writeTempConfig 把 content 写入临时文件并返回路径。
// content 为 "" 时仍创建一个 0 字节文件（用于空文件场景测试）。
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

// assertRulesEquals 重新解析文件并断言 rules 列表内容。
func assertRulesEquals(t *testing.T, path string, expected []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &doc))
	root := mappingRoot(&doc)
	require.NotNil(t, root, "顶层 mapping 不存在")
	seq := findRulesSequence(root)
	if len(expected) == 0 {
		assert.Nil(t, seq, "应无 rules 节点")
		return
	}
	require.NotNil(t, seq, "rules 节点缺失")
	var got []string
	for _, c := range seq.Content {
		got = append(got, c.Value)
	}
	assert.Equal(t, expected, got)
}

func TestInsertRule_IntoExistingRules(t *testing.T) {
	path := writeTempConfig(t, `# 顶部注释
proxy-groups: []
rules:
  - DOMAIN,old.com,DIRECT
  - MATCH,REJECT
`)
	require.NoError(t, InsertRule(path, "DOMAIN-SUFFIX,example.com,DIRECT", 1))
	assertRulesEquals(t, path, []string{
		"DOMAIN-SUFFIX,example.com,DIRECT",
		"DOMAIN,old.com,DIRECT",
		"MATCH,REJECT",
	})
}

func TestInsertRule_AppendWhenIndexTooLarge(t *testing.T) {
	path := writeTempConfig(t, "rules:\n  - DOMAIN,a.com,DIRECT\n")
	require.NoError(t, InsertRule(path, "DOMAIN,b.com,PROXY", 999))
	assertRulesEquals(t, path, []string{"DOMAIN,a.com,DIRECT", "DOMAIN,b.com,PROXY"})
}

func TestInsertRule_IndexLessThanOneInsertsAtTop(t *testing.T) {
	path := writeTempConfig(t, "rules:\n  - DOMAIN,a.com,DIRECT\n")
	require.NoError(t, InsertRule(path, "DOMAIN,b.com,DIRECT", 0))
	assertRulesEquals(t, path, []string{"DOMAIN,b.com,DIRECT", "DOMAIN,a.com,DIRECT"})
}

func TestInsertRule_CreatesRulesKeyWhenAbsent(t *testing.T) {
	path := writeTempConfig(t, "# 用户配置\nproxy-groups: []\nmode: rule\n")
	require.NoError(t, InsertRule(path, "DOMAIN-SUFFIX,example.com,DIRECT", 1))
	assertRulesEquals(t, path, []string{"DOMAIN-SUFFIX,example.com,DIRECT"})

	// 其他顶层键仍应保留
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "proxy-groups:")
	assert.Contains(t, string(data), "mode: rule")
}

func TestInsertRule_PreservesCommentsAndKeyOrder(t *testing.T) {
	src := `# mihomo 配置
mixed-port: 7890

# 代理组
proxy-groups:
  - name: PROXY

# 规则
rules:
  - DOMAIN,a.com,DIRECT
`
	path := writeTempConfig(t, src)
	require.NoError(t, InsertRule(path, "DOMAIN,b.com,PROXY", 1))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "# mihomo 配置")
	assert.Contains(t, content, "# 代理组")
	assert.Contains(t, content, "# 规则")
	assert.Contains(t, content, "mixed-port: 7890")
	// 新规则插入到顶部
	assert.Less(t, strings.Index(content, "DOMAIN,b.com,PROXY"), strings.Index(content, "DOMAIN,a.com,DIRECT"))
}

func TestInsertRule_EmptyFileCreatesMapping(t *testing.T) {
	path := writeTempConfig(t, "")
	require.NoError(t, InsertRule(path, "DOMAIN,a.com,DIRECT", 1))
	assertRulesEquals(t, path, []string{"DOMAIN,a.com,DIRECT"})
}

func TestInsertRule_EmptyRulesValueBecomesSequence(t *testing.T) {
	// `rules:` 后无值（null）—— 应被提升为空序列后插入
	path := writeTempConfig(t, "mode: rule\nrules:\n")
	require.NoError(t, InsertRule(path, "DOMAIN,a.com,DIRECT", 1))
	assertRulesEquals(t, path, []string{"DOMAIN,a.com,DIRECT"})
}

func TestInsertRule_EmptyRuleStringRejected(t *testing.T) {
	path := writeTempConfig(t, "rules: []\n")
	err := InsertRule(path, "", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不能为空")
}

func TestCountRules(t *testing.T) {
	t.Run("existing", func(t *testing.T) {
		path := writeTempConfig(t, "rules:\n  - DOMAIN,a.com,DIRECT\n  - MATCH,PROXY\n")
		n, err := CountRules(path)
		require.NoError(t, err)
		assert.Equal(t, 2, n)
	})
	t.Run("absent", func(t *testing.T) {
		path := writeTempConfig(t, "mode: rule\n")
		n, err := CountRules(path)
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})
	t.Run("empty file", func(t *testing.T) {
		path := writeTempConfig(t, "")
		n, err := CountRules(path)
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})
}

func TestDeleteRule_RemovesFirstMatchByContent(t *testing.T) {
	path := writeTempConfig(t, `rules:
  - DOMAIN-SUFFIX,example.com,DIRECT
  - DOMAIN-SUFFIX,example.com,DIRECT
  - MATCH,REJECT
`)
	require.NoError(t, DeleteRule(path, "DOMAIN-SUFFIX", "example.com", "DIRECT", false))
	// 仅删除第一条匹配，第二条同名规则保留
	assertRulesEquals(t, path, []string{
		"DOMAIN-SUFFIX,example.com,DIRECT",
		"MATCH,REJECT",
	})
}

func TestDeleteRule_MatchIncludesNoResolve(t *testing.T) {
	path := writeTempConfig(t, `rules:
  - IP-CIDR,10.0.0.0/24,DIRECT,no-resolve
  - IP-CIDR,10.0.0.0/24,DIRECT
`)
	// 仅删除带 no-resolve 的那条，不带 no-resolve 的同名规则保留
	require.NoError(t, DeleteRule(path, "IP-CIDR", "10.0.0.0/24", "DIRECT", true))
	assertRulesEquals(t, path, []string{
		"IP-CIDR,10.0.0.0/24,DIRECT",
	})
}

func TestDeleteRule_MatchMatchType(t *testing.T) {
	path := writeTempConfig(t, `rules:
  - MATCH,REJECT
  - DOMAIN,a.com,DIRECT
`)
	require.NoError(t, DeleteRule(path, "MATCH", "", "REJECT", false))
	assertRulesEquals(t, path, []string{"DOMAIN,a.com,DIRECT"})
}

func TestDeleteRule_NotFoundReturnsError(t *testing.T) {
	path := writeTempConfig(t, "rules:\n  - DOMAIN,a.com,DIRECT\n")
	err := DeleteRule(path, "DOMAIN-SUFFIX", "missing.com", "DIRECT", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未在配置文件中找到")
	// 配置文件应保持不变
	assertRulesEquals(t, path, []string{"DOMAIN,a.com,DIRECT"})
}

func TestDeleteRule_NoRulesKeyReturnsError(t *testing.T) {
	path := writeTempConfig(t, "mode: rule\n")
	err := DeleteRule(path, "DOMAIN", "a.com", "DIRECT", false)
	require.Error(t, err)
}

func TestDeleteRule_PreservesCommentsAndKeyOrder(t *testing.T) {
	src := `# mihomo 配置
mixed-port: 7890

# 规则
rules:
  # keep me
  - DOMAIN,a.com,DIRECT
  - DOMAIN,b.com,PROXY
`
	path := writeTempConfig(t, src)
	require.NoError(t, DeleteRule(path, "DOMAIN", "a.com", "DIRECT", false))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "# mihomo 配置")
	assert.Contains(t, content, "# 规则")
	assert.Contains(t, content, "mixed-port: 7890")
	assert.Contains(t, content, "DOMAIN,b.com,PROXY")
	assert.NotContains(t, content, "DOMAIN,a.com,DIRECT")
}

func TestNormalizeInsertIndex(t *testing.T) {
	assert.Equal(t, 0, normalizeInsertIndex(0, 5))
	assert.Equal(t, 0, normalizeInsertIndex(-3, 5))
	assert.Equal(t, 0, normalizeInsertIndex(1, 5))
	assert.Equal(t, 4, normalizeInsertIndex(5, 5))
	assert.Equal(t, 5, normalizeInsertIndex(6, 5))
	assert.Equal(t, 5, normalizeInsertIndex(999, 5))
}

// TestExtractProxyPolicies_ReadsGroupsAndNodes 验证按分类返回：
// groups = 内置 + proxy-groups；nodes = proxies；且去重、保留顺序。
func TestExtractProxyPolicies_ReadsGroupsAndNodes(t *testing.T) {
	path := writeTempConfig(t, `mixed-port: 7890
proxies:
  - name: SS-1
  - name: SS-2
proxy-groups:
  - name: PROXY
  - name: AUTO
`)
	groups, nodes, err := ExtractProxyPolicies(path)
	require.NoError(t, err)

	wantGroups := []string{"DIRECT", "REJECT", "PROXY", "AUTO"}
	wantNodes := []string{"SS-1", "SS-2"}
	assert.Equal(t, wantGroups, groups)
	assert.Equal(t, wantNodes, nodes)
}

// TestExtractProxyPolicies_DedupAcrossGroupsAndNodes 验证同名跨分类去重：
// 若同名同时出现在 proxy-groups 与 proxies，则只保留 groups 中首次出现项。
func TestExtractProxyPolicies_DedupAcrossGroupsAndNodes(t *testing.T) {
	path := writeTempConfig(t, `proxies:
  - name: SS-1
proxy-groups:
  - name: SS-1
  - name: AUTO
`)
	groups, nodes, err := ExtractProxyPolicies(path)
	require.NoError(t, err)

	assert.Equal(t, []string{"DIRECT", "REJECT", "SS-1", "AUTO"}, groups)
	// nodes 中 SS-1 已被 groups 占用，应被剔除 → nodes 为空
	assert.Equal(t, []string{}, nodes)
}

// TestExtractProxyPolicies_MissingFileFallsBack 验证文件不存在时降级为内置策略。
func TestExtractProxyPolicies_MissingFileFallsBack(t *testing.T) {
	groups, nodes, err := ExtractProxyPolicies(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	require.NoError(t, err)
	assert.Equal(t, []string{"DIRECT", "REJECT"}, groups)
	assert.Equal(t, []string{}, nodes)
}

// TestExtractProxyPolicies_EmptyConfigFallsBack 验证无 proxies/groups 的配置仅返回内置策略。
func TestExtractProxyPolicies_EmptyConfigFallsBack(t *testing.T) {
	path := writeTempConfig(t, "mode: rule\n")
	groups, nodes, err := ExtractProxyPolicies(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"DIRECT", "REJECT"}, groups)
	assert.Equal(t, []string{}, nodes)
}

// TestExtractPolicies_FlatCompat 验证向后兼容的扁平 API：
// 内置 → proxy-groups → proxies，去重，保留顺序。
func TestExtractPolicies_FlatCompat(t *testing.T) {
	path := writeTempConfig(t, `proxies:
  - name: SS-1
  - name: SS-2
proxy-groups:
  - name: PROXY
  - name: AUTO
`)
	got, err := ExtractPolicies(path)
	require.NoError(t, err)
	want := []string{"DIRECT", "REJECT", "PROXY", "AUTO", "SS-1", "SS-2"}
	assert.Equal(t, want, got)
}
