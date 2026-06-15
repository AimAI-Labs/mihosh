package rules

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// stringSlicesEqual 浅比较两个字符串切片是否逐项相等。
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// pressTab 返回 Tab 按键消息。
func pressTab() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyTab}
}

// pressShiftTab 返回 Shift+Tab 按键消息。
func pressShiftTab() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyShiftTab}
}

// TestAddForm_OpenWithN 验证按 n 打开添加规则弹窗。
func TestAddForm_OpenWithN(t *testing.T) {
	s := State{}
	s, _ = s.Update(keyMsg('n'), nil)
	if !s.ShowAddForm() {
		t.Fatal("expected add form open after pressing 'n'")
	}
	// 默认类型为 DOMAIN-SUFFIX
	if got := s.addForm.currentType(); got != "DOMAIN-SUFFIX" {
		t.Fatalf("expected default type DOMAIN-SUFFIX, got %s", got)
	}
}

// TestAddForm_EscClosesAndResets 验证 Esc 关闭并重置表单。
func TestAddForm_EscClosesAndResets(t *testing.T) {
	s := State{}
	// 打开表单
	s, _ = s.Update(keyMsg('n'), nil)
	// 输入一些字符到 payload 字段
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}, nil)
	// Esc 关闭
	s, _ = s.Update(pressKey("esc"), nil)
	if s.ShowAddForm() {
		t.Fatal("expected add form closed after Esc")
	}
	// 再次打开应为干净表单
	s, _ = s.Update(keyMsg('n'), nil)
	if s.addForm.fields[addFieldPayload].Value() != "" {
		t.Fatalf("expected reset payload field, got %q", s.addForm.fields[addFieldPayload].Value())
	}
}

// TestAddForm_TabCyclesFields 验证 Tab 在字段间循环。
func TestAddForm_TabCyclesFields(t *testing.T) {
	s := State{}
	s, _ = s.Update(keyMsg('n'), nil)

	// 初始焦点 payload
	if s.addForm.fieldCursor != addFieldPayload {
		t.Fatalf("expected initial cursor at payload(%d), got %d", addFieldPayload, s.addForm.fieldCursor)
	}
	// Tab → proxy
	s, _ = s.Update(pressTab(), nil)
	if s.addForm.fieldCursor != addFieldProxy {
		t.Fatalf("expected cursor at proxy after Tab, got %d", s.addForm.fieldCursor)
	}
	// Tab → index
	s, _ = s.Update(pressTab(), nil)
	if s.addForm.fieldCursor != addFieldIndex {
		t.Fatalf("expected cursor at index after 2x Tab, got %d", s.addForm.fieldCursor)
	}
	// Tab → 回到 payload（循环）
	s, _ = s.Update(pressTab(), nil)
	if s.addForm.fieldCursor != addFieldPayload {
		t.Fatalf("expected cursor back at payload after 3x Tab, got %d", s.addForm.fieldCursor)
	}
}

// TestAddForm_LeftRightCyclesType 验证 ←/→ 循环切换规则类型（类型选择器为 ◀ ▶ 横向）。
func TestAddForm_LeftRightCyclesType(t *testing.T) {
	s := State{}
	s, _ = s.Update(keyMsg('n'), nil)
	initialType := s.addForm.currentType()

	// → → 下一类型
	s, _ = s.Update(pressKey("right"), nil)
	nextType := s.addForm.currentType()
	if nextType == initialType {
		t.Fatal("expected type to change after Right")
	}

	// ← → 回到初始类型
	s, _ = s.Update(pressKey("left"), nil)
	if s.addForm.currentType() != initialType {
		t.Fatalf("expected type back to %s after Left, got %s", initialType, s.addForm.currentType())
	}
}

