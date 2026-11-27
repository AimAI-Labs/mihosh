package tui

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/nodes"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/rules"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestLateHTTPCompletionsCannotAffectCurrentTarget(t *testing.T) {
	for _, payload := range []tea.Msg{
		messages.GroupsMsg{Groups: map[string]model.Group{"late": {Name: "late"}}, OrderedNames: []string{"late"}},
		messages.ProxiesMsg{"late": {Name: "late"}},
		messages.ConfigModeMsg{Mode: "global"},
		messages.ConnectionsMsg{Resp: &model.ConnectionsResponse{}},
		messages.AutoRefreshMsg{Result: messages.AutoRefreshResult{Mode: "global"}},
		messages.MihomoConfigMsg{Config: &model.MihomoConfig{ExternalController: "old-controller"}},
		messages.ErrMsg{Err: errors.New("old controller failed")},
	} {
		for _, action := range []string{"switch", "restart", "switch-back"} {
			t.Run(action+"/"+httpPayloadName(payload), func(t *testing.T) {
				m, _ := targetTestModel(t)
				cmd := messages.CoreRequest(m.client, func(*api.Client) tea.Msg { return payload })
				old := m.client.EndpointSnapshot()
				endpoint := old
				if action != "restart" {
					endpoint.ExternalController = "new-controller"
				}
				m.updateCoreTarget(config.MihomoTarget{Endpoint: endpoint}, coreTargetReconnect)
				if action == "switch-back" {
					m.updateCoreTarget(config.MihomoTarget{Endpoint: old}, coreTargetReconnect)
				}
				generation := m.client.Generation()
				updated, follow := m.Update(cmd())
				next := updated.(Model)
				require.Nil(t, follow)
				require.Empty(t, next.notice.Message.Text)
				require.Equal(t, generation, next.client.Generation())
				require.NotContains(t, next.nodesState.Groups, "late")
				require.NotContains(t, next.nodesState.Proxies, "late")
				require.Empty(t, next.nodesState.Mode)
				require.False(t, next.autoRefreshSynced)
			})
		}
	}
}

func httpPayloadName(msg tea.Msg) string {
	switch msg.(type) {
	case messages.GroupsMsg:
		return "groups"
	case messages.ProxiesMsg:
		return "proxies"
	case messages.ConfigModeMsg:
		return "mode"
	case messages.ConnectionsMsg:
		return "connections"
	case messages.AutoRefreshMsg:
		return "auto-refresh"
	case messages.MihomoConfigMsg:
		return "config"
	default:
		return "error"
	}
}

func TestCurrentHTTPCompletionStillApplies(t *testing.T) {
	m, _ := targetTestModel(t)
	cmd := messages.CoreRequest(m.client, func(*api.Client) tea.Msg {
		return messages.GroupsMsg{Groups: map[string]model.Group{"current": {Name: "current"}}, OrderedNames: []string{"current"}}
	})
	updated, _ := m.Update(cmd())
	require.Contains(t, updated.(Model).nodesState.Groups, "current")
}

