package common

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
