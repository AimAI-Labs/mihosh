package settings

import (
	"context"
	"os/exec"
	"runtime"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type SysStatusTickMsg time.Time
type SysStatusResultMsg struct {
	Output string
	Err    error
}

type SysStatusState struct {
	Supported bool
	Viewport  viewport.Model
	LastTick  time.Time
}

func NewSysStatusState() SysStatusState {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	return SysStatusState{
		Supported: runtime.GOOS == "linux",
		Viewport:  vp,
	}
}

func FetchSysStatusCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "systemctl", "status", "mihomo", "--no-pager", "-n", "50")
		// systemctl often returns exit code 3 if service is not running but status is requested
		out, err := cmd.CombinedOutput()
		return SysStatusResultMsg{
			Output: string(out),
			Err:    err,
		}
	}
}

func TickSysStatusCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return SysStatusTickMsg(t)
	})
}

func (s *SysStatusState) Update(msg tea.Msg) tea.Cmd {
	if !s.Supported {
		return nil
	}

	switch msg := msg.(type) {
	case SysStatusTickMsg:
		s.LastTick = time.Time(msg)
		return FetchSysStatusCmd()

	case SysStatusResultMsg:
		// Only update if changed to avoid flicker
		if msg.Output != "" {
			// Using SetContent resets to top, so we should try to avoid resetting scroll if possible,
			// or just set it. For simplicity, just set it.
			s.Viewport.SetContent(msg.Output)
		} else if msg.Err != nil {
			s.Viewport.SetContent(msg.Err.Error())
		}
		return TickSysStatusCmd()

	case tea.MouseMsg:
		var cmd tea.Cmd
		s.Viewport, cmd = s.Viewport.Update(msg)
		return cmd
	}

	return nil
}
