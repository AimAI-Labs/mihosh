package rules

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	tea "github.com/charmbracelet/bubbletea"
)

// sampleRulesForFilter 提供一组用于过滤测试的规则样本
func sampleRulesForFilter() []model.Rule {
	return []model.Rule{
		{Type: "DOMAIN", Payload: "example.com", Proxy: "DIRECT"},
		{Type: "DOMAIN-SUFFIX", Payload: "google.com", Proxy: "PROXY"},
		{Type: "IP-CIDR", Payload: "1.2.3.0/24", Proxy: "REJECT"},
		{Type: "DOMAIN-KEYWORD", Payload: "github", Proxy: "PROXY"},
	}
}

// TestRulesState_FilterEngineToggle 验证过滤输入模式下 Ctrl+R / Ctrl+F 的二态切换。
// Ctrl+R 在 普通↔正则 间切换；Ctrl+F 在 普通↔模糊 间切换。
func TestRulesState_FilterEngineToggle(t *testing.T) {
	s := State{}
	s = s.ApplyRules(sampleRulesForFilter())

	// 进入过滤输入模式
	s, _ = s.Update(keyMsg('/'), nil)
	if !s.ruleFilterMode {
		t.Fatal("expected filter mode after pressing '/'")
	}
	if s.FilterEngine != FilterEngineSubstring {
		t.Fatalf("default engine should be Substring, got %d", s.FilterEngine)
	}

	// Ctrl+R → Regex
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlR}, nil)
	if s.FilterEngine != FilterEngineRegex {
		t.Fatalf("expected Regex after Ctrl+R, got %d", s.FilterEngine)
	}

	// Ctrl+R 再次 → Substring
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlR}, nil)
	if s.FilterEngine != FilterEngineSubstring {
		t.Fatalf("expected Substring after second Ctrl+R, got %d", s.FilterEngine)
	}

	// Ctrl+F → Fuzzy
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlF}, nil)
	if s.FilterEngine != FilterEngineFuzzy {
		t.Fatalf("expected Fuzzy after Ctrl+F, got %d", s.FilterEngine)
	}

	// Ctrl+F 再次 → Substring
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlF}, nil)
	if s.FilterEngine != FilterEngineSubstring {
		t.Fatalf("expected Substring after second Ctrl+F, got %d", s.FilterEngine)
	}

	// 可以从一个非普通引擎直接切到另一个：Regex → Ctrl+F → Fuzzy
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlR}, nil) // → Regex
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlF}, nil) // → Fuzzy
	if s.FilterEngine != FilterEngineFuzzy {
		t.Fatalf("expected Fuzzy after Regex→Ctrl+F, got %d", s.FilterEngine)
	}
}

// TestRulesState_UpdateFilteredRules_Engines 直接验证各引擎的匹配行为。
func TestRulesState_UpdateFilteredRules_Engines(t *testing.T) {
	rules := sampleRulesForFilter()

	t.Run("substring_multi_keyword_and", func(t *testing.T) {
		s := State{rules: rules, ruleFilter: "domain direct"}
		s.FilterEngine = FilterEngineSubstring
		s.updateFilteredRules()
		// "domain direct" 仅命中 example.com (DOMAIN + DIRECT)
		if len(s.filteredRuleIndices) != 1 {
			t.Fatalf("expected 1 match for 'domain direct', got %d", len(s.filteredRuleIndices))
		}
		if s.rules[s.filteredRuleIndices[0]].Payload != "example.com" {
			t.Errorf("expected example.com, got %s", s.rules[s.filteredRuleIndices[0]].Payload)
		}
	})

	t.Run("regex_match", func(t *testing.T) {
		// g..gle 使用通配符，匹配 google.com（区别于普通子串）
		s := State{rules: rules, ruleFilter: `g..gle`}
		s.FilterEngine = FilterEngineRegex
		s.updateFilteredRules()
		if len(s.filteredRuleIndices) != 1 {
			t.Fatalf("expected 1 regex match for 'g..gle', got %d", len(s.filteredRuleIndices))
		}
		if s.rules[s.filteredRuleIndices[0]].Payload != "google.com" {
			t.Errorf("expected google.com, got %s", s.rules[s.filteredRuleIndices[0]].Payload)
		}
	})

	t.Run("regex_case_insensitive", func(t *testing.T) {
		s := State{rules: rules, ruleFilter: `domain`}
		s.FilterEngine = FilterEngineRegex
		s.updateFilteredRules()
		// (?i)domain 命中 DOMAIN / DOMAIN-SUFFIX / DOMAIN-KEYWORD 三条
		if len(s.filteredRuleIndices) != 3 {
			t.Fatalf("expected 3 case-insensitive regex matches, got %d", len(s.filteredRuleIndices))
		}
	})

	t.Run("regex_invalid_matches_nothing", func(t *testing.T) {
		s := State{rules: rules, ruleFilter: `(unclosed`}
		s.FilterEngine = FilterEngineRegex
		s.updateFilteredRules()
		// 非法正则：文本过滤结果为空
		if len(s.filteredRuleIndices) != 0 {
			t.Fatalf("expected 0 matches for invalid regex, got %d", len(s.filteredRuleIndices))
		}
	})

	t.Run("fuzzy_subsequence", func(t *testing.T) {
		// "ggl" 是 "google.com" 的有序子序列
		s := State{rules: rules, ruleFilter: "ggl"}
		s.FilterEngine = FilterEngineFuzzy
		s.updateFilteredRules()
		if len(s.filteredRuleIndices) != 1 {
			t.Fatalf("expected 1 fuzzy match for 'ggl', got %d", len(s.filteredRuleIndices))
		}
		if s.rules[s.filteredRuleIndices[0]].Payload != "google.com" {
			t.Errorf("expected google.com, got %s", s.rules[s.filteredRuleIndices[0]].Payload)
		}
	})

	t.Run("fuzzy_no_match", func(t *testing.T) {
		// "xyz" 不作为任何字段子序列出现
		s := State{rules: rules, ruleFilter: "xyz"}
		s.FilterEngine = FilterEngineFuzzy
		s.updateFilteredRules()
		if len(s.filteredRuleIndices) != 0 {
			t.Fatalf("expected 0 fuzzy matches for 'xyz', got %d", len(s.filteredRuleIndices))
		}
	})
}

// TestRulesState_FilterInputAcceptsMultibyte 验证过滤输入接受多字节字符（如中文）。
func TestRulesState_FilterInputAcceptsMultibyte(t *testing.T) {
	s := State{}
	s = s.ApplyRules(sampleRulesForFilter())
	s, _ = s.Update(keyMsg('/'), nil)

	// 输入一个中文字符（多字节）
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'搜'}}, nil)
	if s.ruleFilter != "搜" {
		t.Fatalf("expected filter text '搜', got %q", s.ruleFilter)
	}
}
