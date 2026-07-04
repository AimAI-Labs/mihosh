package tui

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	tea "github.com/charmbracelet/bubbletea"
)

// TestRuleFilterModeDoesNotQuitOnQ 回归测试：
// 在规则页搜索过滤模式下，按 'q' 应被捕获为输入字符，而不是触发全局退出。
func TestRuleFilterModeDoesNotQuitOnQ(t *testing.T) {
	rulesState := newRulesStateWithRules()

	// 进入过滤模式
	rulesState, _ = rulesState.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !rulesState.FilterMode() {
		t.Fatalf("expected rules page to enter filter mode after pressing '/'")
	}

	m := Model{
		currentPage: layout.PageRules,
		rulesState:  rulesState,
	}

	next, cmd := m.Update(keyMsg("q"))
	if cmd != nil {
		t.Fatalf("expected no command (in particular not tea.Quit) when typing 'q' in rules filter mode, got non-nil cmd")
	}

	got := next.(Model)
	if got.rulesState.FilterMode() != true {
		t.Fatalf("filter mode should still be active after typing 'q'")
	}
	// 验证 'q' 真的被当作输入字符追加到过滤文本，而非被丢弃/退出
	if ps := got.rulesState.ToPageState(120, 30); ps.FilterText != "q" {
		t.Fatalf("expected filter text to contain 'q', got %q", ps.FilterText)
	}
}

// TestRulesNormalModeStillQuitsOnQ 确保：非过滤模式下，'q' 仍正常触发全局退出。
// 这防止修复过度——退出功能不应被破坏。
func TestRulesNormalModeStillQuitsOnQ(t *testing.T) {
	m := Model{
		currentPage: layout.PageRules,
		rulesState:  newRulesStateWithRules(),
	}

	_, cmd := m.Update(keyMsg("q"))
	if cmd == nil {
		t.Fatalf("expected quit command when pressing 'q' outside filter mode")
	}
	// tea.Quit 是一个返回 tea.QuitMsg 的 Cmd；执行它应得到 tea.QuitMsg
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected command to produce tea.QuitMsg in normal mode")
	}
}

// newRulesStateWithRules 构造一个带少量规则的初始规则页状态。
func newRulesStateWithRules() rules.State {
	return rules.State{}.ApplyRules([]model.Rule{
		{Type: "DOMAIN", Payload: "a.com", Proxy: "DIRECT"},
		{Type: "DOMAIN-SUFFIX", Payload: "b.com", Proxy: "PROXY"},
	})
}
