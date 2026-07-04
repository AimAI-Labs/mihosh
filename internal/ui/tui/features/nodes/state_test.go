package nodes

import (
	"strings"
	"testing"

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
	if !next.TestAllActive || next.TestAllTotal != 2 || len(next.TestAllRunning) != 2 {
		t.Fatalf("expected concurrent batch state initialized, active=%v total=%d running=%d", next.TestAllActive, next.TestAllTotal, len(next.TestAllRunning))
	}
}

func TestNodesState_ApplyTestDone_AdvancesBatchTarget(t *testing.T) {
	state := State{
		Testing:        true,
		TestingTarget:  "HK-01（已完成 0/2）",
		TestAllActive:  true,
		TestAllRunning: []string{"HK-01", "JP-01"},
		TestAllTotal:   2,
		TestAllDone:    0,
	}

	state = state.ApplyTestDone("HK-01", 123, nil, "http://test.url")
	if !state.Testing || !strings.Contains(state.TestingTarget, "JP-01") {
		t.Fatalf("expected batch to continue with JP-01, got Testing=%v target=%q", state.Testing, state.TestingTarget)
	}

	state = state.ApplyTestDone("JP-01", 101, nil, "http://test.url")
	if state.Testing || state.TestingTarget != "" || state.TestAllActive {
		t.Fatalf("expected batch to finish and clear status, got Testing=%v target=%q active=%v", state.Testing, state.TestingTarget, state.TestAllActive)
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
