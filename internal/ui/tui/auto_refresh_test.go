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
			json.NewEncoder(w).Encode(model.ProxiesResponse{Proxies: proxies})
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

func TestAutoRefreshNoticeExpiresOnTicks(t *testing.T) {
	model := Model{
		config: &config.Config{AutoRefreshInterval: 5},
	}

	next, _ := model.Update(messages.AutoRefreshMsg{Changed: true})
	got := next.(Model)
	if got.notice == "" {
		t.Fatalf("expected changed refresh to set notice")
	}

	for i := 0; i < autoRefreshNoticeTicks; i++ {
		next, _ = got.Update(messages.AutoRefreshTickMsg(time.Now()))
		got = next.(Model)
	}

	if got.notice != "" {
		t.Fatalf("expected auto refresh notice to expire, got %q", got.notice)
	}
}
