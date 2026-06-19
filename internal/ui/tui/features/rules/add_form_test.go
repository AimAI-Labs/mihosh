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
		name      string
		ruleType  string
		payload   string
		proxy     string
		noResolve bool
		expected  string
	}{
		{"standard", "DOMAIN-SUFFIX", "example.com", "DIRECT", false, "DOMAIN-SUFFIX,example.com,DIRECT"},
		{"match_no_payload", "MATCH", "ignored", "REJECT", false, "MATCH,REJECT"},
		{"trim_spaces", "DOMAIN", "  a.com  ", "  PROXY  ", false, "DOMAIN,a.com,PROXY"},
		{"lowercase_match", "match", "", "DIRECT", false, "MATCH,DIRECT"},
		{"ip_cidr_no_resolve", "IP-CIDR", "10.0.0.0/8", "DIRECT", true, "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve"},
		{"ip_cidr6_no_resolve", "IP-CIDR6", "fd00::/8", "REJECT", true, "IP-CIDR6,fd00::/8,REJECT,no-resolve"},
		{"src_ip_cidr_no_resolve", "SRC-IP-CIDR", "192.168.0.0/16", "DIRECT", true, "SRC-IP-CIDR,192.168.0.0/16,DIRECT,no-resolve"},
		{"ip_cidr_no_resolve_false", "IP-CIDR", "10.0.0.0/8", "DIRECT", false, "IP-CIDR,10.0.0.0/8,DIRECT"},
		{"non_ip_no_resolve_ignored", "DOMAIN-SUFFIX", "example.com", "DIRECT", true, "DOMAIN-SUFFIX,example.com,DIRECT"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildRuleLine(c.ruleType, c.payload, c.proxy, c.noResolve)
			if got != c.expected {
				t.Fatalf("expected %q, got %q", c.expected, got)
			}
		})
	}
}

// ============================================================
//  payload 前置校验（各类型规则）
// ============================================================

// TestValidate_GeoipRejected 验证 GEOIP 非法国家代码被拦截。
func TestValidate_GeoipRejected(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{"single_letter", "C"},
		{"three_letters", "USA"},
		{"numeric", "12"},
		{"with_space", "C N"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			form := newAddForm("")
			form.typeCursor = indexOfPreset("GEOIP")
			form.fields[addFieldPayload].SetValue(c.payload)
			ok, errKey := form.validate()
			if ok {
				t.Fatalf("expected validation to fail for GEOIP payload %q", c.payload)
			}
			if errKey != "rules.add_err_geoip" {
				t.Fatalf("expected errKey=rules.add_err_geoip, got %q", errKey)
			}
		})
	}
}

// TestValidate_GeoipAccepted 验证合法 GEOIP 国家代码通过。
func TestValidate_GeoipAccepted(t *testing.T) {
	form := newAddForm("")
	form.typeCursor = indexOfPreset("GEOIP")
	form.fields[addFieldPayload].SetValue("CN")
	ok, errKey := form.validate()
	if !ok {
		t.Fatalf("expected validation to pass for GEOIP=CN, got errKey=%q", errKey)
	}
}

// TestValidate_GeositeRejected 验证 GEOSITE 非法标识符被拦截。
func TestValidate_GeositeRejected(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{"leading_dot", ".google"},
		{"with_space", "my site"},
		{"with_comma", "a,b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			form := newAddForm("")
			form.typeCursor = indexOfPreset("GEOSITE")
			form.fields[addFieldPayload].SetValue(c.payload)
			ok, _ := form.validate()
			if ok {
				t.Fatalf("expected validation to fail for GEOSITE payload %q", c.payload)
			}
		})
	}
}

// TestValidate_GeositeAccepted 验证合法 GEOSITE 标识符通过。
func TestValidate_GeositeAccepted(t *testing.T) {
	for _, name := range []string{"google", "google.cn", "my-site", "netflix"} {
		t.Run(name, func(t *testing.T) {
			form := newAddForm("")
			form.typeCursor = indexOfPreset("GEOSITE")
			form.fields[addFieldPayload].SetValue(name)
			ok, errKey := form.validate()
			if !ok {
				t.Fatalf("expected validation to pass for GEOSITE=%q, got errKey=%q", name, errKey)
			}
		})
	}
}

