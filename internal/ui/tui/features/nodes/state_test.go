package nodes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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

func TestNodesLocateActiveProxyModesLayoutsAndFilters(t *testing.T) {
	i18n.Init()
	manyNodes := make([]string, 0, 30)
	for i := 0; i < 30; i++ {
		manyNodes = append(manyNodes, fmt.Sprintf("HK-%02d", i))
	}
	groups := map[string]model.Group{
		"GLOBAL": {Name: "GLOBAL", Type: "Selector", Now: "US-01", All: []string{"HK-00", "US-01", "Proxy"}},
		"Auto":   {Name: "Auto", Type: "URLTest", Now: "HK-25", All: manyNodes},
		"Proxy":  {Name: "Proxy", Type: "Selector", Now: "Auto", All: append([]string{"Auto"}, manyNodes...)},
		"Media":  {Name: "Media", Type: "Selector", Now: "US-01", All: []string{"US-01", "JP-01"}},
	}
	ordered := []string{"GLOBAL", "Auto", "Proxy", "Media"}

	// Rule mode: jumps to priority group "Proxy" -> "Auto", second press drills down to "Auto" -> "HK-25".
	s := State{Mode: "rule"}.ApplyGroups(groups, ordered)
	s.applyGroupSelection(3) // Start on Media
	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if s.GroupNames[s.SelectedGroup] != "Proxy" || s.displayProxies()[s.SelectedProxy] != "Auto" {
		t.Fatalf("rule mode first locate expected Proxy -> Auto, got %s -> %s", s.GroupNames[s.SelectedGroup], s.displayProxies()[s.SelectedProxy])
	}
	if cmd == nil {
		t.Fatal("expected info notice command on locate")
	}
	if notice, ok := cmd().(messages.NoticeMsg); !ok || !strings.Contains(notice.Text, "Auto") {
		t.Fatalf("unexpected locate notice: %#v", notice)
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if s.GroupNames[s.SelectedGroup] != "Auto" || s.displayProxies()[s.SelectedProxy] != "HK-25" {
		t.Fatalf("rule mode second locate expected drill-down Auto -> HK-25, got %s -> %s", s.GroupNames[s.SelectedGroup], s.displayProxies()[s.SelectedProxy])
	}

	// Both layouts render the located deep node visibly.
	for _, layout := range []NodesLayout{LayoutCards, LayoutColumns} {
		s.Layout = layout
		rendered := ansi.Strip(RenderNodesPage(s.ToPageState(120, 26)))
		if !strings.Contains(rendered, "HK-25") {
			t.Fatalf("layout %v did not render located active node HK-25", layout)
		}
	}

	// Global mode jumps to GLOBAL -> US-01.
	gs := State{Mode: "global"}.ApplyGroups(groups, ordered)
	gs.applyGroupSelection(3)
	gs, _ = gs.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if gs.GroupNames[gs.SelectedGroup] != "GLOBAL" || gs.displayProxies()[gs.SelectedProxy] != "US-01" {
		t.Fatalf("global mode expected GLOBAL -> US-01, got %s -> %s", gs.GroupNames[gs.SelectedGroup], gs.displayProxies()[gs.SelectedProxy])
	}

	// Direct mode keeps selection unchanged and returns an info notice.
	ds := State{Mode: "direct"}.ApplyGroups(groups, ordered)
	ds.applyGroupSelection(3)
	ds, cmd = ds.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if ds.SelectedGroup != 3 || cmd == nil {
		t.Fatalf("direct mode should not jump and should notify")
	}
	if notice, ok := cmd().(messages.NoticeMsg); !ok || notice.Level != messages.NoticeInfo {
		t.Fatalf("expected direct mode info notice, got %#v", notice)
	}

	// Search filter: preserves matching filter, clears non-matching filter.
	fs := State{Mode: "global", NodeFilter: "us-01"}.ApplyGroups(groups, ordered)
	fs, _ = fs.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if fs.NodeFilter != "us-01" || fs.displayProxies()[fs.SelectedProxy] != "US-01" {
		t.Fatalf("matching search filter should be preserved, got filter=%q", fs.NodeFilter)
	}
	fs.NodeFilter = "nomatch"
	fs.updateFilteredProxies()
	fs, _ = fs.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if fs.NodeFilter != "" || fs.displayProxies()[fs.SelectedProxy] != "US-01" {
		t.Fatalf("non-matching filter should clear to locate active node, got filter=%q", fs.NodeFilter)
	}

	// SortOrderAvailable hiding untested active node resets sort to locate it.
	as := State{Mode: "global", ProxySortOrder: SortOrderAvailable}.ApplyGroups(groups, ordered)
	as, _ = as.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if as.ProxySortOrder != SortOrderOriginal || as.displayProxies()[as.SelectedProxy] != "US-01" {
		t.Fatalf("available-only sort should reset to locate untested active node")
	}
}