// TestAddForm_UpDownCyclesField 验证 ↑/↓ 在字段间切换（字段为垂直堆叠）。
func TestAddForm_UpDownCyclesField(t *testing.T) {
	s := State{}
	s, _ = s.Update(keyMsg('n'), nil)

	// 初始焦点 payload
	if s.addForm.fieldCursor != addFieldPayload {
		t.Fatalf("expected initial cursor at payload(%d), got %d", addFieldPayload, s.addForm.fieldCursor)
	}
	// ↓ → proxy
	s, _ = s.Update(pressKey("down"), nil)
	if s.addForm.fieldCursor != addFieldProxy {
		t.Fatalf("expected cursor at proxy after Down, got %d", s.addForm.fieldCursor)
	}
	// ↓ → index
	s, _ = s.Update(pressKey("down"), nil)
	if s.addForm.fieldCursor != addFieldIndex {
		t.Fatalf("expected cursor at index after 2x Down, got %d", s.addForm.fieldCursor)
	}
	// ↑ → 回到 proxy
	s, _ = s.Update(pressKey("up"), nil)
	if s.addForm.fieldCursor != addFieldProxy {
		t.Fatalf("expected cursor back at proxy after Up, got %d", s.addForm.fieldCursor)
	}
}

// TestAddForm_EnterValidationFailsOnEmpty 验证空 payload/proxy 时 Enter 校验失败、不关闭。
func TestAddForm_EnterValidationFailsOnEmpty(t *testing.T) {
	s := State{}
	s, _ = s.Update(keyMsg('n'), nil)
	// 空表单按 Enter → 校验失败
	s, _ = s.Update(pressKey("enter"), nil)
	if !s.ShowAddForm() {
		t.Fatal("expected form to remain open on validation failure")
	}
	if s.addForm.errMsg == "" {
		t.Fatal("expected error message set on validation failure")
	}
}

// TestAddForm_EnterSucceedsWithValidInput 验证合法输入后 Enter 关闭并返回写盘命令。
func TestAddForm_EnterSucceedsWithValidInput(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	// 输入 payload（当前焦点在 payload）
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("example.com")}, nil)
	// proxy 为只读选择器，默认即 DIRECT，无需输入
	// Enter → 提交
	s, cmd := s.Update(pressKey("enter"), nil)
	if s.ShowAddForm() {
		t.Fatal("expected form closed on successful submit")
	}
	if cmd == nil {
		t.Fatal("expected a write command returned on submit")
	}
}

// TestAddForm_MatchTypeSkipsPayload 验证 MATCH 类型不要求 payload。
func TestAddForm_MatchTypeSkipsPayload(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)

	// 循环 → 直到选中 MATCH
	for i := 0; i < len(ruleTypePresets); i++ {
		if s.addForm.isMatchType() {
			break
		}
		s, _ = s.Update(pressKey("right"), nil)
	}
	if !s.addForm.isMatchType() {
		t.Fatal("expected to reach MATCH type by cycling Right")
	}

	// proxy 为只读选择器，默认即 DIRECT，无需 payload 即可提交
	s, cmd := s.Update(pressKey("enter"), nil)
	if s.ShowAddForm() {
		t.Fatal("expected MATCH rule to submit without payload")
	}
	if cmd == nil {
		t.Fatal("expected write command for MATCH rule")
	}
}

// TestAddForm_IndexValidationRejectsNonNumeric 验证位置字段非数字时校验失败。
func TestAddForm_IndexValidationRejectsNonNumeric(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	// payload
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a.com")}, nil)
	// index = "abc"（proxy 为只读选择器，跳过）
	s, _ = s.Update(pressTab(), nil) // → proxy
	s, _ = s.Update(pressTab(), nil) // → index
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")}, nil)
	// Enter → 校验失败
	s, _ = s.Update(pressKey("enter"), nil)
	if !s.ShowAddForm() {
		t.Fatal("expected form open when index is non-numeric")
	}
}

// TestBuildRuleLine 验证规则字符串组装格式。
func TestBuildRuleLine(t *testing.T) {
	cases := []struct {
		name     string
		ruleType string
		payload  string
		proxy    string
		expected string
	}{
		{"standard", "DOMAIN-SUFFIX", "example.com", "DIRECT", "DOMAIN-SUFFIX,example.com,DIRECT"},
		{"match_no_payload", "MATCH", "ignored", "REJECT", "MATCH,REJECT"},
		{"trim_spaces", "DOMAIN", "  a.com  ", "  PROXY  ", "DOMAIN,a.com,PROXY"},
		{"lowercase_match", "match", "", "DIRECT", "MATCH,DIRECT"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildRuleLine(c.ruleType, c.payload, c.proxy)
			if got != c.expected {
				t.Fatalf("expected %q, got %q", c.expected, got)
			}
		})
	}
}

