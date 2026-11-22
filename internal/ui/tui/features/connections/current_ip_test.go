package connections

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestTrafficTabRendersCurrentIPBetweenModeSwitchAndChart(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	chart := model.NewChartData(60)
	for i := 0; i < 5; i++ {
		chart.AddSpeedData(int64((i+1)*1024), int64((i+1)*2048))
	}

	s := NewState("http://127.0.0.1:7890", model.DefaultSiteTests(), nil, 1000, chart)
	s = s.SetCurrentIPFetcher(func(_ context.Context, _ string) (*model.IPInfo, error) {
		return &model.IPInfo{
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
		}, nil
	})

	s, cmd := s.EnsureCurrentIP()
	require.NotNil(t, cmd)
	s, _ = s.Update(cmd())

	rendered := RenderConnectionsPage(s.ToPageState(chart, 120, 36))
	lines := strings.Split(ansi.Strip(rendered), "\n")
	require.Greater(t, len(lines), 5)

	// Lines 0..2: mode switch bar (流量监控)
	require.Contains(t, lines[1], i18n.T("conns.tab_traffic"))
	// Line 3: single-line current IP info bar
	require.Contains(t, lines[3], "[JP] Japan")
	require.Contains(t, lines[3], "203.0.113.42")
	require.Contains(t, lines[3], "AS2514")
	require.Contains(t, lines[3], "↻ 300s")
	// Line 4: upload/download speed chart top border
	require.Contains(t, lines[4], i18n.T("conns.chart_title"))

	// Mouse hit on IP toggles masking; mouse hit on refresh triggers a fresh fetch;
	// chart hit still resolves cleanly right below line 3.
	pageState := s.ToPageState(chart, 120, 36)
	ipX, ipY, ok := findConnMousePoint(s, 120, 36, MouseTargetIPMaskToggle, -1)
	require.True(t, ok)
	require.Equal(t, connectionsModeSwitchHeight, ipY)

	s, cmd = s.HandleMouseLeft(ipX, ipY, 120, 36)
	require.Nil(t, cmd)
	require.True(t, s.currentIP.masked)
	maskedLines := strings.Split(ansi.Strip(RenderConnectionsPage(s.ToPageState(chart, 120, 36))), "\n")
	require.NotContains(t, maskedLines[3], "203.0.113.42")
	require.Contains(t, maskedLines[3], "••••••••")

	// Pressing 'i' unmasks IP back to visible.
	s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	require.Nil(t, cmd)
	require.False(t, s.currentIP.masked)

	// Clicking refresh button triggers new lookup.
	refX, refY, ok := findConnMousePoint(s, 120, 36, MouseTargetIPRefresh, -1)
	require.True(t, ok)
	require.Equal(t, connectionsModeSwitchHeight, refY)
	s, cmd = s.HandleMouseLeft(refX, refY, 120, 36)
	require.NotNil(t, cmd)
	require.True(t, s.currentIP.loading)

	// Chart hit still lands on line 4+.
	chartHit := ResolveMouseHit(pageState, 10, 4)
	require.Equal(t, MouseTargetChart, chartHit.Target)
}

func TestCurrentIPCountdownAutoRefreshAndStaleDrop(t *testing.T) {
	calls := 0
	s := NewState("http://127.0.0.1:7890", nil, nil, 1000, nil).
		SetCurrentIPFetcher(func(_ context.Context, proxyAddr string) (*model.IPInfo, error) {
			calls++
			if proxyAddr != "http://127.0.0.1:7890" {
				return nil, errors.New("unexpected proxy")
			}
			return &model.IPInfo{IP: "198.51.100.1", CountryCode: "US"}, nil
		})

	s, firstCmd := s.RefreshCurrentIP()
	require.NotNil(t, firstCmd)
	firstMsg := firstCmd().(messages.CurrentIPInfoMsg)

	// Trigger a second refresh before the first finishes -> first response must be ignored.
	s, secondCmd := s.RefreshCurrentIP()
	require.NotNil(t, secondCmd)
	s, _ = s.Update(firstMsg)
	require.True(t, s.currentIP.loading)
	require.Nil(t, s.currentIP.info)

	s, _ = s.Update(secondCmd())
	require.False(t, s.currentIP.loading)
	require.NotNil(t, s.currentIP.info)
	require.Equal(t, currentIPRefreshInterval, s.currentIP.countdown)

	// Tick decrements countdown on Traffic tab, but pauses on Active tab.
	s, cmd := s.Update(messages.ConnTickMsg(time.Now()))
	require.Nil(t, cmd)
	require.Equal(t, currentIPRefreshInterval-1, s.currentIP.countdown)

	s.setConnViewMode(ConnViewActive)
	s, cmd = s.Update(messages.ConnTickMsg(time.Now()))
	require.Nil(t, cmd)
	require.Equal(t, currentIPRefreshInterval-1, s.currentIP.countdown)

	// Reaching 0 triggers auto-refresh.
	s.setConnViewMode(ConnViewTraffic)
	s.currentIP.countdown = 1
	s, cmd = s.Update(messages.ConnTickMsg(time.Now()))
	require.NotNil(t, cmd)
	require.True(t, s.currentIP.loading)
	s, _ = s.Update(cmd())
	require.Equal(t, 3, calls)

	// CoreTargetChangedMsg invalidates cached IP.
	s, _ = s.Update(messages.CoreTargetChangedMsg{Invalidate: true, ProxyURL: "http://127.0.0.1:7891"})
	require.Nil(t, s.currentIP.info)
	require.True(t, s.currentIP.dirty)
}

func TestCurrentIPErrorSetsRetryCountdown(t *testing.T) {
	s := NewState("http://127.0.0.1:7890", nil, nil, 1000, nil).
		SetCurrentIPFetcher(func(_ context.Context, _ string) (*model.IPInfo, error) {
			return nil, errors.New("dial tcp timeout")
		})

	s, cmd := s.RefreshCurrentIP()
	require.NotNil(t, cmd)
	s, _ = s.Update(cmd())
	require.False(t, s.currentIP.loading)
	require.Nil(t, s.currentIP.info)
	require.Contains(t, s.currentIP.err, "dial tcp timeout")
	require.Equal(t, currentIPRetryInterval, s.currentIP.countdown)
}
