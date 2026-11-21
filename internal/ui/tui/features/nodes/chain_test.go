package nodes

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func newChainTestState() State {
	s := NewState(nil, "http://www.gstatic.com/generate_204", 5000)
	s.Mode = "rule"
	s.GroupNames = []string{"PROXY", "RELAY"}
	s.Groups = map[string]model.Group{
		"PROXY": {Name: "PROXY", Type: "Selector", Now: "HK-01", All: []string{"DIRECT", "HK-01", "SG-02", "US-03", "RELAY"}},
		"RELAY": {Name: "RELAY", Type: "URLTest", Now: "HK-01", All: []string{"HK-01"}},
	}
	s.Proxies = map[string]model.Proxy{
		"HK-01": {Name: "HK-01", Type: "Shadowsocks", History: []model.Delay{{Delay: 42}}},
		"SG-02": {Name: "SG-02", Type: "VMess", History: []model.Delay{{Delay: 85}}},
		"US-03": {Name: "US-03", Type: "Trojan", History: []model.Delay{{Delay: 160}}},
	}
	s.SelectedGroup = 0
	s.updateCurrentProxies()
	return s
}

func TestNodesChainModalInteractionAndRendering(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	s := newChainTestState()

	// Press 'c' opens chain modal for currently selected group and captures input.
	var cmd tea.Cmd
	s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	require.Nil(t, cmd)
	require.True(t, s.ChainModal.Open)
	require.Equal(t, "PROXY", s.ChainModal.Group)
	require.True(t, s.InputCapture().Active)
	// Built-in DIRECT must be excluded from candidates.
	for _, cand := range s.ChainModal.Candidates {
		require.NotEqual(t, "DIRECT", cand.Name)
	}

	// Add HK-01 (cursor 0) and SG-02 (cursor 1) to draft chain.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, []string{"HK-01"}, s.ChainModal.DraftNodes)

	// Attempting to apply with only 1 node shows warning instead of submitting.
	s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.Nil(t, cmd)
	require.False(t, s.ChainModal.Applying)
	require.NotEmpty(t, s.ChainModal.Notice)

	// Move down and add SG-02, then US-03.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyDown})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyDown})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, []string{"HK-01", "SG-02", "US-03"}, s.ChainModal.DraftNodes)

	// Switch focus to hops pane and reorder last hop earlier via '['.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.Equal(t, chainFocusHops, s.ChainModal.Focus)
	require.Equal(t, 2, s.ChainModal.HopCursor)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	require.Equal(t, []string{"HK-01", "US-03", "SG-02"}, s.ChainModal.DraftNodes)
	require.Equal(t, 1, s.ChainModal.HopCursor)

	// Render modal cleanly on both narrow and wide viewports.
	wideOut := RenderNodesPage(s.ToPageState(120, 30))
	require.Contains(t, wideOut, "HK-01")
	require.Contains(t, wideOut, "US-03")
	narrowOut := RenderNodesPage(s.ToPageState(60, 20))
	require.NotEmpty(t, narrowOut)

	// Remove middle hop via 'd'.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	require.Equal(t, []string{"HK-01", "SG-02"}, s.ChainModal.DraftNodes)

	// Submit valid chain triggers async command.
	s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)
	require.True(t, s.ChainModal.Applying)

	// Simulate successful result with persisted chain-edits.yaml.
	require.NoError(t, profile.WriteChainEdits(profile.ChainEdits{
		Chains: []profile.ProxyChain{{
			Group:   "PROXY",
			Nodes:   []string{"HK-01", "SG-02"},
			Enabled: true,
		}},
	}))
	s = s.ApplyChainResult(messages.ProxyChainAppliedMsg{Group: "PROXY", ExitNode: "SG-02", Applied: true})
	require.False(t, s.ChainModal.Open)
	activeChain, ok := s.ActiveGroupChain()
	require.True(t, ok)
	require.Equal(t, []string{"HK-01", "SG-02"}, activeChain.Nodes)

	// Main page displays active chain badge.
	pageOut := RenderNodesPage(s.ToPageState(120, 30))
	require.Contains(t, pageOut, "HK-01 → SG-02")
}
