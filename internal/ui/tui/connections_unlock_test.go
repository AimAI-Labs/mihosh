package tui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
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
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
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
		width: 106, height: 24, currentPage: layout.PageConnections,
		connsState: connections.NewState(proxy.URL, nil, nil, 1000, nil),
	}
	click := func(line string, x, y int) tea.Cmd {
		next, cmd := m.Update(tea.MouseMsg{X: lipgloss.Width(line[:x]), Y: y,
			Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
		m = next.(Model)
		return cmd
	}
	clickFooter := func(label string) tea.Cmd {
		t.Helper()
		y := m.height - common.StatusBarHeight - 1
		rows := strings.Split(ansi.Strip(m.View()), "\n")
		at := strings.Index(rows[y], label)
		require.NotEqual(t, -1, at, rows[y])
		return click(rows[y], at, y)
	}
	clickToolbar := func() tea.Cmd {
		t.Helper()
		label := "S " + i18n.T("conns.unlock.run_all")
		for y, row := range strings.Split(ansi.Strip(m.View()), "\n") {
			if at := strings.Index(row, label); at >= 0 {
				return click(row, at, y)
			}
		}
		t.Fatalf("unlock toolbar is missing %q", label)
		return nil
	}

	require.Nil(t, clickFooter("U "+i18n.T("conns.unlock.title")))
	require.Contains(t, ansi.Strip(m.View()), "Bilibili")
	require.True(t, m.connsState.InputCapture().Active)

	var single tea.Cmd
	for y, row := range strings.Split(ansi.Strip(m.View()), "\n") {
		if name := strings.Index(row, "Bilibili (HK/MO/TW)"); name >= 0 {
			require.Contains(t, row[:name], "Bilibili (CN)", "wide modal must place services side by side")
			action := strings.Index(row[name:], "↻")
			require.NotEqual(t, -1, action)
			single = click(row, name+action, y)
			break
		}
	}
	require.NotNil(t, single)
	result := single().(messages.UnlockTestMsg)
	require.Equal(t, model.UnlockFailed, result.Result.Status)
	next, save := m.Update(result)
	m = next.(Model)
	require.NotNil(t, save)
	require.Nil(t, save())
	require.EqualValues(t, 1, calls.Load())
	require.Contains(t, ansi.Strip(m.View()), i18n.T("conns.unlock.failed"))
	require.Contains(t, ansi.Strip(m.View()), result.Result.CheckedAt.Local().Format("2006-01-02 15:04:05"))

	batch := clickToolbar()
	require.NotNil(t, batch)
	probes := batch().(tea.BatchMsg)
	require.Len(t, probes, len(service.UnlockServiceNames()))
	require.Contains(t, ansi.Strip(m.View()), i18n.T("conns.unlock.checking"))
	for _, probe := range probes {
		before := calls.Load()
		next, save := m.Update(probe())
		m = next.(Model)
		if save != nil {
			require.Nil(t, save())
		}
		require.Greater(t, calls.Load(), before, "each service check must contact the proxy")
	}
	require.NotContains(t, ansi.Strip(m.View()), i18n.T("conns.unlock.checking"))

	stale := clickFooter("↵ " + i18n.T("conns.unlock.run_one"))
	require.NotNil(t, stale)
	require.Nil(t, clickFooter("Esc "+i18n.T("conns.unlock.close")))
	require.False(t, m.connsState.InputCapture().Active)
	require.Nil(t, clickFooter("U "+i18n.T("conns.unlock.title")))
	old := stale().(messages.UnlockTestMsg)
	old.Result.Status = model.UnlockAvailable
	next, _ = m.Update(old)
	m = next.(Model)
	require.NotContains(t, ansi.Strip(m.View()), i18n.T("conns.unlock.available"))

	m.connsState = connections.NewState(proxy.URL, nil, nil, 1000, nil)
	require.Nil(t, clickFooter("U "+i18n.T("conns.unlock.title")))
	require.Contains(t, ansi.Strip(m.View()), i18n.T("conns.unlock.failed"), "a fresh page should restore checked results")
}
