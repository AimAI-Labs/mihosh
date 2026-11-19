package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReloadConfigPayloadSendsConfigurationWithoutPath(t *testing.T) {
	const payload = "mode: rule\nrules:\n  - MATCH,DIRECT\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/configs", r.URL.Path)
		assert.Equal(t, "true", r.URL.Query().Get("force"))
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		assert.Equal(t, map[string]string{"payload": payload}, body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)
	require.NoError(t, client.ReloadConfigPayload([]byte(payload)))
}

func TestPatchConfigAndAPIError(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && r.URL.Path == "/configs" {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "bad config", http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)
	require.NoError(t, client.PatchConfig(map[string]any{"allow-lan": false, "log-level": "debug", "mixed-port": 7891}))
	assert.Equal(t, false, got["allow-lan"])
	assert.Equal(t, "debug", got["log-level"])
	assert.Equal(t, float64(7891), got["mixed-port"])

	err := client.ReloadConfigPayload([]byte("invalid"))
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Contains(t, apiErr.Body, "bad config")
}

func TestGetMemoryDecodesFirstObjectFromOpenStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/memory", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"inuse":1024,"oslimit":2048}` + "\n"))
		require.NoError(t, err)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient(config.MihomoEndpoint{
		ExternalController: server.URL,
	}, 100)

	start := time.Now()
	mem, err := client.GetMemory()

	require.NoError(t, err)
	require.Less(t, time.Since(start), 100*time.Millisecond)
	require.Equal(t, int64(1024), mem.Inuse)
	require.Equal(t, int64(2048), mem.OSLimit)
}

// TestUpdateEndpointSwitchesBaseURLAndSecret 验证 UpdateEndpoint 后，
// 后续请求会使用新的 baseURL 与 secret（而非构造时固化的旧值）。
// 这是「手动改 mihomo 配置后 mihosh 报无法连接」问题的核心修复点。
func TestUpdateEndpointSwitchesBaseURLAndSecret(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/version", r.URL.Path)
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"version":"test"}`))
	}))
	defer server.Close()

	client := NewClient(config.MihomoEndpoint{
		ExternalController: "http://127.0.0.1:1", // 故意指向不可达地址
		Secret:             "old-secret",
	}, 100)

	// 切换到真实测试服务并改密钥
	client.UpdateEndpoint(server.URL, "new-secret")

	_, err := client.GetVersion()
	require.NoError(t, err)
	require.Equal(t, "Bearer new-secret", gotAuth)
}
