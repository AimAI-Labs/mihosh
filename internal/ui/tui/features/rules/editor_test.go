package rules

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
)

// TestState_EditConfigKey_ReturnsCommand 按 e 应返回非空命令（编辑器或错误命令）。
func TestState_EditConfigKey_ReturnsCommand(t *testing.T) {
	s := State{configPath: "/tmp/nonexistent-config.yaml"}
	_, cmd := s.Update(keyMsg('e'))
	if cmd == nil {
		t.Fatal("expected a non-nil command when pressing 'e'")
	}
}

// TestState_EditConfig_NoConfigPath 配置路径缺失时返回携带错误的 ConfigEditFinishedMsg。
func TestState_EditConfig_NoConfigPath(t *testing.T) {
	s := State{configPath: ""}
	_, cmd := s.Update(keyMsg('e'))
	if cmd == nil {
		t.Fatal("expected error command when config path is empty")
	}
	msg := cmd()
	ce, ok := msg.(messages.ConfigEditFinishedMsg)
	if !ok {
		t.Fatalf("expected ConfigEditFinishedMsg, got %T", msg)
	}
	if ce.Err == nil {
		t.Fatal("expected non-nil error when config path is empty")
	}
}
