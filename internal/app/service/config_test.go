package service

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
)

func TestConfigService_FetchMihomoConfig_NilClient(t *testing.T) {
	s := NewConfigService()
	cmd := s.FetchMihomoConfig(nil)
	if cmd == nil {
		t.Fatal("expected a tea.Cmd, got nil")
	}

	msg := cmd()
	configMsg, ok := msg.(messages.MihomoConfigMsg)
	if !ok {
		t.Fatalf("expected messages.MihomoConfigMsg, got %T", msg)
	}

	// 始终从 YAML 文件读取，不再依赖 API
	if !configMsg.FromFile {
		t.Error("expected FromFile=true (always reads from YAML)")
	}
	// 从文件读取时无 API 错误
	if configMsg.Err != nil {
		t.Errorf("expected no error when reading from file, got %v", configMsg.Err)
	}
	// 即使 API client 为 nil，Config 也应从 YAML/默认值填充，不为 nil
	if configMsg.Config == nil {
		t.Fatal("expected Config to be populated from YAML, got nil")
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
