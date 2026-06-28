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
	if state.LastOutput == "" {
		t.Error("expected LastOutput to be updated")
	}
}

func TestParseSystemctlStatusCompact(t *testing.T) {
	input := `● mihomo.service - mihomo Daemon, Another Clash Kernel.
     Loaded: loaded (/etc/systemd/system/mihomo.service; enabled; preset: enabled)
     Active: active (running) since Sun 2026-06-28 13:22:20 CST; 54min ago
    Process: 1022073 ExecStartPre=/usr/bin/sleep 1s (code=exited, status=0/SUCCESS)
   Main PID: 1022080 (mihomo)
      Tasks: 11 (limit: 4291)
     Memory: 30.3M (peak: 31.7M)
        CPU: 12.589s
     CGroup: /system.slice/mihomo.service
             └─1022080 /home/ubuntu/Apps/local/mihomo/mihomo -d /home/ubuntu/Apps/local/mihomo`

	output := parseSystemctlStatusCompact(input)
	expected := "Service: mihomo (loaded) | PID: 1022080\nState: active (running) | Mem: 30.3M | CPU: 12.589s"

	if output != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, output)
	}

	// Test missing fields gracefully
	outputMissing := parseSystemctlStatusCompact("● mihomo.service - \n     Loaded: loaded\n")
	if outputMissing == "" {
		t.Errorf("Should not be empty on partial match")
	}
}
