package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestRuleMouseUsesRenderedWindowAfterKeyScroll(t *testing.T) {
	rules := make([]model.Rule, 40)
	for i := range rules {
		rules[i] = model.Rule{Type: "Domain", Payload: fmt.Sprintf("rule-%02d.example", i), Proxy: "DIRECT"}
	}
	s := (State{}).ApplyRules(rules)
	for i := 0; i < 25; i++ {
		s, _ = s.Update(pressKey("down"))
	}
	const width, height = 100, 18
	page := s.ToPageState(width, height)
	firstY := rulesHeaderHeight + 1
	want := page.SelectedRule - ruleListHeight(page) + 1
	require.Positive(t, want)
	lines := strings.Split(ansi.Strip(RenderRulesPage(page)), "\n")
	require.Contains(t, lines[firstY], rules[want].Payload)

	s, _ = s.HandleMouseLeft(2, firstY, width, height)
	require.Equal(t, want, s.selectedRule)
	s, _ = s.HandleMouseLeft(2, firstY, width, height)
	require.True(t, s.showEditForm)
	require.Equal(t, want, s.editOriginIndex)
}
