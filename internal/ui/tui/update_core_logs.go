package tui

import "github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"

func (m *Model) usesProcessLogs() bool {
	return m.coreSvc != nil && m.config != nil && m.config.Core.Mode == "managed" && m.config.Core.Runtime != "service" && m.coreSvc.UsesProcessLogs()
}

func (m *Model) syncProcessLogs() {
	if !m.usesProcessLogs() {
		return
	}
	output, start, running := m.coreSvc.ReadLogs(m.logsState.CoreLogCursor())
	m.logsState, _ = m.logsState.Update(messages.CoreProcessLogsMsg{Start: start, Output: output, Flush: !running})
}
