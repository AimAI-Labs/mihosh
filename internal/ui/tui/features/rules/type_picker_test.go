package rules

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestRuleTypePickerRendersEmptySearch(t *testing.T) {
	s := State{}
	s, _ = s.Update(keyMsg('n'))
	s, _ = s.Update(pressShiftTab())
	s, _ = s.Update(keyMsg(' '))
	require.True(t, s.addForm.typePickerOpen)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!!!")})

	rendered := ansi.Strip(RenderRulesPage(s.ToPageState(80, 22)))
	require.Contains(t, rendered, i18n.T("rules.picker_empty"))

	s, _ = s.Update(pressKey("enter"))
	require.True(t, s.addForm.typePickerOpen)
}