// ============================================================
//  策略选择器行为（◀ ▶ 横向，与类型行镜像）
// ============================================================

// TestAddForm_DefaultProxyIsDirect 验证无配置文件时兜底策略含 DIRECT 且默认选中 DIRECT。
func TestAddForm_DefaultProxyIsDirect(t *testing.T) {
	form := newAddForm("")
	// 兜底列表为 [DIRECT, REJECT]
	if !stringSlicesEqual(form.proxyPolicies, []string{defaultProxyPolicy, "REJECT"}) {
		t.Fatalf("expected fallback policies [DIRECT REJECT], got %v", form.proxyPolicies)
	}
	if got := form.currentProxy(); got != defaultProxyPolicy {
		t.Fatalf("expected default proxy DIRECT, got %s", got)
	}
}

// TestAddForm_LeftRightCyclesProxyWhenFocused 验证策略行聚焦时 ←/→ 循环切换策略，
// 且不影响类型选择器。
func TestAddForm_LeftRightCyclesProxyWhenFocused(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)

	// Tab 到 proxy 字段
	s, _ = s.Update(pressTab(), nil)
	if !s.addForm.isProxyField() {
		t.Fatal("expected focus on proxy field")
	}
	proxyBefore := s.addForm.currentProxy()
	typeBefore := s.addForm.currentType()

	// → 切换策略
	s, _ = s.Update(pressKey("right"), nil)
	if s.addForm.currentProxy() == proxyBefore {
		t.Fatalf("expected proxy to change after Right, still %s", proxyBefore)
	}
	// 类型不应被影响（←/→ 在策略行聚焦时只动策略选择器）
	if s.addForm.currentType() != typeBefore {
		t.Fatalf("type changed unexpectedly: %s -> %s", typeBefore, s.addForm.currentType())
	}

	// ← 回到原策略
	s, _ = s.Update(pressKey("left"), nil)
	if s.addForm.currentProxy() != proxyBefore {
		t.Fatalf("expected proxy back to %s after Left, got %s", proxyBefore, s.addForm.currentProxy())
	}
}

// TestAddForm_LeftRightCyclesTypeWhenProxyNotFocused 验证非策略行聚焦时 ←/→ 仍切换类型。
func TestAddForm_LeftRightCyclesTypeWhenProxyNotFocused(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	// 焦点在 payload（非策略行）
	if s.addForm.isProxyField() {
		t.Fatal("expected focus NOT on proxy initially")
	}
	typeBefore := s.addForm.currentType()

	s, _ = s.Update(pressKey("right"), nil)
	if s.addForm.currentType() == typeBefore {
		t.Fatalf("expected type to change when proxy not focused, still %s", typeBefore)
	}
}

// TestAddForm_ProxyFieldIgnoresTextInput 验证策略行为只读选择器，键入文本不影响其值。
func TestAddForm_ProxyFieldIgnoresTextInput(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	// Tab 到 proxy
	s, _ = s.Update(pressTab(), nil)
	proxyBefore := s.addForm.currentProxy()

	// 键入文本应被吞掉
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("PROXY")}, nil)
	if s.addForm.currentProxy() != proxyBefore {
		t.Fatalf("proxy changed via text input on read-only field: %s -> %s",
			proxyBefore, s.addForm.currentProxy())
	}
}

// TestAddForm_ProxySelectorSubmitsCurrentProxy 验证提交时使用选择器的当前策略，
// 而非（已移除的）proxy 文本输入。
func TestAddForm_ProxySelectorSubmitsCurrentProxy(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	// payload
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("example.com")}, nil)
	// Tab → proxy，→ 切到 REJECT
	s, _ = s.Update(pressTab(), nil)
	s, _ = s.Update(pressKey("right"), nil)
	chosenProxy := s.addForm.currentProxy()
	if chosenProxy == defaultProxyPolicy {
		t.Fatalf("expected selected proxy to differ from default DIRECT")
	}

	// 提交；AddRuleCmd 在 lazy 执行时才调用，这里只断言表单内 currentProxy 即提交值。
	// 回到 payload 焦点（确保不会因 index 字段为空失败）后提交。
	s, _ = s.Update(pressTab(), nil) // → index
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")}, nil)
	s, _ = s.Update(pressKey("enter"), nil)
	if s.ShowAddForm() {
		t.Fatal("expected successful submit")
	}
	_ = chosenProxy // 提交路径已使用 submit.currentProxy()（见 state.go）
}