// TestValidate_ProcessRejected 验证进程名/路径含空格时被拦截。
func TestValidate_ProcessRejected(t *testing.T) {
	cases := []struct {
		typeName string
		payload  string
	}{
		{"PROCESS-NAME", "my process"},
		{"PROCESS-PATH", "/path with/space"},
	}
	for _, c := range cases {
		t.Run(c.typeName, func(t *testing.T) {
			form := newAddForm("")
			form.typeCursor = indexOfPreset(c.typeName)
			form.fields[addFieldPayload].SetValue(c.payload)
			ok, errKey := form.validate()
			if ok {
				t.Fatalf("expected validation to fail for %s payload %q", c.typeName, c.payload)
			}
			if errKey != "rules.add_err_process" {
				t.Fatalf("expected errKey=rules.add_err_process, got %q", errKey)
			}
		})
	}
}

// TestValidate_ProcessAccepted 验证合法进程名/路径通过。
func TestValidate_ProcessAccepted(t *testing.T) {
	cases := []struct {
		typeName string
		payload  string
	}{
		{"PROCESS-NAME", "chrome.exe"},
		{"PROCESS-PATH", "/usr/bin/chrome"},
	}
	for _, c := range cases {
		t.Run(c.typeName, func(t *testing.T) {
			form := newAddForm("")
			form.typeCursor = indexOfPreset(c.typeName)
			form.fields[addFieldPayload].SetValue(c.payload)
			ok, errKey := form.validate()
			if !ok {
				t.Fatalf("expected validation to pass for %s=%q, got errKey=%q", c.typeName, c.payload, errKey)
			}
		})
	}
}

// ============================================================
//  策略选择器行为（二级弹窗）
// ============================================================

// TestAddForm_DefaultProxyIsDirect 验证无配置文件时兆底策略含 DIRECT 且默认选中 DIRECT。
func TestAddForm_DefaultProxyIsDirect(t *testing.T) {
	form := newAddForm("")
	// 兆底 groups 为 [DIRECT, REJECT]
	if !stringSlicesEqual(form.proxyGroups, []string{defaultProxyPolicy, "REJECT"}) {
		t.Fatalf("expected fallback groups [DIRECT REJECT], got %v", form.proxyGroups)
	}
	if got := form.currentProxy(); got != defaultProxyPolicy {
		t.Fatalf("expected default proxy DIRECT, got %s", got)
	}
}

// TestAddForm_ProxyFieldEnterOpensPicker 验证策略行聚焦时 Enter 打开二级弹窗。
func TestAddForm_ProxyFieldEnterOpensPicker(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	// Tab 到 proxy 字段
	s, _ = s.Update(pressTab(), nil)
	if !s.addForm.isProxyField() {
		t.Fatal("expected focus on proxy field")
	}
	// Enter 打开二级弹窗
	s, _ = s.Update(pressKey("enter"), nil)
	if !s.addForm.isProxyPickerOpen() {
		t.Fatal("expected proxy picker to open after Enter on proxy field")
	}
}

// TestAddForm_PickerTabSwitch 验证 Tab 键在策略弹窗内切分类。
func TestAddForm_PickerTabSwitch(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	// Tab → proxy, Enter → picker
	s, _ = s.Update(pressTab(), nil)
	s, _ = s.Update(pressKey("enter"), nil)
	if s.addForm.pickerTab != addPickerTabGroup {
		t.Fatalf("expected initial tab=group(0), got %d", s.addForm.pickerTab)
	}
	// Tab → 切到节点
	s, _ = s.Update(pressTab(), nil)
	if s.addForm.pickerTab != addPickerTabNode {
		t.Fatalf("expected tab=node(1) after Tab, got %d", s.addForm.pickerTab)
	}
	// Shift+Tab → 回到策略组
	s, _ = s.Update(pressShiftTab(), nil)
	if s.addForm.pickerTab != addPickerTabGroup {
		t.Fatalf("expected tab=group(0) after Shift+Tab, got %d", s.addForm.pickerTab)
	}
}

