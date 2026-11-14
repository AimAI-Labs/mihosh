package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentNodeAndChainUseOneSnapshot(t *testing.T) {
	var snapshots, individual atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.IsAbs():
			// Act as the local outbound-IP proxy too; no external request is sent.
			io.WriteString(w, `{"query":"203.0.113.1","status":"success"}`)
		case r.URL.Path == "/proxies":
			snapshots.Add(1)
			io.WriteString(w, `{"proxies":{"GLOBAL":{"now":"Proxy","all":["Proxy"]},"Proxy":{"now":"first","all":["first"]},"first":{}}}`)
		case r.URL.Path == "/proxies/first/delay":
			io.WriteString(w, `{"delay":42}`)
		case strings.HasPrefix(r.URL.Path, "/proxies/"):
			individual.Add(1)
			if strings.HasSuffix(r.URL.Path, "/GLOBAL") {
				io.WriteString(w, `{"now":"changed-after-test"}`)
			} else {
				io.WriteString(w, `{}`)
			}
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)
	svc := service.NewProxyService(client, "http://test.invalid", 1000)
	var out bytes.Buffer
	require.NoError(t, runTestAction(&out, svc, server.URL, actionCurrent, "", outputFormatJSON))
	require.EqualValues(t, 1, snapshots.Load())
	require.Zero(t, individual.Load(), "chain must come from the same response as the selected node")
	var result struct {
		Node  string   `json:"node"`
		Chain []string `json:"chain"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	require.Equal(t, "first", result.Node)
	require.Equal(t, []string{"GLOBAL", "Proxy", "first"}, result.Chain)
}

func TestResolveTestAction(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantAction testAction
		wantTarget string
		wantErr    bool
	}{
		{
			name:       "no args tests current selected node",
			args:       []string{},
			wantAction: actionCurrent,
		},
		{
			name:       "node action with target",
			args:       []string{"node", "HK"},
			wantAction: actionNode,
			wantTarget: "HK",
		},
		{
			name:       "group action with target",
			args:       []string{"group", "Auto"},
			wantAction: actionGroup,
			wantTarget: "Auto",
		},
		{
			name:    "single arg is invalid",
			args:    []string{"HK"},
			wantErr: true,
		},
		{
			name:    "unknown action is invalid",
			args:    []string{"foo", "bar"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			action, target, err := resolveTestAction(tc.args)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.wantAction, action)
			assert.Equal(t, tc.wantTarget, target)
		})
	}
}

func TestRenderNodeTestOutputJSON(t *testing.T) {
	var out bytes.Buffer
	err := renderNodeTestOutput(&out, "HK", 15, outputFormatJSON)
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, `"action": "node"`)
	assert.Contains(t, output, `"node": "HK"`)
	assert.Contains(t, output, `"delay_ms": 15`)
}

func TestRenderCurrentTestOutputTable(t *testing.T) {
	ipInfo := &model.IPInfo{
		IP:          "1.1.1.1",
		Country:     "United States",
		CountryCode: "US",
		City:        "Los Angeles",
		AS:          "AS13335",
		Org:         "Cloudflare",
	}

	var out bytes.Buffer
	err := renderCurrentTestOutput(&out, "HK", []string{"GLOBAL", "Proxy", "HK"}, ipInfo, outputFormatTable)
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "KEY")
	assert.Contains(t, output, "VALUE")
	assert.Contains(t, output, "NODE")
	assert.Contains(t, output, "HK")
}

func TestResolveCurrentSelectedNode(t *testing.T) {
	tests := []struct {
		name      string
		proxies   map[string]model.Proxy
		wantNode  string
		wantFound bool
	}{
		{
			name: "resolve from global chain",
			proxies: map[string]model.Proxy{
				"GLOBAL": {Now: "Proxy", All: []string{"Proxy"}},
				"Proxy":  {Now: "HK", All: []string{"HK", "US"}},
				"HK":     {},
			},
			wantNode:  "HK",
			wantFound: true,
		},
		{
			name: "resolve from proxy root without global",
			proxies: map[string]model.Proxy{
				"Proxy": {Now: "JP", All: []string{"JP"}},
				"JP":    {},
			},
			wantNode:  "JP",
			wantFound: true,
		},
		{
			name: "no selected leaf returns not found",
			proxies: map[string]model.Proxy{
				"GLOBAL": {Now: "Proxy", All: []string{"Proxy"}},
				"Proxy":  {All: []string{"HK", "US"}},
			},
			wantFound: false,
		},
		{
			name: "cycle returns not found",
			proxies: map[string]model.Proxy{
				"GLOBAL": {Now: "Proxy", All: []string{"Proxy"}},
				"Proxy":  {Now: "GLOBAL", All: []string{"GLOBAL"}},
			},
			wantFound: false,
		},
		{
			name: "invalid GLOBAL still falls back to Proxy",
			proxies: map[string]model.Proxy{
				"GLOBAL": {Now: "missing"},
				"Proxy":  {Now: "JP", All: []string{"JP"}},
				"JP":     {},
			},
			wantNode: "JP", wantFound: true,
		},
		{
			name: "GLOBAL leaf takes precedence over Proxy",
			proxies: map[string]model.Proxy{
				"GLOBAL": {Now: "HK"}, "HK": {},
				"Proxy": {Now: "JP"}, "JP": {},
			},
			wantNode: "HK", wantFound: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotNode, chain, gotFound := resolveCurrentSelectedNode(tc.proxies)
			assert.Equal(t, tc.wantFound, gotFound)
			assert.Equal(t, tc.wantNode, gotNode)
			if gotFound {
				assert.Equal(t, gotNode, chain[len(chain)-1])
			}
		})
	}
}