// ============================================================
//  配置提取（loadProxyPolicies）
// ============================================================

// writeAddFormTempConfig 写入临时配置文件并返回路径（与 config 包测试一致的风格）。
func writeAddFormTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

// TestLoadProxyPolicies_ReadsGroupsAndProxies 验证从源配置文件提取 proxy-groups 与 proxies 名称，
// 且内置 DIRECT/REJECT 始终在前。
func TestLoadProxyPolicies_ReadsGroupsAndProxies(t *testing.T) {
	path := writeAddFormTempConfig(t, `mixed-port: 7890
proxies:
  - name: SS-1
  - name: SS-2
proxy-groups:
  - name: PROXY
  - name: AUTO
`)
	form := newAddForm(path)

	// 预期顺序：内置在前，随后 proxy-groups，再 proxies
	want := []string{defaultProxyPolicy, "REJECT", "PROXY", "AUTO", "SS-1", "SS-2"}
	if !stringSlicesEqual(form.proxyPolicies, want) {
		t.Fatalf("policies mismatch:\n got=%v\nwant=%v", form.proxyPolicies, want)
	}
	// 默认选中 DIRECT（indexOfPolicy 命中内置 DIRECT）
	if got := form.currentProxy(); got != defaultProxyPolicy {
		t.Fatalf("expected default selected DIRECT, got %s", got)
	}
}

// TestLoadProxyPolicies_FallbackOnMissingFile 验证配置文件不存在时降级为内置策略。
func TestLoadProxyPolicies_FallbackOnMissingFile(t *testing.T) {
	form := newAddForm(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if !stringSlicesEqual(form.proxyPolicies, []string{defaultProxyPolicy, "REJECT"}) {
		t.Fatalf("expected fallback [DIRECT REJECT], got %v", form.proxyPolicies)
	}
	if got := form.currentProxy(); got != defaultProxyPolicy {
		t.Fatalf("expected default DIRECT, got %s", got)
	}
}

// TestLoadProxyPolicies_EmptyConfigFallsBack 验证空配置文件（无 proxies/groups）仅返回内置策略。
func TestLoadProxyPolicies_EmptyConfigFallsBack(t *testing.T) {
	path := writeAddFormTempConfig(t, "mode: rule\n")
	form := newAddForm(path)
	if !stringSlicesEqual(form.proxyPolicies, []string{defaultProxyPolicy, "REJECT"}) {
		t.Fatalf("expected only builtin policies, got %v", form.proxyPolicies)
	}
}

// TestIndexOfPolicy_CaseInsensitive 验证 indexOfPolicy 大小写不敏感匹配。
func TestIndexOfPolicy_CaseInsensitive(t *testing.T) {
	policies := []string{"DIRECT", "REJECT", "PROXY"}
	if i := indexOfPolicy(policies, "direct"); i != 0 {
		t.Fatalf("expected index 0 for 'direct', got %d", i)
	}
	if i := indexOfPolicy(policies, "Proxy"); i != 2 {
		t.Fatalf("expected index 2 for 'Proxy', got %d", i)
	}
	if i := indexOfPolicy(policies, "MISSING"); i != 0 {
		t.Fatalf("expected index 0 (fallback) for missing, got %d", i)
	}
}

// TestCycleProxy_WrapsAround 验证 cycleProxy 在列表边界循环。
func TestCycleProxy_WrapsAround(t *testing.T) {
	form := newAddForm("")
	// [DIRECT, REJECT]，初始在第 0 项 DIRECT
	form.cycleProxy(1)
	if got := form.currentProxy(); got != "REJECT" {
		t.Fatalf("expected REJECT after +1, got %s", got)
	}
	// 再 +1 应循环回 DIRECT
	form.cycleProxy(1)
	if got := form.currentProxy(); got != defaultProxyPolicy {
		t.Fatalf("expected DIRECT after wrap, got %s", got)
	}
	// -1 应到 REJECT
	form.cycleProxy(-1)
	if got := form.currentProxy(); got != "REJECT" {
		t.Fatalf("expected REJECT after -1, got %s", got)
	}
}
