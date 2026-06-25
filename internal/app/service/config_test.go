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

	// API 不可达时降级到 YAML 文件读取
	if !configMsg.FromFile {
		t.Error("expected FromFile=true for nil client fallback")
	}
	if configMsg.Err == nil || configMsg.Err.Error() != "API client is nil" {
		t.Errorf("expected err 'API client is nil', got %v", configMsg.Err)
	}
	// 即使 API 不可达，Config 也应从 YAML/默认值填充，不为 nil
	if configMsg.Config == nil {
		t.Fatal("expected Config to be populated from YAML fallback, got nil")
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
