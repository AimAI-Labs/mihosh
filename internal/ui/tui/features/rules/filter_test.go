package rules

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	tea "github.com/charmbracelet/bubbletea"
)

// keyMsg helper: create a tea.KeyMsg from a single rune
func keyMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// pressKey helper: create a tea.KeyMsg from a key name
func pressKey(name string) tea.KeyMsg {
	switch name {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	}
	return tea.KeyMsg{}
}

func TestTypeFilterAppliesToRules(t *testing.T) {
	rules := []model.Rule{
		{Type: "DOMAIN", Payload: "a.com", Proxy: "DIRECT"},
		{Type: "DOMAIN-SUFFIX", Payload: "b.com", Proxy: "PROXY"},
		{Type: "IP-CIDR", Payload: "1.2.3.0/24", Proxy: "REJECT"},
		{Type: "DOMAIN", Payload: "c.com", Proxy: "DIRECT"},
	}

	s := State{}
	s = s.ApplyRules(rules)

	// Press 't' to open type filter overlay
	s, _ = s.Update(keyMsg('t'))
	if !s.showTypeFilter {
		t.Fatalf("type filter overlay should be open after pressing 't'")
	}

	// Press Space to select first type
	s, _ = s.Update(keyMsg(' '))
	if len(s.selectedTypes) != 1 {
		t.Fatalf("expected 1 selected type, got %v", s.selectedTypes)
	}

	// Press Enter to confirm
	s, _ = s.Update(pressKey("enter"))
	if s.showTypeFilter {
		t.Fatal("overlay should be closed after Enter")
	}
	if len(s.filteredRuleIndices) == 0 {
		t.Fatalf("expected filtered rules to be non-empty after selecting a type")
	}
}

// TestTypeFilterRenderReflectsSelection checks that after selecting a type and
// confirming, the rendered page only shows matching rules.
func TestTypeFilterRenderReflectsSelection(t *testing.T) {
	rules := []model.Rule{
		{Type: "DOMAIN", Payload: "a.com", Proxy: "DIRECT"},
		{Type: "DOMAIN-SUFFIX", Payload: "b.com", Proxy: "PROXY"},
		{Type: "IP-CIDR", Payload: "1.2.3.0/24", Proxy: "REJECT"},
		{Type: "DOMAIN", Payload: "c.com", Proxy: "DIRECT"},
	}

	s := State{}
	s = s.ApplyRules(rules)

	// Open overlay
	s, _ = s.Update(keyMsg('t'))

	// Find the DOMAIN type index in availableTypes and navigate to it
	targetIdx := 0
	for i, ty := range s.availableTypes {
		if ty == "DOMAIN" {
			targetIdx = i
			break
		}
	}
	for s.typeFilterCursor < targetIdx {
		s, _ = s.Update(pressKey("down"))
	}
	// Select it
	s, _ = s.Update(keyMsg(' '))
	// Confirm
	s, _ = s.Update(pressKey("enter"))

	pageState := s.ToPageState(120, 30)
	rendered := RenderRulesPage(pageState)

	if !strings.Contains(rendered, "a.com") {
		t.Errorf("expected a.com in rendered output")
	}
	if !strings.Contains(rendered, "c.com") {
		t.Errorf("expected c.com in rendered output")
	}
	if strings.Contains(rendered, "b.com") {
		t.Errorf("DOMAIN-SUFFIX rule should NOT appear after filtering to DOMAIN")
	}
	if strings.Contains(rendered, "1.2.3.0/24") {
		t.Errorf("IP-CIDR rule should NOT appear after filtering to DOMAIN")
	}
}

// TestTypeFilterEscClearsSelection ensures Esc in overlay cancels the pending selection
// and returns to an unfiltered state (consistent with normal-mode Esc).
func TestTypeFilterEscClearsSelection(t *testing.T) {
	rules := []model.Rule{
		{Type: "DOMAIN", Payload: "a.com", Proxy: "DIRECT"},
		{Type: "DOMAIN-SUFFIX", Payload: "b.com", Proxy: "PROXY"},
		{Type: "IP-CIDR", Payload: "1.2.3.0/24", Proxy: "REJECT"},
	}

	s := State{}
	s = s.ApplyRules(rules)

	// Open overlay, select a type, then press Esc
	s, _ = s.Update(keyMsg('t'))
	s, _ = s.Update(keyMsg(' '))
	s, _ = s.Update(pressKey("esc"))

	if s.showTypeFilter {
		t.Fatal("overlay should be closed after Esc")
	}
	if len(s.selectedTypes) != 0 {
		t.Fatalf("Esc should cancel selection, got %v", s.selectedTypes)
	}
	// filteredRuleIndices must reflect no filter
	if len(s.filteredRuleIndices) != len(rules) {
		t.Fatalf("expected all %d rules after Esc cancel, got %d", len(rules), len(s.filteredRuleIndices))
	}
}

// TestTypeFilterReopenPreservesSelection ensures reopening 't' keeps the previously
// applied filter so the user can add/remove types incrementally.
func TestTypeFilterReopenPreservesSelection(t *testing.T) {
	rules := []model.Rule{
		{Type: "DOMAIN", Payload: "a.com", Proxy: "DIRECT"},
		{Type: "DOMAIN-SUFFIX", Payload: "b.com", Proxy: "PROXY"},
		{Type: "IP-CIDR", Payload: "1.2.3.0/24", Proxy: "REJECT"},
		{Type: "DOMAIN", Payload: "c.com", Proxy: "DIRECT"},
	}

	s := State{}
	s = s.ApplyRules(rules)

	// Open, select DOMAIN, confirm
	s, _ = s.Update(keyMsg('t'))
	s, _ = s.Update(keyMsg(' '))
	s, _ = s.Update(pressKey("enter"))

	if len(s.selectedTypes) != 1 {
		t.Fatalf("expected 1 selected type, got %v", s.selectedTypes)
	}

	// Reopen 't' — selection should be preserved so the user can add more types
	s, _ = s.Update(keyMsg('t'))
	if len(s.selectedTypes) != 1 {
		t.Fatalf("reopening 't' should preserve selection, got %v", s.selectedTypes)
	}
	// availableTypes should still be populated
	if len(s.availableTypes) == 0 {
		t.Fatal("availableTypes should be populated on reopen")
	}
}

