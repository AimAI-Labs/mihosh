package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildStatusSnapshotAggregatesConnectionSpeeds(t *testing.T) {
	conns := model.ConnectionsResponse{
		Connections: []model.Connection{
			{UploadSpeed: 100, DownloadSpeed: 200},
			{UploadSpeed: 300, DownloadSpeed: 400},
		},
	}
	mem := model.MemoryResponse{Inuse: 1024, OSLimit: 2048}
	proxies := map[string]model.Proxy{
		"GLOBAL": {Name: "GLOBAL", Now: "Proxy", All: []string{"Proxy"}},
		"Proxy":  {Name: "Proxy", Now: "HK", All: []string{"HK"}},
		"HK":     {Name: "HK"},
	}

	got := buildStatusSnapshot("rule", conns, mem, proxies)

	assert.Equal(t, "rule", got.Mode)
	assert.Equal(t, 2, got.ActiveConnections)
	assert.Equal(t, int64(400), got.UploadSpeed)
	assert.Equal(t, int64(600), got.DownloadSpeed)
	assert.Equal(t, int64(1024), got.MemoryInuse)
	assert.Equal(t, int64(2048), got.MemoryOSLimit)
	require.Len(t, got.SelectedGroups, 2)
	assert.Equal(t, statusSelectedGroup{Name: "GLOBAL", Current: "Proxy", Leaf: "HK"}, got.SelectedGroups[0])
	assert.Equal(t, statusSelectedGroup{Name: "Proxy", Current: "HK", Leaf: "HK"}, got.SelectedGroups[1])
}

func TestResolveSelectedGroupsSkipsMissingGroupsAndAvoidsLoops(t *testing.T) {
	proxies := map[string]model.Proxy{
		"GLOBAL": {Name: "GLOBAL", Now: "Proxy", All: []string{"Proxy"}},
		"Proxy":  {Name: "Proxy", Now: "GLOBAL", All: []string{"GLOBAL"}},
	}

	got := resolveSelectedGroups(proxies)

	require.Len(t, got, 2)
	assert.Equal(t, statusSelectedGroup{Name: "GLOBAL", Current: "Proxy"}, got[0])
	assert.Equal(t, statusSelectedGroup{Name: "Proxy", Current: "GLOBAL"}, got[1])
}

func TestRenderStatusJSON(t *testing.T) {
	snapshot := statusSnapshot{
		Mode:              "global",
		ActiveConnections: 1,
		UploadSpeed:       512,
		DownloadSpeed:     1024,
		MemoryInuse:       2048,
		MemoryOSLimit:     4096,
		SelectedGroups: []statusSelectedGroup{
			{Name: "GLOBAL", Current: "Proxy", Leaf: "HK"},
		},
	}

	var out bytes.Buffer
	err := renderStatus(&out, snapshot, outputFormatJSON)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, "global", got["mode"])
	assert.Equal(t, float64(1), got["active_connections"])
	assert.Equal(t, float64(512), got["upload_speed"])
	assert.Equal(t, float64(1024), got["download_speed"])
	assert.Equal(t, float64(2048), got["memory_inuse"])
	assert.Equal(t, float64(4096), got["memory_oslimit"])
	assert.Contains(t, out.String(), `"selected_groups"`)
	assert.Contains(t, out.String(), `"leaf": "HK"`)
}

func TestRenderStatusPlainAndTable(t *testing.T) {
	snapshot := statusSnapshot{
		Mode:              "direct",
		ActiveConnections: 3,
		UploadSpeed:       1024,
		DownloadSpeed:     2048,
		MemoryInuse:       4096,
		MemoryOSLimit:     8192,
		SelectedGroups: []statusSelectedGroup{
			{Name: "GLOBAL", Current: "Proxy", Leaf: "HK"},
		},
	}

	for _, tt := range []struct {
		name     string
		format   outputFormat
		contains []string
	}{
		{
			name:   "plain",
			format: outputFormatPlain,
			contains: []string{
				"模式: direct",
				"活跃连接数: 3",
				"上传速率: 1.0 KB/s",
				"下载速率: 2.0 KB/s",
				"内存: 4.0 KB / 8.0 KB",
				"GLOBAL: Proxy -> HK",
			},
		},
		{
			name:   "table",
			format: outputFormatTable,
			contains: []string{
				"KEY",
				"MODE",
				"ACTIVE_CONNECTIONS",
				"UPLOAD_SPEED",
				"1.0 KB/s",
				"GLOBAL",
				"Proxy -> HK",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := renderStatus(&out, snapshot, tt.format)
			require.NoError(t, err)
			output := out.String()
			for _, want := range tt.contains {
				assert.True(t, strings.Contains(output, want), "expected %q in %q", want, output)
			}
		})
	}
}
