package tui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestConnectionFooterUnlockTestsUseProxyAndHandleResults(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	var calls atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			calls.Add(1)
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()

	m := Model{
		width: 54, height: 24, currentPage: layout.PageConnections,
		connsState: connections.NewState(proxy.URL, nil, nil, 1000, nil),
	}
	clickFooter := func(label string) tea.Cmd {
		t.Helper()
		y := m.height - common.StatusBarHeight - 1
		rows := strings.Split(ansi.Strip(m.View()), "\n")
		at := strings.Index(rows[y], label)
		require.NotEqual(t, -1, at, rows[y])
		next, cmd := m.Update(tea.MouseMsg{X: lipgloss.Width(rows[y][:at]), Y: y,
			Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
		m = next.(Model)
		return cmd
	}

	require.Nil(t, clickFooter("U "+i18n.T("conns.unlock.title")))
	require.Contains(t, ansi.Strip(m.View()), "Bilibili")
	require.True(t, m.connsState.InputCapture().Active)

	single := clickFooter("↵ " + i18n.T("conns.unlock.run_one"))
	require.NotNil(t, single)
	result := single().(messages.UnlockTestMsg)
	require.Equal(t, service.UnlockFailed, result.Result.Status)
	next, _ := m.Update(result)
	m = next.(Model)
	require.EqualValues(t, 1, calls.Load())
	require.Contains(t, ansi.Strip(m.View()), i18n.T("conns.unlock.failed"))

	batch := clickFooter("S " + i18n.T("conns.unlock.run_all"))
	require.NotNil(t, batch)
	probes := batch().(tea.BatchMsg)
	require.Len(t, probes, len(service.UnlockServiceNames()))
	require.Contains(t, ansi.Strip(m.View()), i18n.T("conns.unlock.checking"))
	for _, probe := range probes {
		before := calls.Load()
		next, _ := m.Update(probe())
		m = next.(Model)
		require.Greater(t, calls.Load(), before, "each service check must contact the proxy")
	}
	require.NotContains(t, ansi.Strip(m.View()), i18n.T("conns.unlock.checking"))

	stale := clickFooter("↵ " + i18n.T("conns.unlock.run_one"))
	require.NotNil(t, stale)
	require.Nil(t, clickFooter("Esc "+i18n.T("conns.unlock.close")))
	require.False(t, m.connsState.InputCapture().Active)
	require.Nil(t, clickFooter("U "+i18n.T("conns.unlock.title")))
	old := stale().(messages.UnlockTestMsg)
	old.Result.Status = service.UnlockAvailable
	next, _ = m.Update(old)
	m = next.(Model)
	require.NotContains(t, ansi.Strip(m.View()), i18n.T("conns.unlock.available"))
}