// TestTypeFilterMouseDoubleClickToggles verifies that two consecutive left-clicks
// (within the double-click threshold) on the same list item toggle its selection.
// A single click should only move the cursor without toggling.
func TestTypeFilterMouseDoubleClickToggles(t *testing.T) {
	rules := []model.Rule{
		{Type: "DOMAIN", Payload: "a.com", Proxy: "DIRECT"},
		{Type: "DOMAIN-SUFFIX", Payload: "b.com", Proxy: "PROXY"},
		{Type: "IP-CIDR", Payload: "1.2.3.0/24", Proxy: "REJECT"},
	}

	const pageW, pageH = 120, 30

	s := State{}
	s = s.ApplyRules(rules)

	// Open overlay
	s, _ = s.Update(keyMsg('t'))
	if !s.showTypeFilter {
		t.Fatal("overlay should be open after pressing 't'")
	}

	// Resolve bounds + first list-item coords (consistent with rendering).
	// 列表上方留一空行分隔标题边框，故列表首行坐标为 top+2。
	pageState := s.ToPageState(pageW, pageH)
	left, top, right, bottom := ResolveTypeFilterModalBounds(pageState, pageW, pageH)
	firstItemY := ResolveTypeFilterListItemAt(pageState, left+3, top+2, pageW, pageH)
	if firstItemY != 0 {
		t.Fatalf("expected first list item index 0, got %d", firstItemY)
	}

	// First click — should move cursor but NOT select
	s, _ = s.HandleMouseLeft(left+3, top+2, pageW, pageH)
	if s.typeFilterCursor != 0 {
		t.Fatalf("cursor should be 0 after first click, got %d", s.typeFilterCursor)
	}
	if len(s.selectedTypes) != 0 {
		t.Fatalf("single click should NOT toggle selection, got %v", s.selectedTypes)
	}

	// Second click (immediately, within threshold) — should toggle selection ON
	s, _ = s.HandleMouseLeft(left+3, top+2, pageW, pageH)
	if len(s.selectedTypes) != 1 {
		t.Fatalf("double click should select the type, got %v", s.selectedTypes)
	}

	// Simulate a pause longer than the threshold so the next click starts fresh.
	// This makes the following two clicks a *new* double-click that toggles OFF.
	s.typeFilterDC = common.DoubleClickDetector[struct{}]{}

	// Third click — starts a new double-click window, only moves cursor (still selected)
	s, _ = s.HandleMouseLeft(left+3, top+2, pageW, pageH)
	if len(s.selectedTypes) != 1 {
		t.Fatalf("single click should NOT toggle selection, got %v", s.selectedTypes)
	}

	// Fourth click — completes the new double-click, toggles selection OFF
	s, _ = s.HandleMouseLeft(left+3, top+2, pageW, pageH)
	if len(s.selectedTypes) != 0 {
		t.Fatalf("second double-click should deselect, got %v", s.selectedTypes)
	}

	_ = right
	_ = bottom
}

// TestTypeFilterMouseOutsideClosesAndKeepsFilter verifies that clicking outside
// the modal bounds closes it while preserving the selected types (behaves like Enter).
func TestTypeFilterMouseOutsideClosesAndKeepsFilter(t *testing.T) {
	rules := []model.Rule{
		{Type: "DOMAIN", Payload: "a.com", Proxy: "DIRECT"},
		{Type: "DOMAIN-SUFFIX", Payload: "b.com", Proxy: "PROXY"},
		{Type: "IP-CIDR", Payload: "1.2.3.0/24", Proxy: "REJECT"},
		{Type: "DOMAIN", Payload: "c.com", Proxy: "DIRECT"},
	}

	const pageW, pageH = 120, 30

	s := State{}
	s = s.ApplyRules(rules)

	// Open overlay and select the first type via keyboard
	s, _ = s.Update(keyMsg('t'))
	s, _ = s.Update(keyMsg(' '))
	if len(s.selectedTypes) != 1 {
		t.Fatalf("expected 1 selected type after Space, got %v", s.selectedTypes)
	}

	// Compute bounds; click at top-left corner of the page (guaranteed outside)
	pageState := s.ToPageState(pageW, pageH)
	left, top, _, _ := ResolveTypeFilterModalBounds(pageState, pageW, pageH)
	outsideX, outsideY := 0, 0
	if outsideX >= left && outsideX < left+1 {
		outsideX = left - 1
	}
	if outsideY >= top && outsideY < top+1 {
		outsideY = top - 1
	}

	s, _ = s.HandleMouseLeft(outsideX, outsideY, pageW, pageH)

	if s.showTypeFilter {
		t.Fatal("overlay should be closed after clicking outside")
	}
	if len(s.selectedTypes) != 1 {
		t.Fatalf("selection should be preserved (1 type), got %v", s.selectedTypes)
	}
	// filteredRuleIndices should reflect the active filter
	if len(s.filteredRuleIndices) == 0 || len(s.filteredRuleIndices) == len(rules) {
		t.Fatalf("expected a filtered subset, got %d of %d", len(s.filteredRuleIndices), len(rules))
	}
}
