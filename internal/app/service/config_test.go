package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
)

func managedConfigFile(t *testing.T, contents string) string {
	t.Helper()
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	cfg := config.DefaultConfig
	cfg.Core.Mode = "managed"
	if err := config.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	path, err := config.ManagedMihomoConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigReadUsesFileValues(t *testing.T) {
	managedConfigFile(t, "external-controller: 127.0.0.1:9191\nsecret: from-file\nmixed-port: 7891\nallow-lan: true\nlog-level: debug\n")
	result := NewConfigService().ReadMihomoConfig()
	if result.ExternalController != "127.0.0.1:9191" || result.Secret != "from-file" || result.MixedPort != 7891 || !result.AllowLan || result.LogLevel != "debug" {
		t.Fatalf("unexpected file configuration: %+v", result)
	}
}

func TestConfigReadRetainsDefaultFallback(t *testing.T) {
	path := managedConfigFile(t, "")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	result := NewConfigService().ReadMihomoConfig()
	if result == nil || result.ExternalController != config.DefaultMihomoEndpoint.ExternalController || result.MixedPort != config.DefaultMihomoEndpoint.MixedPort {
		t.Fatalf("default fallback changed: %+v", result)
	}
}

func TestConfigService_SaveMihomoConfigField_Success(t *testing.T) {
	// We can't fully test success without mocking the file system and API client easily,
	// but we can at least invoke it to make sure it doesn't panic.
	s := NewConfigService()
	cmd := s.SaveMihomoConfigField(nil, "test_key", "test_value")
	if cmd == nil {
		t.Fatal("expected a tea.Cmd, got nil")
	}

	// Invoking the returned tea.Msg would write to the actual config path or fail
	// depending on the environment, so we just check it returns a function
	// and doesn't crash on construction.
}

func TestConfigService_SaveMihomoConfigField_FirstLaunchWritesYAML(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)
	t.Setenv("APPDATA", tempDir)
	isolateMihomoDiscovery(t)

	s := NewConfigService()
	cmd := s.SaveMihomoConfigField(nil, "external-controller", "127.0.0.1:9090")
	requireCmdNotNil := func() messages.MihomoConfigSavedMsg {
		msg := cmd()
		savedMsg, ok := msg.(messages.MihomoConfigSavedMsg)
		if !ok {
			t.Fatalf("expected MihomoConfigSavedMsg, got %T", msg)
		}
		return savedMsg
	}

	saved := requireCmdNotNil()
	if !saved.WriteOK {
		t.Errorf("expected WriteOK to be true on first launch save, got false (err: %v)", saved.Err)
	}
}
