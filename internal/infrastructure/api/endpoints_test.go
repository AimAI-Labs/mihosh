package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/stretchr/testify/require"
)

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

	client := NewClient(&config.Config{
		APIAddress: server.URL,
		Timeout:    100,
	})

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

	client := NewClient(&config.Config{
		APIAddress: "http://127.0.0.1:1", // 故意指向不可达地址
		Secret:     "old-secret",
		Timeout:    100,
	})

	// 切换到真实测试服务并改密钥
	client.UpdateEndpoint(server.URL, "new-secret")

	_, err := client.GetVersion()
	require.NoError(t, err)
	require.Equal(t, "Bearer new-secret", gotAuth)
}
