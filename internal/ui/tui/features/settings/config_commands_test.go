package settings

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
)

func TestConfigReadAdapterBindsTargetAndKeepsFileMessage(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	cfg := config.DefaultConfig
	cfg.Core.Mode, cfg.Core.ExternalController = "external", "old-controller"
	if err := config.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	svc := service.NewConfigService()
	client := api.NewClient(config.MihomoEndpoint{ExternalController: "old-controller"}, 1000)
	cmd := FetchMihomoConfig(svc, client)
	client.UpdateEndpoint("new-controller", "")
	bound, ok := cmd().(messages.CoreHTTPMsg)
	if !ok || bound.Generation == client.Generation() {
		t.Fatal("config read adapter lost its scheduled target")
	}
	result, ok := bound.Payload.(messages.MihomoConfigMsg)
	if !ok || !result.FromFile || result.Err != nil || result.Config == nil {
		t.Fatalf("unexpected config read message: %+v", bound.Payload)
	}
	if result, ok := FetchMihomoConfig(svc, nil)().(messages.MihomoConfigMsg); !ok || result.Config == nil || !result.FromFile {
		t.Fatal("nil-client read no longer produces file/default configuration")
	}
}

func TestConfigWriteAdapterReportsWriteFailure(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	cfg := config.DefaultConfig
	cfg.Core.Mode, cfg.Core.ExternalController = "external", "remote-controller"
	if err := config.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	command := SaveMihomoConfigField(service.NewConfigService(), nil, "mixed-port", 7891)
	result, ok := command().(messages.MihomoConfigSavedMsg)
	if !ok || result.WriteOK || result.Err == nil {
		t.Fatalf("write adapter lost failure semantics: %+v", result)
	}
}
