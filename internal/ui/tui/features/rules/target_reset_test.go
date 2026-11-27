package rules

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/stretchr/testify/require"
)

func TestRulesResetClosesOldTargetEdits(t *testing.T) {
	s := (State{configPath: "old.yaml", FilterEngine: FilterEngineRegex, ColorAdjustLight: 0.3}).
		ApplyRules([]model.Rule{{Type: "Domain", Payload: "old.example", Proxy: "DIRECT"}})
	s.showEditForm = true
	s.showTypeFilter = true
	s.addForm = newAddForm(s.configPath)
	current, _ := s.Update(messages.CoreTargetChangedMsg{ConfigPath: "same.yaml"})
	require.Len(t, current.rules, 1)
	require.True(t, current.showEditForm)
	require.Equal(t, "same.yaml", current.configPath)

	reset, _ := s.Update(messages.CoreTargetChangedMsg{Reset: true, Invalidate: true, ConfigPath: "new.yaml"})
	require.Empty(t, reset.rules)
	require.Empty(t, reset.filteredRuleIndices)
	require.False(t, reset.InputCapture().Active)
	require.False(t, reset.showTypeFilter)
	require.Equal(t, "new.yaml", reset.configPath)
	require.Equal(t, FilterEngineRegex, reset.FilterEngine)
	require.Equal(t, 0.3, reset.ColorAdjustLight)
}