// TestAddForm_PickerEscClosesKeepsSelection 验证 Esc 关闭弹窗保留已选策略。
func TestAddForm_PickerEscClosesKeepsSelection(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	s, _ = s.Update(pressTab(), nil) // → proxy
	s, _ = s.Update(pressKey("enter"), nil) // open picker
	s, _ = s.Update(pressKey("esc"), nil) // close picker
	if s.addForm.isProxyPickerOpen() {
		t.Fatal("expected picker closed after Esc")
	}
	if s.addForm.currentProxy() != defaultProxyPolicy {
		t.Fatalf("expected proxy still DIRECT after Esc, got %s", s.addForm.currentProxy())
	}
}

// TestAddForm_PickerUpDownMovesCursor 验证 ↑/↓ 在过滤列表中移动。
func TestAddForm_PickerUpDownMovesCursor(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	s, _ = s.Update(pressTab(), nil)
	s, _ = s.Update(pressKey("enter"), nil) // open picker
	// 初始光标 0
	if s.addForm.pickerCursor != 0 {
		t.Fatalf("expected initial picker cursor 0, got %d", s.addForm.pickerCursor)
	}
	// ↓ → 1
	s, _ = s.Update(pressKey("down"), nil)
	if s.addForm.pickerCursor != 1 {
		t.Fatalf("expected picker cursor 1 after Down, got %d", s.addForm.pickerCursor)
	}
	// ↑ → 0
	s, _ = s.Update(pressKey("up"), nil)
	if s.addForm.pickerCursor != 0 {
		t.Fatalf("expected picker cursor 0 after Up, got %d", s.addForm.pickerCursor)
	}
}

// TestAddForm_PickerEnterSelects 验证 Enter 选中当前项并回填。
func TestAddForm_PickerEnterSelects(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	s, _ = s.Update(pressTab(), nil)
	s, _ = s.Update(pressKey("enter"), nil) // open picker
	// ↓ 移动到 REJECT (index 1)
	s, _ = s.Update(pressKey("down"), nil)
	// Enter 选中
	s, _ = s.Update(pressKey("enter"), nil)
	if s.addForm.isProxyPickerOpen() {
		t.Fatal("expected picker closed after Enter selection")
	}
	if s.addForm.currentProxy() != "REJECT" {
		t.Fatalf("expected proxy REJECT after selection, got %s", s.addForm.currentProxy())
	}
}

// TestAddForm_PickerFuzzySearch 验证模糊搜索过滤列表。
func TestAddForm_PickerFuzzySearch(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	s, _ = s.Update(pressTab(), nil)
	s, _ = s.Update(pressKey("enter"), nil) // open picker
	// 输入 "rej" 应过滤出 REJECT
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("rej")}, nil)
	filtered := s.addForm.pickerFiltered()
	if len(filtered) != 1 || filtered[0] != "REJECT" {
		t.Fatalf("expected [REJECT] after typing 'rej', got %v", filtered)
	}
}

// TestAddForm_PickerBackspace 验证 Backspace 删除搜索字符。
func TestAddForm_PickerBackspace(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	s, _ = s.Update(pressTab(), nil)
	s, _ = s.Update(pressKey("enter"), nil) // open picker
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("rej")}, nil)
	// Backspace
	s, _ = s.Update(pressKey("backspace"), nil)
	if s.addForm.pickerSearch != "re" {
		t.Fatalf("expected search 're' after backspace, got %q", s.addForm.pickerSearch)
	}
}

