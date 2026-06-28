package settings

import (
	"context"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/logs"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

var activeStatusRe = regexp.MustCompile(`Active: (.*? (?:.*?\)))`)

type SysStatusState struct {
	Supported  bool
	Viewport   viewport.Model
	LastTick   time.Time
	LastOutput string
}

func NewSysStatusState() SysStatusState {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	return SysStatusState{
		Supported: runtime.GOOS == "linux",
		Viewport:  vp,
	}
}

func (s *SysStatusState) SyncDimensions(width, remainingHeight int) {
	if !s.Supported {
		return
	}
	s.Viewport.Width = width - 4
	if remainingHeight < 0 {
		remainingHeight = 0
	}
	s.Viewport.Height = remainingHeight
}

func (s *SysStatusState) UpdateContent(sysLogs []model.LogEntry) {
	if !s.Supported {
		return
	}
	combined := s.LastOutput
	if len(sysLogs) > 0 {
		combined += "\n\n"
		for _, lg := range sysLogs {
			// logs.RenderLogEntry width parameter is usually 0 if we want it to wrap itself, but we can pass viewport width.
			combined += logs.RenderLogEntry(lg, false, s.Viewport.Width, 0) + "\n"
		}
	}
	s.Viewport.SetContent(combined)
}

func FetchSysStatusCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "systemctl", "status", "mihomo", "--no-pager", "-n", "0")
		// systemctl often returns exit code 3 if service is not running but status is requested
		out, err := cmd.CombinedOutput()
		return messages.SysStatusResultMsg{
			Output: string(out),
			Err:    err,
		}
	}
}

func TickSysStatusCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return messages.SysStatusTickMsg(t)
	})
}

func (s *SysStatusState) Update(msg tea.Msg) tea.Cmd {
	if !s.Supported {
		return nil
	}

	switch msg := msg.(type) {
	case messages.SysStatusTickMsg:
		s.LastTick = time.Time(msg)
		return FetchSysStatusCmd()

	case messages.SysStatusResultMsg:
		if msg.Output != "" {
			parsed := parseSystemctlStatusCompact(msg.Output)
			if parsed != s.LastOutput {
				s.LastOutput = parsed
			}
		} else if msg.Err != nil {
			s.LastOutput = msg.Err.Error()
		}
		return TickSysStatusCmd()

	case tea.MouseMsg:
		var cmd tea.Cmd
		s.Viewport, cmd = s.Viewport.Update(msg)
		return cmd
	}

	return nil
}

func parseSystemctlStatusCompact(output string) string {
	if output == "" {
		return ""
	}

	loaded := "unknown"
	active := "unknown"
	pid := "unknown"
	mem := "unknown"
	cpu := "unknown"

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Loaded:") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) >= 2 {
				loaded = parts[1]
			}
		} else if strings.HasPrefix(line, "Active:") {
			if matches := activeStatusRe.FindStringSubmatch(line); len(matches) > 1 {
				active = matches[1]
			} else {
				parts := strings.SplitN(line, " ", 3)
				if len(parts) >= 2 {
					active = parts[1]
				}
			}
		} else if strings.HasPrefix(line, "Main PID:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				pid = parts[2]
			}
		} else if strings.HasPrefix(line, "Memory:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				mem = parts[1]
			}
		} else if strings.HasPrefix(line, "CPU:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				cpu = parts[1]
			}
		}
	}

	return "Service: mihomo (" + loaded + ") | PID: " + pid + "\nState: " + active + " | Mem: " + mem + " | CPU: " + cpu
}
