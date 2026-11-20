package tui

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

func (m *Model) publishNotice(text string, level messages.NoticeLevel) {
	m.notice.Publish(messages.NoticeMsg{Text: text, Level: level})
}

func (m *Model) publishError(err error) {
	if err == nil {
		return
	}
	detail := err.Error()
	text := detail
	switch {
	case strings.Contains(detail, "context dead"):
		text = i18n.T("status.err.timeout_node")
	case strings.Contains(detail, "connection refused"):
		text = i18n.T("status.err.refused")
	case strings.Contains(detail, "timeout"):
		text = i18n.T("status.err.timeout")
	}
	m.notice.Publish(messages.NoticeMsg{Text: text, Level: messages.NoticeError, Detail: detail})
}

func (m *Model) clearErrorNotice() {
	if m.notice.Message.Level == messages.NoticeError {
		m.notice.Clear()
	}
}

func (m Model) visibleNotice() messages.NoticeMsg {
	if m.notice.Message.Text != "" || !m.nodesState.Testing {
		return m.notice.Message
	}
	text := i18n.T("status.testing")
	if target := strings.TrimSpace(m.nodesState.TestingTarget); target != "" {
		text += ": " + target
	}
	return messages.NoticeMsg{Text: text, Level: messages.NoticeInfo}
}
