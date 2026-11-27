package rules

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
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

func TestTypeFiltersShareLayoutWithTheRuleList(t *testing.T) {
	s := State{}.ApplyRules([]model.Rule{
		{Type: "Domain", Payload: "a.example", Proxy: "DIRECT"},
		{Type: "IPCIDR", Payload: "192.0.2.0/24", Proxy: "REJECT"},
	})
	s, _ = s.Update(keyMsg('t'))
	require.True(t, s.showTypeFilter)
	require.False(t, s.InputCapture().Active)
	for _, width := range []int{45, 80, 120} {
		page := s.ToPageState(width, 28)
		rows, hits := typeFilterLayout(page)
		require.NotEmpty(t, rows)
		view := strings.Split(ansi.Strip(RenderRulesPage(page)), "\n")
		firstRule := rulesHeaderHeight + len(rows) + 1
		require.Contains(t, view[firstRule], "a.example")
		require.Equal(t, 0, ResolveMouseHit(page, firstRule))
		for _, hit := range hits {
			if hit.name == "DOMAIN" {
				selected, _ := s.HandleMouseLeft(hit.x, hit.y, width, 28)
				require.Equal(t, []int{0}, selected.filteredRuleIndices)
			}
			if hit.name == "DOMAIN-SUFFIX" {
				require.False(t, hit.enabled)
				next, _ := s.HandleMouseLeft(hit.x, hit.y, width, 28)
				require.Empty(t, next.selectedTypes)
			}
		}
	}
	s, _ = s.Update(keyMsg(' '))
	require.Equal(t, []string{"DOMAIN"}, s.selectedTypes)
	s, _ = s.Update(keyMsg('t'))
	require.False(t, s.showTypeFilter)
	require.Equal(t, []int{0}, s.filteredRuleIndices)
	s, _ = s.Update(keyMsg('t'))
	s, _ = s.Update(pressKey("esc"))
	require.Empty(t, s.selectedTypes)
	require.Len(t, s.filteredRuleIndices, 2)
}

func TestTypeFilterUpdatesAvailabilityAndKeepsSearchAvailable(t *testing.T) {
	s := State{}.ApplyRules([]model.Rule{{Type: "Domain", Payload: "a.example", Proxy: "DIRECT"}})
	s, _ = s.Update(keyMsg('t'))
	s, _ = s.Update(keyMsg(' '))
	s, _ = s.Update(keyMsg('/'))
	require.True(t, s.ruleFilterMode)
	s = s.ApplyRules([]model.Rule{
		{Type: "ProcessNameRegex", Payload: "curl", Proxy: "DIRECT"},
		{Type: "SubRules", Payload: "example"},
		{Type: "IP-CIDR6", Payload: "2001:db8::/32", Proxy: "DIRECT"},
	})
	require.Empty(t, s.selectedTypes)
	require.Equal(t, []string{"IP-CIDR", "PROCESS-NAME-REGEX", "SUB-RULE"}, s.availableTypes)
}

func TestRulesClickSearchBarFocusesAndOutsideExits(t *testing.T) {
	s := State{}.ApplyRules([]model.Rule{{Type: "Domain", Payload: "a.example", Proxy: "DIRECT"}})
	s, _ = s.HandleMouseLeft(4, 1, 80, 24)
	require.True(t, s.ruleFilterMode)
	s, _ = s.HandleMouseLeft(4, rulesHeaderHeight+1, 80, 24)
	require.False(t, s.ruleFilterMode)
}
