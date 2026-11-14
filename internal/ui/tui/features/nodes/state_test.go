package nodes

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNodesState_TestAllStartsWithFirstProxyTarget(t *testing.T) {
	state := State{
		GroupNames:     []string{"Auto"},
		SelectedGroup:  0,
		CurrentProxies: []string{"HK-01", "JP-01"},
	}

	next, _ := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if !next.Testing {
		t.Fatalf("expected Testing=true after pressing a")
	}
	if !strings.Contains(next.TestingTarget, "HK-01") {
		t.Fatalf("expected TestingTarget contains first proxy, got %q", next.TestingTarget)
	}
	_, total := next.testBatch.Progress()
	if !next.testBatch.Active() || total != 2 {
		t.Fatalf("expected active batch with two tasks, total=%d", total)
	}
}

func TestNodesState_ApplyTestDone_AdvancesBatchTarget(t *testing.T) {
	state := State{CurrentProxies: []string{"HK-01", "JP-01"}}
	state, cmd := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	commands := cmd().(tea.BatchMsg)
	for i, command := range commands {
		result := command().(messages.TestDoneMsg)
		result.Delay, result.Err = 123, nil
		state, _ = state.CompleteTest(result)
		if i == 0 && (!state.Testing || !strings.Contains(state.TestingTarget, "JP-01")) {
			t.Fatalf("expected batch to continue with JP-01, target=%q", state.TestingTarget)
		}
	}
	if state.Testing || state.TestingTarget != "" || state.testBatch.Active() {
		t.Fatal("expected completed batch to clear testing status")
	}
}

func TestNodesState_DetailModalSupportsHomeAndEnd(t *testing.T) {
	state := State{
		ShowTestDetail:  true,
		DetailScrollTop: 5,
	}

	homeMsg := tea.KeyMsg{Type: tea.KeyHome}
	state, _ = state.Update(homeMsg)
	if state.DetailScrollTop != 0 {
		t.Fatalf("expected home to jump top, got %d", state.DetailScrollTop)
	}

	endMsg := tea.KeyMsg{Type: tea.KeyEnd}
	state, _ = state.Update(endMsg)
	if state.DetailScrollTop <= 0 {
		t.Fatalf("expected end to move scroll near bottom, got %d", state.DetailScrollTop)
	}
}

func TestNodesState_FilterEngineToggle(t *testing.T) {
	state := State{
		NodeFilterMode: true,
	}

	// Toggle Regex
	state, _ = state.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if state.FilterEngine != FilterEngineRegex {
		t.Fatalf("expected FilterEngineRegex after Ctrl+R, got %v", state.FilterEngine)
	}

	// Toggle Fuzzy
	state, _ = state.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if state.FilterEngine != FilterEngineFuzzy {
		t.Fatalf("expected FilterEngineFuzzy after Ctrl+F, got %v", state.FilterEngine)
	}

	// Toggle Back to Substring
	state, _ = state.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if state.FilterEngine != FilterEngineSubstring {
		t.Fatalf("expected FilterEngineSubstring after second Ctrl+F, got %v", state.FilterEngine)
	}
}

func TestNodesState_UpdateFilteredProxies(t *testing.T) {
	state := State{
		CurrentProxies: []string{"HK-01", "HK-02", "SG-01", "US-01"},
		NodeFilter:     "^hk",
		FilterEngine:   FilterEngineRegex,
	}

	state.updateFilteredProxies()
	if len(state.FilteredProxyIndices) != 2 {
		t.Fatalf("regex expected 2 matches, got %d", len(state.FilteredProxyIndices))
	}

	state.NodeFilter = "^hk(" // invalid regex
	state.updateFilteredProxies()
	if len(state.FilteredProxyIndices) != 0 {
		t.Fatalf("invalid regex expected 0 matches, got %d", len(state.FilteredProxyIndices))
	}

	state.FilterEngine = FilterEngineFuzzy
	state.NodeFilter = "h1" // should match HK-01
	state.updateFilteredProxies()
	if len(state.FilteredProxyIndices) != 1 {
		t.Fatalf("fuzzy expected 1 match, got %d", len(state.FilteredProxyIndices))
	}
	if state.FilteredProxyIndices[0] != 0 {
		t.Fatalf("fuzzy expected HK-01 (index 0), got %d", state.FilteredProxyIndices[0])
	}
}

func TestNodeSearchKeepsWholeSubstringAndClearsOnEscape(t *testing.T) {
	s := State{CurrentProxies: []string{"HK west SG", "HK SG", "other"}, NodeFilterMode: true, NodeFilter: "hk sg"}
	s.updateFilteredProxies()
	if len(s.FilteredProxyIndices) != 1 || s.FilteredProxyIndices[0] != 1 {
		t.Fatalf("node search must match the whole substring: %v", s.FilteredProxyIndices)
	}
	s.NodeFilter = "中文"
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if s.NodeFilter != "中" {
		t.Fatalf("backspace split a Chinese character: %q", s.NodeFilter)
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if s.NodeFilterMode || s.NodeFilter != "" || len(s.displayProxies()) != 3 {
		t.Fatal("node Escape must leave search and clear the query")
	}
}
