package logs

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
)

func TestProcessLogsHandleChunksAndDoNotReplay(t *testing.T) {
	s := NewState(nil)
	s = s.applyProcessLogs(messages.CoreProcessLogsMsg{Output: "time=now level=warning msg=\"startup"})
	if len(s.Logs()) != 0 {
		t.Fatal("partial line was emitted")
	}
	cursor := s.CoreLogCursor()
	s = s.applyProcessLogs(messages.CoreProcessLogsMsg{Start: cursor, Output: " failed\"\n"})
	if len(s.Logs()) != 1 || s.Logs()[0].Type != "warning" || s.Logs()[0].Payload != "startup failed" {
		t.Fatal("chunked startup log was lost", s.Logs())
	}
	s = s.applyProcessLogs(messages.CoreProcessLogsMsg{Output: "old\n"})
	if len(s.Logs()) != 1 {
		t.Fatal("old output was replayed")
	}
	s = s.ClearLogs()
	s = s.applyProcessLogs(messages.CoreProcessLogsMsg{Start: s.CoreLogCursor(), Output: "level=fatal msg=\"core exited\"", Flush: true})
	if len(s.Logs()) != 1 || s.Logs()[0].Type != "error" || s.Logs()[0].Payload != "core exited" {
		t.Fatal("unterminated final error was lost")
	}
}

func TestProcessLogsStayBoundedAndClearPendingOnTargetChange(t *testing.T) {
	s := NewState(nil)
	s = s.applyProcessLogs(messages.CoreProcessLogsMsg{Output: strings.Repeat("x", 64<<10)})
	s = s.applyProcessLogs(messages.CoreProcessLogsMsg{Start: s.CoreLogCursor(), Output: strings.Repeat("y", 64<<10)})
	if len(s.coreLogPending) != 64<<10 {
		t.Fatal("partial output grew beyond its limit")
	}
	s, _ = s.Update(messages.CoreTargetChangedMsg{Reset: true})
	s = s.applyProcessLogs(messages.CoreProcessLogsMsg{Start: s.CoreLogCursor(), Output: strings.Repeat("entry\n", common.LogsCap+10)})
	if len(s.Logs()) != common.LogsCap || s.Logs()[0].Payload != "entry" {
		t.Fatal("process logs exceeded the ring or retained previous target output")
	}
}
