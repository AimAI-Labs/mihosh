package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func runBatchAndCollectCurrentIP(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			out = append(out, runBatchAndCollectCurrentIP(t, sub)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func TestConnectionsTrafficTabCurrentIPFlow(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	chart := model.NewChartData(60)
	chart.AddSpeedData(1024, 2048)

	var fetches atomic.Int32
	ipList := []*model.IPInfo{
		{
			IP:           "203.0.113.42",
			Country:      "Japan",
			CountryCode:  "JP",
			City:         "Tokyo",
			RegionName:   "Tokyo",
			ASN:          2514,
			AS:           "AS2514",
			Organization: "NTT PC Communications, Inc.",
			Timezone:     "Asia/Tokyo",
			Latitude:     35.68,
			Longitude:    139.68,
		},
		{
			IP:           "198.51.100.7",
			Country:      "Singapore",
			CountryCode:  "SG",
			City:         "Singapore",
			ASN:          13335,
			AS:           "AS13335",
			Organization: "Cloudflare, Inc.",
			Timezone:     "Asia/Singapore",
			Latitude:     1.29,
			Longitude:    103.85,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"connections":[]}`))
	}))
	defer srv.Close()
	client := api.NewClient(config.MihomoEndpoint{ExternalController: srv.URL}, 1000)

	m := Model{
		client:      client,
		width:       120,
		height:      34,
		currentPage: layout.PageNodes,
		chartData:   chart,
		connsState: connections.NewState("http://127.0.0.1:7890", model.DefaultSiteTests(), client, 1000, chart).
			SetCurrentIPFetcher(func(_ context.Context, _ string) (*model.IPInfo, error) {
				idx := int(fetches.Add(1)) - 1
				if idx >= len(ipList) {
					idx = len(ipList) - 1
				}
				return ipList[idx], nil
			}),
	}

	// Navigating from Nodes -> Connections triggers the initial current IP fetch.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	require.Equal(t, layout.PageConnections, m.currentPage)
	require.NotNil(t, cmd)

	var gotIP messages.CurrentIPInfoMsg
	foundIP := false
	for _, msg := range runBatchAndCollectCurrentIP(t, cmd) {
		if ipMsg, ok := msg.(messages.CurrentIPInfoMsg); ok {
			gotIP = ipMsg
			foundIP = true
		}
	}
	require.True(t, foundIP, "entering connections page must schedule a current IP fetch")

	next, _ = m.Update(gotIP)
	m = next.(Model)

	view := ansi.Strip(m.View())
	require.Contains(t, view, "[JP] Japan")
	require.Contains(t, view, "203.0.113.42")
	require.Contains(t, view, "AS2514")
	require.Contains(t, view, "NTT PC Communications, Inc.")

	// Verify the IP info row sits strictly between the Traffic tab bar and the Speed chart.
	lines := strings.Split(view, "\n")
	trafficRow, ipRow, chartRow := -1, -1, -1
	for i, line := range lines {
		if trafficRow == -1 && strings.Contains(line, i18n.T("conns.tab_traffic")) {
			trafficRow = i
		}
		if ipRow == -1 && strings.Contains(line, "203.0.113.42") {
			ipRow = i
		}
		if chartRow == -1 && strings.Contains(line, i18n.T("conns.chart_title")) {
			chartRow = i
		}
	}
	require.Greater(t, ipRow, trafficRow)
	require.Less(t, ipRow, chartRow)
	require.Equal(t, chartRow-1, ipRow, "IP info should occupy the single row directly above the speed chart")

	// ConnTickMsg decrements the countdown in the rendered bar.
	next, _ = m.Update(messages.ConnTickMsg(time.Now()))
	m = next.(Model)
	require.Contains(t, ansi.Strip(m.View()), "↻ 299s")

	// Pressing 'r' refreshes current IP info.
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = next.(Model)
	foundIP = false
	for _, msg := range runBatchAndCollectCurrentIP(t, cmd) {
		if ipMsg, ok := msg.(messages.CurrentIPInfoMsg); ok {
			next, _ = m.Update(ipMsg)
			m = next.(Model)
			foundIP = true
		}
	}
	require.True(t, foundIP, "manual refresh on Traffic tab must refresh current IP")
	require.Contains(t, ansi.Strip(m.View()), "198.51.100.7")
	require.Contains(t, ansi.Strip(m.View()), "[SG] Singapore")
}
