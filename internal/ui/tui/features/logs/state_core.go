package logs

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/charmbracelet/x/ansi"
)

var coreLogLevel = regexp.MustCompile(`\blevel=(debug|info|warn|warning|error|fatal|panic)\b`)

func (s State) CoreLogCursor() uint64 { return s.coreLogCursor }

func (s State) applyProcessLogs(msg messages.CoreProcessLogsMsg) State {
	end := msg.Start + uint64(len(msg.Output))
	if end < s.coreLogCursor {
		return s
	}
	if msg.Start > s.coreLogCursor {
		s.coreLogPending = ""
	}
	skip := min(uint64(len(msg.Output)), s.coreLogCursor-min(s.coreLogCursor, msg.Start))
	text := s.coreLogPending + msg.Output[skip:]
	s.coreLogCursor = end
	lines := strings.Split(text, "\n")
	s.coreLogPending = lines[len(lines)-1]
	lines = lines[:len(lines)-1]
	if msg.Flush && s.coreLogPending != "" {
		lines = append(lines, s.coreLogPending)
		s.coreLogPending = ""
	}
	if len(s.coreLogPending) > 64<<10 {
		s.coreLogPending = s.coreLogPending[len(s.coreLogPending)-(64<<10):]
	}
	for _, line := range lines {
		line = strings.TrimSpace(ansi.Strip(line))
		if line == "" {
			continue
		}
		level := "info"
		if match := coreLogLevel.FindStringSubmatch(line); match != nil {
			level = match[1]
			switch level {
			case "warn":
				level = "warning"
			case "fatal", "panic":
				level = "error"
			}
		}
		if _, payload, ok := strings.Cut(line, " msg="); ok {
			if decoded, err := strconv.Unquote(payload); err == nil {
				line = decoded
			}
		}
		s = s.AppendLog(level, line)
	}
	return s
}
