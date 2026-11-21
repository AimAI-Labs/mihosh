package bootstrap

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
)

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
