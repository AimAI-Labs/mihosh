package rules

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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
	// Tab → proxy
	s, _ = s.Update(pressTab(), nil)
	// 输入 proxy
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("DIRECT")}, nil)
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

	// Tab → proxy，输入 DIRECT，Enter 应成功（无需 payload）
	s, _ = s.Update(pressTab(), nil)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("DIRECT")}, nil)
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
	// proxy
	s, _ = s.Update(pressTab(), nil)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("DIRECT")}, nil)
	// index = "abc"
	s, _ = s.Update(pressTab(), nil)
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
