package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestNewAppBindsHTTPAndWebSocketToSameConfiguredTarget(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	const secret = "shared-fixture-token"
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			if r.Header.Get("Authorization") != "Bearer "+secret {
				t.Error("HTTP client sent the wrong token")
			}
			_, _ = w.Write([]byte(`{"version":"same-controller"}`))
			return
		}
		if r.URL.Query().Get("token") != secret {
			t.Error("WebSocket client sent the wrong token")
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		if r.URL.Path == "/memory" {
			_ = conn.WriteJSON(api.MemoryData{Inuse: 818})
		}
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()
	cfg := config.DefaultConfig
	cfg.Core.Mode = "external"
	cfg.Core.ExternalController, cfg.Core.ExternalSecret = server.URL, secret
	require.NoError(t, config.Save(&cfg))
	app := NewApp()
	defer app.Close()
	require.Equal(t, server.URL, app.Target.Endpoint.ExternalController)
	require.Empty(t, app.Target.ConfigPath)
	version, err := app.Client.GetVersion()
	require.NoError(t, err)
	require.Equal(t, "same-controller", version.Version)
	memory := make(chan api.MemoryData, 1)
	app.WSClient.SetMemoryHandler(func(data api.MemoryData) { memory <- data })
	require.NoError(t, app.WSClient.Start(t.Context()))
	select {
	case data := <-memory:
		require.Equal(t, int64(818), data.Inuse)
	case <-time.After(2 * time.Second):
		t.Fatal("WebSocket did not reach the same configured controller")
	}
}

func TestNewAppWiresAllServicesAndHandlesMissingConfig(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())

	app := NewApp()
	defer app.Close()

	if app.Config == nil {
		t.Fatal("expected fallback default config, got nil")
	}
	if app.ConfigErr == nil {
		t.Fatal("expected ConfigErr when config file does not exist")
	}
	if app.Client == nil || app.WSClient == nil {
		t.Fatal("expected API and WS clients to be wired")
	}
	if app.ConfigSvc == nil || app.ConnSvc == nil || app.CoreSvc == nil || app.ProfileSvc == nil || app.IPResolver == nil {
		t.Fatal("expected all application services to be wired")
	}
	if app.ProxyService("https://example.com", 1000) == nil {
		t.Fatal("expected ProxyService factory to return non-nil service")
	}
}

func TestNewAppWithClientPreservesProvidedAndNilClient(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())

	provided := api.NewClient(config.MihomoEndpoint{ExternalController: "127.0.0.1:9999"}, 1500)
	app := NewAppWithClient(provided)
	defer app.Close()
	if app.Client != provided {
		t.Fatal("expected NewAppWithClient to preserve caller-supplied *api.Client")
	}

	offline := NewAppWithClient(nil)
	defer offline.Close()
	if offline.Client != nil {
		t.Fatal("expected NewAppWithClient(nil) to preserve nil client")
	}
}