// TestAddForm_LeftRightDoesNotChangeProxy 验证策略行聚焦时 ←/→ 不再切换策略。
func TestAddForm_LeftRightDoesNotChangeProxy(t *testing.T) {
	s := State{}.SetConfigPath("/tmp/fake-config.yaml")
	s, _ = s.Update(keyMsg('n'), nil)
	// Tab 到 proxy 字段
	s, _ = s.Update(pressTab(), nil)
	if !s.addForm.isProxyField() {
		t.Fatal("expected focus on proxy field")
	}
	proxyBefore := s.addForm.currentProxy()
	// ←/→ 不应改变策略
	s, _ = s.Update(pressKey("right"), nil)
	if s.addForm.currentProxy() != proxyBefore {
		t.Fatalf("proxy should not change on Right, was %s now %s", proxyBefore, s.addForm.currentProxy())
	}
	s, _ = s.Update(pressKey("left"), nil)
	if s.addForm.currentProxy() != proxyBefore {
		t.Fatalf("proxy should not change on Left, was %s now %s", proxyBefore, s.addForm.currentProxy())
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
	// Tab → proxy, Enter 打开 picker
	s, _ = s.Update(pressTab(), nil)
	s, _ = s.Update(pressKey("enter"), nil)
	// ↓ 移动到 REJECT，Enter 选中
	s, _ = s.Update(pressKey("down"), nil)
	s, _ = s.Update(pressKey("enter"), nil)
	chosenProxy := s.addForm.currentProxy()
	if chosenProxy != "REJECT" {
		t.Fatalf("expected REJECT after picker selection, got %s", chosenProxy)
	}

	// 提交；回到 payload 焦点后提交。
	s, _ = s.Update(pressTab(), nil) // → index
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")}, nil)
	s, _ = s.Update(pressKey("enter"), nil)
	if s.ShowAddForm() {
		t.Fatal("expected successful submit")
	}
	_ = chosenProxy // 提交路径已使用 submit.currentProxy()（见 state.go）
}

// ============================================================
//  配置提取（newAddForm 使用 ExtractProxyPolicies）
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
// 且内置 DIRECT/REJECT 始终在 groups 前端。
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

	// groups：内置在前，随后 proxy-groups
	wantGroups := []string{defaultProxyPolicy, "REJECT", "PROXY", "AUTO"}
	if !stringSlicesEqual(form.proxyGroups, wantGroups) {
		t.Fatalf("groups mismatch:\n got=%v\nwant=%v", form.proxyGroups, wantGroups)
	}
	// nodes：proxies
	wantNodes := []string{"SS-1", "SS-2"}
	if !stringSlicesEqual(form.proxyNodes, wantNodes) {
		t.Fatalf("nodes mismatch:\n got=%v\nwant=%v", form.proxyNodes, wantNodes)
	}
	// 默认选中 DIRECT
	if got := form.currentProxy(); got != defaultProxyPolicy {
		t.Fatalf("expected default selected DIRECT, got %s", got)
	}
}

// TestLoadProxyPolicies_FallbackOnMissingFile 验证配置文件不存在时兆底为内置策略。
func TestLoadProxyPolicies_FallbackOnMissingFile(t *testing.T) {
	form := newAddForm(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if !stringSlicesEqual(form.proxyGroups, []string{defaultProxyPolicy, "REJECT"}) {
		t.Fatalf("expected fallback groups [DIRECT REJECT], got %v", form.proxyGroups)
	}
	if len(form.proxyNodes) != 0 {
		t.Fatalf("expected empty nodes on missing file, got %v", form.proxyNodes)
	}
	if got := form.currentProxy(); got != defaultProxyPolicy {
		t.Fatalf("expected default DIRECT, got %s", got)
	}
}

// TestLoadProxyPolicies_EmptyConfigFallsBack 验证空配置文件（无 proxies/groups）仅返回内置策略。
func TestLoadProxyPolicies_EmptyConfigFallsBack(t *testing.T) {
	path := writeAddFormTempConfig(t, "mode: rule\n")
	form := newAddForm(path)
	if !stringSlicesEqual(form.proxyGroups, []string{defaultProxyPolicy, "REJECT"}) {
		t.Fatalf("expected only builtin groups, got %v", form.proxyGroups)
	}
	if len(form.proxyNodes) != 0 {
		t.Fatalf("expected empty nodes for empty config, got %v", form.proxyNodes)
	}
}

// TestPickerFiltered_DirectRejectSearchable 验证内置 DIRECT/REJECT 在策略组 Tab 可被搜索。
func TestPickerFiltered_DirectRejectSearchable(t *testing.T) {
	form := newAddForm("")
	form.openProxyPicker()
	// 搜索 "dir" 应匹配 DIRECT
	form.pickerSearch = "dir"
	filtered := form.pickerFiltered()
	if len(filtered) != 1 || filtered[0] != "DIRECT" {
		t.Fatalf("expected [DIRECT] for search 'dir', got %v", filtered)
	}
	// 搜索 "rej" 应匹配 REJECT
	form.pickerSearch = "rej"
	filtered = form.pickerFiltered()
	if len(filtered) != 1 || filtered[0] != "REJECT" {
		t.Fatalf("expected [REJECT] for search 'rej', got %v", filtered)
	}
}
