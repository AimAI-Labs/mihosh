package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCommandUsesOneProxySnapshot(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxies" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if requests.Add(1) == 1 {
			_, _ = io.WriteString(w, `{"proxies":{"GLOBAL":{"all":["Select","GLOBAL"]},"Select":{"type":"Selector","now":"first","all":["first"]},"first":{"history":[{"delay":11}]}}}`)
		} else {
			_, _ = io.WriteString(w, `{"proxies":{"GLOBAL":{"all":["Select","GLOBAL"]},"Select":{"type":"Selector","now":"second","all":["second"]},"second":{"history":[{"delay":77}]}}}`)
		}
	}))
	defer server.Close()
	cfg := config.DefaultConfig
	cfg.Core.Mode, cfg.Core.ExternalController = "external", server.URL
	require.NoError(t, config.Save(&cfg))
	out, err := os.CreateTemp(t.TempDir(), "list-output")
	require.NoError(t, err)
	savedOutput, savedStdout := listOutput, os.Stdout
	listOutput, os.Stdout = "json", out
	t.Cleanup(func() { listOutput, os.Stdout = savedOutput, savedStdout; _ = out.Close() })
	require.NoError(t, listCmd.RunE(listCmd, nil))
	require.EqualValues(t, 1, requests.Load(), "one display must use one proxy snapshot")
	_, err = out.Seek(0, io.SeekStart)
	require.NoError(t, err)
	data, err := io.ReadAll(out)
	require.NoError(t, err)
	require.Contains(t, string(data), `"current": "first"`)
	require.Contains(t, string(data), `"delay_ms": 11`)
	require.NotContains(t, string(data), "second")
}

func TestRenderGroupList(t *testing.T) {
	groups := map[string]model.Group{
		"Auto": {
			Name: "Auto",
			Type: "Selector",
			Now:  "HK",
			All:  []string{"HK", "US"},
		},
		"Global": {
			Name: "Global",
			Type: "Selector",
			Now:  "Direct",
			All:  []string{"Direct", "Auto"},
		},
	}
	orderedNames := []string{"Global", "Auto"}
	proxies := map[string]model.Proxy{
		"HK":     {History: []model.Delay{{Delay: 10}}},
		"US":     {History: []model.Delay{{Delay: 20}}},
		"Direct": {History: []model.Delay{{Delay: 5}}},
		"Auto":   {History: []model.Delay{{Delay: 30}}},
	}

	tests := []struct {
		name     string
		format   outputFormat
		contains []string
	}{
		{
			name:   "JSON format",
			format: outputFormatJSON,
			contains: []string{
				`"groups"`,
				`"name": "Auto"`,
				`"delay_ms": 10`,
			},
		},
		{
			name:   "Table format",
			format: outputFormatTable,
			contains: []string{
				"GROUP",
				"PROXY",
				"Auto",
				"HK",
			},
		},
	}

	t.Run("RespectsConfiguredOrder (Plain)", func(t *testing.T) {
		var out bytes.Buffer
		err := renderGroupList(&out, groups, orderedNames, proxies, outputFormatPlain)
		assert.NoError(t, err)

		output := out.String()
		globalIdx := strings.Index(output, "[Selector] Global")
		autoIdx := strings.Index(output, "[Selector] Auto")
		assert.NotEqual(t, -1, globalIdx, "missing Global group output")
		assert.NotEqual(t, -1, autoIdx, "missing Auto group output")
		assert.Less(t, globalIdx, autoIdx, "expected Global before Auto")
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := renderGroupList(&out, groups, orderedNames, proxies, tt.format)
			assert.NoError(t, err)

			output := out.String()
			for _, c := range tt.contains {
				assert.Contains(t, output, c)
			}
		})
	}
}
