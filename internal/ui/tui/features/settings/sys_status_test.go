package settings

import (
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
)

func TestSysStatusState_Update(t *testing.T) {
	state := NewSysStatusState()
	state.Supported = true
	state.Viewport.Width = 80
	state.Viewport.Height = 20

	// Test TickMsg triggers Cmd
	cmd := state.Update(messages.SysStatusTickMsg(time.Now()))
	if cmd == nil {
		t.Error("expected cmd to be returned on TickMsg")
	}

	// Test ResultMsg updates viewport and returns new tick
	resultMsg := messages.SysStatusResultMsg{Output: "active (running)", Err: nil}
	cmd = state.Update(resultMsg)
	if cmd == nil {
		t.Error("expected new tick cmd on ResultMsg")
	}
	if state.Viewport.View() == "" {
		t.Error("expected viewport to be updated")
	}
}
