package tui

import (
	"errors"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

var coreDependentPages = []layout.PageType{
	layout.PageNodes,
	layout.PageConnections,
	layout.PageLogs,
	layout.PageRules,
	layout.PageSub,
}

func placeholderModel(page layout.PageType) Model {
	return Model{
		currentPage:       page,
		width:             100,
		height:            30,
		connectionChecked: true,
		connectionReady:   false,
	}
}

func TestDisconnectedCoreRendersPlaceholderOnCoreDependentPages(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	for _, page := range coreDependentPages {
		m := placeholderModel(page)
		require.True(t, m.coreUnavailable(), "page %d should show the placeholder", page)
		require.Contains(t, ansi.Strip(m.View()), i18n.T("placeholder.title"), "page %d", page)
		require.Empty(t, m.mainPageLayout().help, "page %d should hide unusable shortcuts", page)
	}
}

func TestConnectedCoreRendersRealPages(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	m := placeholderModel(layout.PageNodes)
	m.connectionReady = true
	require.False(t, m.coreUnavailable())
	require.NotContains(t, ansi.Strip(m.View()), i18n.T("placeholder.title"))
}

func TestUncheckedConnectionKeepsPageUntilFirstCheck(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	m := Model{currentPage: layout.PageNodes, width: 100, height: 30}
	require.False(t, m.coreUnavailable())
	require.NotContains(t, ansi.Strip(m.View()), i18n.T("placeholder.title"))
}

func TestCoreAndSettingsPagesNeverShowPlaceholder(t *testing.T) {
	for _, page := range []layout.PageType{layout.PageCore, layout.PageSettings} {
		m := placeholderModel(page)
		require.False(t, m.coreUnavailable(), "page %d must stay usable", page)
	}
}

func TestPlaceholderTickAdvancesAnimationWhileDisconnected(t *testing.T) {
	m := placeholderModel(layout.PageNodes)
	next, cmd := m.Update(messages.PlaceholderTickMsg{})
	advanced := next.(Model)
	require.Equal(t, 1, advanced.placeholderState.Frame())
	require.NotNil(t, cmd, "animation must keep ticking while the placeholder is visible")

	connected := placeholderModel(layout.PageNodes)
	connected.connectionReady = true
	next, cmd = connected.Update(messages.PlaceholderTickMsg{})
	require.Nil(t, cmd, "animation must stop once the core is connected")
	require.Equal(t, 0, next.(Model).placeholderState.Frame())
}

func TestCoreConnectionFailureShowsPlaceholderAndRecoveryRestoresPage(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	m := Model{currentPage: layout.PageNodes, width: 100, height: 30}

	next, cmd := m.Update(messages.CoreConnectionMsg{Epoch: 0, Err: errors.New("connection refused"), KeepPage: true})
	m = next.(Model)
	require.True(t, m.coreUnavailable())
	require.NotNil(t, cmd, "placeholder must start its animation")
	require.Contains(t, ansi.Strip(m.View()), i18n.T("placeholder.title"))

	next, _ = m.Update(messages.CoreConnectionMsg{Epoch: 0, Version: "v1.20.0"})
	m = next.(Model)
	require.False(t, m.coreUnavailable())
	require.NotContains(t, ansi.Strip(m.View()), i18n.T("placeholder.title"))
}

func TestPlaceholderSwallowsPageInput(t *testing.T) {
	m := placeholderModel(layout.PageRules)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.False(t, next.(Model).rulesState.ShowAddForm(), "page shortcuts must not open hidden dialogs")
}

func TestRefreshingLogsAndSubscriptionsPlaceholderReconnects(t *testing.T) {
	for _, page := range []layout.PageType{layout.PageLogs, layout.PageSub} {
		m := placeholderModel(page)
		m.client = api.NewClient(config.MihomoEndpoint{ExternalController: "127.0.0.1:9090"}, 1000)
		require.NotNil(t, m.refreshMenu())
		require.True(t, m.connectionChecking, "page %d did not request a connection check", page)
	}
}
