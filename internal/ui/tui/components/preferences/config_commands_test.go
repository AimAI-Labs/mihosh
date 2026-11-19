package preferences

import (
	"errors"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
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

func TestOverrideReadsPreserveDraftAndRejectStaleResults(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	require.NoError(t, profile.WriteMerge([]byte("allow-lan: false\nlog-level: info\nsecret: local\n")))
	s := NewMihomoState(nil, nil)
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.Nil(t, cmd, "the form must be loaded before saving")

	loaded := FetchMihomoOverrides(nil)().(messages.MihomoOverridesMsg)
	require.NoError(t, loaded.Err)
	require.Equal(t, "false", loaded.Overrides["allow-lan"])
	s = s.ApplyMihomoOverrides(&loaded)
	s.setDraftOverride("allow-lan", "")

	require.NoError(t, profile.WriteMerge([]byte("allow-lan: false\nlog-level: debug\nsecret: local\n")))
	refreshed := FetchMihomoOverrides(nil)().(messages.MihomoOverridesMsg)
	s = s.ApplyMihomoOverrides(&refreshed)
	require.True(t, s.HasUnsavedOverrides())
	require.Equal(t, "", s.getDraftOverride("allow-lan"))
	require.Equal(t, "debug", s.getDraftOverride("log-level"))

	s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)
	s = s.CompleteMihomoOverridesSave(&messages.MihomoOverridesSavedMsg{
		WriteOK: true, Overrides: map[string]string{"log-level": "debug", "secret": "local"},
	})
	s = s.ApplyMihomoOverrides(&refreshed)
	require.False(t, s.HasUnsavedOverrides())
	require.Empty(t, s.getDraftOverride("allow-lan"))

	s = s.ApplyMihomoOverrides(&messages.MihomoOverridesMsg{ReadAt: time.Now(), Err: errors.New("invalid merge")})
	_, cmd = s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.Nil(t, cmd, "a failed read must block saving over the file")
}
