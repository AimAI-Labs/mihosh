package common

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestSearchMatcherPreservesPageSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, query, text string
		engine            SearchEngine
		mode              SearchMatchMode
		want              bool
	}{
		{"empty", "", "anything", SearchRegex, MatchWholeSubstring, true},
		{"case", "HK", "hk node", SearchSubstring, MatchWholeSubstring, true},
		{"whole substring", "HK SG", "hk west sg", SearchSubstring, MatchWholeSubstring, false},
		{"keyword AND", "HK SG", "hk west sg", SearchSubstring, MatchAllKeywords, true},
		{"missing keyword", "HK US", "hk west sg", SearchSubstring, MatchAllKeywords, false},
		{"whitespace keywords", "  ", "node", SearchSubstring, MatchAllKeywords, true},
		{"regex", "^HK", "hk node", SearchRegex, MatchAllKeywords, true},
		{"invalid regex", "[", "node", SearchRegex, MatchAllKeywords, false},
		{"fuzzy", "h1", "HK-01", SearchFuzzy, MatchWholeSubstring, true},
		{"Chinese", "中文", "中文节点", SearchSubstring, MatchWholeSubstring, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			match := (SearchQuery{Text: tc.query, Engine: tc.engine}).Matcher(tc.mode)
			if got := match(tc.text); got != tc.want {
				t.Fatalf("match=%t, want %t", got, tc.want)
			}
		})
	}
}

func TestSearchEdit(t *testing.T) {
	query := SearchQuery{}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("中")}, {Type: tea.KeyRunes, Runes: []rune("文")}, {Type: tea.KeyBackspace},
	} {
		if !query.Edit(key) {
			t.Fatal("printable input or backspace was ignored")
		}
	}
	if query.Text != "中" {
		t.Fatalf("query=%q", query.Text)
	}
	for _, tc := range []struct {
		key  tea.KeyType
		want SearchEngine
	}{
		{tea.KeyCtrlR, SearchRegex}, {tea.KeyCtrlF, SearchFuzzy}, {tea.KeyCtrlF, SearchSubstring},
		{tea.KeyCtrlR, SearchRegex}, {tea.KeyCtrlR, SearchSubstring},
	} {
		if !query.Edit(tea.KeyMsg{Type: tc.key}) || query.Engine != tc.want {
			t.Fatalf("engine=%v, want %v", query.Engine, tc.want)
		}
	}
	if query.Edit(tea.KeyMsg{Type: tea.KeyEsc}) || query.Edit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("paste")}) {
		t.Fatal("page controls and multi-character input must retain their existing handling")
	}
	query.Text = ""
	if query.Edit(tea.KeyMsg{Type: tea.KeyBackspace}) {
		t.Fatal("empty backspace changed the query")
	}
}

func TestSearchHandleKey(t *testing.T) {
	q := SearchQuery{}
	if act := q.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}); act != SearchActionChanged || q.Text != "a" {
		t.Fatalf("expected SearchActionChanged and text 'a', got act=%v text=%q", act, q.Text)
	}
	if act := q.HandleKey(tea.KeyMsg{Type: tea.KeyCtrlR}); act != SearchActionChanged || q.Engine != SearchRegex {
		t.Fatalf("expected SearchActionChanged and SearchRegex, got act=%v engine=%v", act, q.Engine)
	}
	if act := q.HandleKey(tea.KeyMsg{Type: tea.KeyEnter}); act != SearchActionConfirm {
		t.Fatalf("expected SearchActionConfirm, got %v", act)
	}
	if act := q.HandleKey(tea.KeyMsg{Type: tea.KeyEsc}); act != SearchActionCancel {
		t.Fatalf("expected SearchActionCancel, got %v", act)
	}
	if act := q.HandleKey(tea.KeyMsg{Type: tea.KeyUp}); act != SearchActionNone {
		t.Fatalf("expected SearchActionNone, got %v", act)
	}
}

func TestRenderSearchHeaderBarAndHits(t *testing.T) {
	box := RenderSearchBox(SearchBoxOptions{
		Label:     "搜索: ",
		Query:     SearchQuery{Text: "中文测试关键词很长需要截断", Engine: SearchFuzzy},
		Active:    true,
		Hint:      " hint",
		HintRegex: " regex",
		HintFuzzy: " fuzzy",
		Suffix:    " [DOMAIN]",
	})
	plainBox := ansi.Strip(box)
	if !strings.Contains(plainBox, "搜索: 中文测试关键词很长需要截断█ [Fuzzy] [DOMAIN]") {
		t.Fatalf("unexpected rendered search box: %q", plainBox)
	}

	for _, trailing := range []string{"", " ⊞ │ ◫ "} {
		for _, stats := range []string{"", "共 12 条"} {
			for _, width := range []int{20, 32, 80, 120} {
				bar := RenderSearchHeaderBar(box, stats, trailing, width)
				lines := strings.Split(ansi.Strip(bar), "\n")
				if len(lines) != SearchHeaderHeight {
					t.Fatalf("width=%d expected %d lines, got %d", width, SearchHeaderHeight, len(lines))
				}
				for i, line := range lines {
					if w := lipgloss.Width(line); w != width {
						t.Fatalf("width=%d trailing=%q stats=%q line %d width=%d, want %d: %q",
							width, trailing, stats, i, w, width, line)
					}
				}
			}
		}
	}

	if !HitSearchHeaderBar(1, 1, 40, 6) || HitSearchHeaderBar(33, 1, 40, 6) || HitSearchHeaderBar(1, 0, 40, 6) {
		t.Fatal("unexpected HitSearchHeaderBar bounds result")
	}
	if !HitBarBlankSpace(20, 1, 80, 18, 8) || HitBarBlankSpace(18, 1, 80, 18, 8) || HitBarBlankSpace(71, 1, 80, 18, 8) {
		t.Fatal("unexpected HitBarBlankSpace bounds result")
	}
}
