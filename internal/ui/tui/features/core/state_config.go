package core

import (
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func isCoreTabKey(key string) bool {
	switch key {
	case "1", "2", "3", "4", "5", "left", "right":
		return true
	}
	return false
}

func (s State) IsMihomoPage() bool { return s.tab == 4 }

func (s State) updateEditorMouse(msg tea.Msg, top int) (State, tea.Cmd) {
	if s.Busy || s.editor.IsSaving() {
		return s, nil
	}
	switch event := msg.(type) {
	case messages.PageMouseClickMsg:
		if event.Y < top {
			return s, nil
		}
		event.Y += 4 - top
		event.Height = max(1, event.Height-footerHeight)
		msg = event
	case messages.PageMouseScrollMsg:
		if event.Y < top {
			return s, nil
		}
		event.Y += 4 - top
		event.Height = max(1, event.Height-footerHeight)
		msg = event
	}
	var cmd tea.Cmd
	s.editor, cmd = s.editor.Update(msg)
	return s, cmd
}

func (s State) HandleEditorMsg(msg tea.Msg, width, height int, sysLogs []model.LogEntry) (State, tea.Cmd) {
	var cmd tea.Cmd
	s.editor, cmd = s.editor.HandleMsg(msg, width, max(1, height-footerHeight), s.editor.Config, sysLogs)
	return s, cmd
}
