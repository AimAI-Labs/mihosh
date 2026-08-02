package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/gorilla/websocket"
)

func TestClient_IgnoresEnvProxy(t *testing.T) {
	// 创建 mock HTTP 服务端
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"1.0.0"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// 解析 mock 服务地址
	rawURL := strings.TrimPrefix(ts.URL, "http://")

	// 在强行设置无效 HTTP_PROXY/HTTPS_PROXY 的环境变量下测试
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:59999")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:59999")
	t.Setenv("http_proxy", "http://127.0.0.1:59999")
	t.Setenv("https_proxy", "http://127.0.0.1:59999")
	t.Setenv("ALL_PROXY", "socks5://127.0.0.1:59999")
	t.Setenv("all_proxy", "socks5://127.0.0.1:59999")

	endpoint := config.MihomoEndpoint{
		ExternalController: rawURL,
	}

	client := NewClient(endpoint, 3000)
	ver, err := client.GetVersion()
	if err != nil {
		t.Fatalf("期望直连成功，但受到环境变量代理影响失败: %v", err)
	}

	if ver.Version != "1.0.0" {
		t.Errorf("期望版本号 1.0.0，实际得到: %s", ver.Version)
	}
}

func TestWSClient_IgnoresEnvProxy(t *testing.T) {
	upgrader := websocket.Upgrader{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteJSON(MemoryData{Inuse: 1024})
	}))
	defer ts.Close()

	rawURL := strings.TrimPrefix(ts.URL, "http://")

	t.Setenv("HTTP_PROXY", "http://127.0.0.1:59999")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:59999")
	t.Setenv("http_proxy", "http://127.0.0.1:59999")
	t.Setenv("https_proxy", "http://127.0.0.1:59999")

	received := make(chan bool, 1)
	wsClient := NewWSClient(rawURL, "")
	wsClient.SetMemoryHandler(func(data MemoryData) {
		if data.Inuse == 1024 {
			select {
			case received <- true:
			default:
			}
		}
	})

	err := wsClient.Start(t.Context())
	if err != nil {
		t.Fatalf("启动 WSClient 失败: %v", err)
	}
	defer wsClient.Stop()

	select {
	case <-received:
		// 成功接收到 WS 消息，说明没有走错误的代理端口
	case <-time.After(3 * time.Second):
		t.Fatal("超时未收到 WebSocket 消息，疑因受环境变量代理影响连接失败")
	}
}
