package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestRefreshGroupProjectionMatchesAPI(t *testing.T) {
	for _, proxies := range []map[string]model.Proxy{
		nil,
		{"leaf": {Name: "leaf"}, "empty": {Type: "Selector", All: []string{}}},
		{"Auto": {Type: "URLTest", Now: "leaf", All: []string{"leaf"}}, "Select": {Type: "Selector", All: []string{"missing"}}},
		{"GLOBAL": {All: []string{"Select", "leaf"}}, "Select": {Type: "Selector", All: []string{"leaf"}}, "Auto": {Type: "URLTest", All: []string{"missing"}}},
		{"GLOBAL": {All: []string{"Select", "Auto", "GLOBAL"}}, "Select": {Type: "Selector", All: []string{"Auto"}}, "Auto": {Type: "URLTest", Now: "Select", All: []string{"Select"}}},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(model.ProxiesResponse{Proxies: proxies})
		}))
		client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)
		apiGroups, apiOrder, err := client.GetGroups()
		server.Close()
		require.NoError(t, err)
		groups, order := model.ProxySnapshot(proxies).Groups()
		require.Equal(t, apiGroups, groups)
		require.ElementsMatch(t, apiOrder, order)
		prefix := 0
		for _, name := range proxies["GLOBAL"].All {
			if _, ok := groups[name]; ok {
				require.Equal(t, name, order[prefix])
				require.Equal(t, name, apiOrder[prefix])
				prefix++
			}
		}
	}
}

func TestDetectAutoRefreshChangeDetectsModeChange(t *testing.T) {
	state := nodes.State{Mode: "rule"}
	result := messages.AutoRefreshResult{Mode: "global"}

	if !detectAutoRefreshChange(state, result) {
		t.Fatalf("expected mode change to be detected")
	}
}

func TestDetectAutoRefreshChangeDetectsSelectedProxyChange(t *testing.T) {
	state := nodes.State{
		Groups: map[string]model.Group{
			"Proxy": {Name: "Proxy", Now: "HK-01", All: []string{"HK-01", "JP-01"}},
		},
	}
	result := messages.AutoRefreshResult{
		Groups: map[string]model.Group{
			"Proxy": {Name: "Proxy", Now: "JP-01", All: []string{"HK-01", "JP-01"}},
		},
	}

	if !detectAutoRefreshChange(state, result) {
		t.Fatalf("expected selected proxy change to be detected")
	}
}

func TestDetectAutoRefreshChangeIgnoresUnchangedData(t *testing.T) {
	state := nodes.State{
		Mode: "rule",
		Groups: map[string]model.Group{
			"Proxy": {Name: "Proxy", Now: "HK-01", All: []string{"HK-01", "JP-01"}},
		},
		GroupNames: []string{"Proxy"},
	}
	result := messages.AutoRefreshResult{
		Mode: "rule",
		Groups: map[string]model.Group{
			"Proxy": {Name: "Proxy", Now: "HK-01", All: []string{"HK-01", "JP-01"}},
		},
		OrderedNames: []string{"Proxy"},
	}

	if detectAutoRefreshChange(state, result) {
		t.Fatalf("expected unchanged data to be ignored")
	}
}

func TestAutoRefreshMessageShowsSyncedCheckmarkAfterSuccessfulRefresh(t *testing.T) {
	model := Model{
		config: &config.Config{AutoRefreshInterval: 5},
	}

	next, _ := model.Update(messages.AutoRefreshMsg{Changed: false})
	got := next.(Model)

	if !got.autoRefreshSynced {
		t.Fatalf("expected successful auto refresh to show synced checkmark")
	}

	next, _ = got.Update(messages.AutoRefreshTickMsg(time.Now()))
	got = next.(Model)
	if got.autoRefreshSynced {
		t.Fatalf("expected synced checkmark to clear after one tick")
	}
}

func TestAutoRefreshNoticeUsesIndependentExpiry(t *testing.T) {
	model := Model{config: &config.Config{AutoRefreshInterval: 5}}
	next, cmd := model.Update(messages.AutoRefreshMsg{Changed: true})
	got := next.(Model)
	if got.notice.Message.Level != messages.NoticeInfo || got.notice.Message.Text == "" || cmd == nil {
		t.Fatal("changed refresh must publish an expiring info notification")
	}

	next, _ = got.Update(messages.AutoRefreshTickMsg(time.Now()))
	got = next.(Model)
	if got.notice.Message.Text == "" {
		t.Fatal("the refresh timer expired a notification")
	}
	next, _ = got.Update(messages.NoticeExpiredMsg{ID: got.notice.ID})
	if next.(Model).notice.Message.Text != "" {
		t.Fatal("the notification timer did not clear the message")
	}
}

func TestStartupConnectTriggersAutoRefreshAndLoadsInitialData(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/proxies":
			_, _ = w.Write([]byte(`{"proxies":{"Proxy":{"type":"Selector","all":["HK-01"],"now":"HK-01"},"HK-01":{"type":"Shadowsocks","name":"HK-01"}}}`))
		case "/configs":
			_, _ = w.Write([]byte(`{"mode":"rule"}`))
		case "/connections":
			_, _ = w.Write([]byte(`{"downloadTotal":2048,"uploadTotal":512,"connections":[]}`))
		case "/rules":
			_, _ = w.Write([]byte(`{"rules":[{"type":"DOMAIN","payload":"example.com","proxy":"Proxy"}]}`))
		case "/version":
			_, _ = w.Write([]byte(`{"version":"v1.20.0"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)
	m := newTestModel(client, "", 1000)
	t.Cleanup(m.Close)

	require.False(t, m.connectionChecked)

	updated, cmd := m.Update(messages.CoreConnectionMsg{Epoch: m.connectionEpoch, Version: "v1.20.0"})
	m = updated.(Model)
	require.True(t, m.connectionChecked)
	require.True(t, m.connectionReady)
	require.Equal(t, 1, m.autoRefreshRemaining)
	require.NotNil(t, cmd)

	var applyCmd func(tea.Cmd)
	applyCmd = func(c tea.Cmd) {
		if c == nil {
			return
		}
		res := c()
		if batch, ok := res.(tea.BatchMsg); ok {
			for _, subCmd := range batch {
				applyCmd(subCmd)
			}
			return
		}
		if res != nil {
			u, nextCmd := m.Update(res)
			m = u.(Model)
			applyCmd(nextCmd)
		}
	}
	applyCmd(cmd)

	require.True(t, m.autoRefreshSynced)
	require.Contains(t, m.nodesState.Groups, "Proxy")
	require.Equal(t, "rule", m.nodesState.Mode)
	require.NotNil(t, m.connsState.Connections)
	require.Equal(t, int64(2048), m.connsState.Connections.DownloadTotal)
	require.Len(t, m.rulesState.ToPageState(120, 40).Rules, 1)

	// Next tick immediately runs a follow-up auto-refresh when AutoRefreshInterval > 0.
	updated, tickCmd := m.Update(messages.AutoRefreshTickMsg(time.Now()))
	m = updated.(Model)
	require.Equal(t, m.autoRefreshInterval(), m.autoRefreshRemaining)
	require.NotNil(t, tickCmd)
}