func TestControllerReadCommandsBindTargetBeforeExecution(t *testing.T) {
	var requestsA, requestsB atomic.Int32
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsA.Add(1)
		switch r.URL.Path {
		case "/proxies":
			_, _ = w.Write([]byte(`{"proxies":{"group":{"type":"Selector","all":["DIRECT"],"now":"DIRECT"}}}`))
		case "/configs":
			_, _ = w.Write([]byte(`{"mode":"rule"}`))
		case "/connections":
			_, _ = w.Write([]byte(`{"connections":[]}`))
		case "/rules":
			_, _ = w.Write([]byte(`{"rules":[]}`))
		case "/memory":
			_, _ = w.Write([]byte(`{"inuse":42}`))
		case "/version":
			_, _ = w.Write([]byte(`{"version":"controller-a"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer a.Close()
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requestsB.Add(1); http.Error(w, "wrong target", 500) }))
	defer b.Close()
	for _, makeCommand := range []struct {
		name string
		make func(*api.Client) tea.Cmd
	}{
		{"groups", nodes.FetchGroups}, {"proxies", nodes.FetchProxies}, {"mode", nodes.FetchConfigMode},
		{"connections", connections.FetchConnections}, {"rules", rules.FetchRules},
		{"version", preferences.FetchMihomoVersion}, {"memory", fetchMemory},
		{"auto-refresh", func(c *api.Client) tea.Cmd { return fetchAutoRefresh(c, nodes.State{}) }},
	} {
		t.Run(makeCommand.name, func(t *testing.T) {
			m, _ := targetTestModel(t)
			m.client = api.NewClient(config.MihomoEndpoint{ExternalController: a.URL}, 1000)
			before := requestsA.Load()
			cmd := makeCommand.make(m.client)
			m.client.UpdateEndpoint(b.URL, "new-secret")
			result, ok := cmd().(messages.CoreHTTPMsg)
			require.True(t, ok)
			require.Less(t, result.Generation, m.client.Generation())
			require.Greater(t, requestsA.Load(), before)
			require.Zero(t, requestsB.Load())
			_, failed := result.Payload.(messages.ErrMsg)
			require.False(t, failed)
		})
	}
}

func TestAutoRefreshKeepsOneTargetAcrossRequests(t *testing.T) {
	var client *api.Client
	var requestsB atomic.Int32
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requestsB.Add(1); http.Error(w, "wrong target", 500) }))
	defer b.Close()
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/proxies" {
			client.UpdateEndpoint(b.URL, "new-secret")
			_, _ = w.Write([]byte(`{"proxies":{}}`))
		} else {
			_, _ = w.Write([]byte(`{"mode":"rule"}`))
		}
	}))
	defer a.Close()
	client = api.NewClient(config.MihomoEndpoint{ExternalController: a.URL}, 1000)
	cmd := fetchAutoRefresh(client, nodes.State{})
	result := cmd().(messages.CoreHTTPMsg)
	refresh, ok := result.Payload.(messages.AutoRefreshMsg)
	require.True(t, ok)
	require.Equal(t, "rule", refresh.Result.Mode)
	require.Zero(t, requestsB.Load())
	require.Less(t, result.Generation, client.Generation())
}

func TestBatchResultsKeepTargetOwnershipAcrossReconnects(t *testing.T) {
	var wrongRequests atomic.Int32
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"delay":42}`))
	}))
	defer a.Close()
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrongRequests.Add(1)
		http.Error(w, "wrong target", http.StatusInternalServerError)
	}))
	defer b.Close()
	for _, action := range []string{"sync", "restart", "switch", "switch-back"} {
		t.Run(action, func(t *testing.T) {
			m, _ := targetTestModel(t)
			m.client = api.NewClient(config.MihomoEndpoint{ExternalController: a.URL}, 1000)
			m.nodesState = nodes.NewState(m.client, "http://test.invalid", 1000)
			var nodeCmd, siteCmd tea.Cmd
			m.nodesState, nodeCmd = m.nodesState.StartSingleTest("node")
			m.connsState = connections.NewState("", []model.SiteTest{{Name: "site", URL: a.URL}}, m.client, 1000, nil)
			m.connsState, siteCmd = m.connsState.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
			endpoint := m.client.EndpointSnapshot()
			update := coreTargetSync
			if action != "sync" {
				update = coreTargetReconnect
			}
			if action == "switch" || action == "switch-back" {
				endpoint.ExternalController = b.URL
			}
			m.updateCoreTarget(config.MihomoTarget{Endpoint: endpoint}, update)
			if action == "switch-back" {
				endpoint.ExternalController = a.URL
				m.updateCoreTarget(config.MihomoTarget{Endpoint: endpoint}, coreTargetReconnect)
			}
			for _, command := range []tea.Cmd{nodeCmd, siteCmd} {
				result := command()
				_, bound := result.(messages.CoreHTTPMsg)
				require.True(t, bound, "batch command must bind to a target before execution")
				updated, follow := m.Update(result)
				m = updated.(Model)
				if action != "sync" {
					require.Nil(t, follow, "stale completion must not schedule a refill or refresh")
				}
			}
			if action == "sync" {
				require.Len(t, m.nodesState.TestResults(), 1)
			} else {
				require.Empty(t, m.nodesState.TestResults())
			}
			require.False(t, m.nodesState.Testing, "reconnect must not leave an abandoned batch active")
			for _, site := range m.connsState.ToPageState(nil, 100, 30).SiteTests {
				require.False(t, site.Testing)
			}
			require.Zero(t, wrongRequests.Load())
		})
	}
}

func TestSelectProxyRefreshesNodesForCurrentTarget(t *testing.T) {
	var selected atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/proxies/group":
			selected.Store(true)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/proxies" && selected.Load():
			_, _ = w.Write([]byte(`{"proxies":{"group":{"name":"group","type":"Selector","all":["node"],"now":"node"},"node":{"name":"node","type":"Direct"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	m, _ := targetTestModel(t)
	m.client = api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)
	completion := nodes.SelectProxy(m.client, "group", "node")()
	require.True(t, selected.Load())
	_, refresh := m.Update(completion)
	require.NotNil(t, refresh)
	batch, ok := refresh().(tea.BatchMsg)
	require.True(t, ok)
	require.Len(t, batch, 2)

	for _, cmd := range batch {
		updated, _ := m.Update(cmd())
		m = updated.(Model)
	}
	require.Equal(t, "node", m.nodesState.Groups["group"].Now)
	require.Contains(t, m.nodesState.Proxies, "node")

	m.client.UpdateEndpoint(server.URL, "changed-generation")
	_, staleRefresh := m.Update(completion)
	require.Nil(t, staleRefresh)
}
