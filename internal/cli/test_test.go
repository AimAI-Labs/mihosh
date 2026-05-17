package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

func TestResolveTestAction(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantAction testAction
		wantTarget string
		wantErr    string
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
			wantErr: "参数格式错误",
		},
		{
			name:    "unknown action is invalid",
			args:    []string{"foo", "bar"},
			wantErr: "参数格式错误",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			action, target, err := resolveTestAction(tc.args)
			if tc.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotNode, gotFound := resolveCurrentSelectedNode(tc.proxies)
			assert.Equal(t, tc.wantFound, gotFound)
			assert.Equal(t, tc.wantNode, gotNode)
		})
	}
}
