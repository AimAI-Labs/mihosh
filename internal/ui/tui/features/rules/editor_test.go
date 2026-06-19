package rules

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
)

func TestParseEditorFromRC(t *testing.T) {
	cases := []struct {
		name, content, want string
	}{
		{"unquoted", "export EDITOR=vim\n", "vim"},
		{"no_export", "EDITOR=nano\n", "nano"},
		{"double_quoted", "export EDITOR=\"vim\"\n", "vim"},
		{"single_quoted_with_args", "export EDITOR='code --wait'\n", "code --wait"},
		{"trailing_comment_unquoted", "export EDITOR=vim # use vim\n", "vim"},
		{"trailing_comment_quoted", "export EDITOR=\"nano\" # comment\n", "nano"},
		{"comment_line_ignored", "# EDITOR=vim\nexport EDITOR=nano\n", "nano"},
		{"last_wins", "export EDITOR=vim\nexport EDITOR=nano\n", "nano"},
		{"my_editor_not_matched", "export MY_EDITOR=vim\n", ""},
		{"no_match", "export PATH=/usr/bin\nalias e=vim\n", ""},
		{"empty_value_skipped", "export EDITOR=vim\nexport EDITOR=''\n", "vim"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseEditorFromRC(c.content); got != c.want {
				t.Fatalf("parseEditorFromRC: got %q, want %q", got, c.want)
			}
		})
	}
}

// TestState_EditConfigKey_ReturnsCommand 按 e 应返回非空命令（编辑器或错误命令）。
func TestState_EditConfigKey_ReturnsCommand(t *testing.T) {
	s := State{configPath: "/tmp/nonexistent-config.yaml"}
	_, cmd := s.Update(keyMsg('e'), nil)
	if cmd == nil {
		t.Fatal("expected a non-nil command when pressing 'e'")
	}
}

// TestState_EditConfig_NoConfigPath 配置路径缺失时返回携带错误的 ConfigEditFinishedMsg。
func TestState_EditConfig_NoConfigPath(t *testing.T) {
	s := State{configPath: ""}
	_, cmd := s.Update(keyMsg('e'), nil)
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
