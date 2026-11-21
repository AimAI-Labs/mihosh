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

type serviceQuickGroup struct {
	label   string
	actions []string
}

func (s State) hasServiceCard() bool {
	return s.Status.ServiceUnit != ""
}

func (s State) serviceQuickGroups() []serviceQuickGroup {
	if !s.Status.ServiceInstalled {
		return []serviceQuickGroup{{actions: []string{"install"}}}
	}
	lifecycle := []string{"start"}
	if s.Status.ServiceRunning {
		lifecycle = []string{"stop", "restart"}
	}
	return []serviceQuickGroup{
		{"runtime", lifecycle},
		{"autostart", []string{"enable", "boot", "disable"}},
		{actions: []string{"uninstall"}},
	}
}

func (s State) serviceQuickActions() []string {
	var actions []string
	for _, group := range s.serviceQuickGroups() {
		actions = append(actions, group.actions...)
	}
	return actions
}

func (s State) clampServiceSelection() State {
	if !s.hasServiceCard() {
		s.serviceFocus, s.serviceActionIdx = false, 0
		return s
	}
	actions := s.serviceQuickActions()
	if len(actions) == 0 {
		s.serviceActionIdx = 0
		return s
	}
	s.serviceActionIdx = max(0, min(s.serviceActionIdx, len(actions)-1))
	return s
}

func (s State) openServiceStatus() (State, tea.Cmd) {
	s, cmd := s.openTab(4)
	if s.hasServiceCard() {
		s.serviceFocus = true
		s.pendingServiceFocus = false
		s.serviceActionIdx = 0
	} else if s.Status.Mode == "managed" || s.Status.Mode == "" {
		s.pendingServiceFocus = true
	}
	return s, cmd
}

func (s State) serviceScrollMax() int {
	width := s.viewWidth
	if width <= 0 {
		width = 100
	}
	height := s.viewHeight
	if height <= 0 {
		height = 28
	}
	return s.layout(width, max(1, height-footerHeight)).serviceMaxScroll
}

func (s State) updateServiceCardKey(msg tea.KeyMsg) (State, tea.Cmd) {
	actions := s.serviceQuickActions()
	s = s.clampServiceSelection()
	switch msg.String() {
	case "1", "2", "3", "4", "5":
		return s.openTab(int(msg.String()[0] - '1'))
	case "esc", "tab", "shift+tab":
		s.serviceFocus = false
		s.pendingServiceFocus = false
		return s, nil
	case "r":
		if s.Busy || s.editor.IsSaving() {
			return s, nil
		}
		return s, s.Refresh()
	case "left":
		if len(actions) > 0 {
			s.serviceActionIdx = max(0, s.serviceActionIdx-1)
		}
		return s, nil
	case "right":
		if len(actions) > 0 {
			s.serviceActionIdx = min(len(actions)-1, s.serviceActionIdx+1)
		}
		return s, nil
	case "up", "k":
		if s.serviceScroll > 0 {
			s.serviceScroll--
			return s, nil
		}
		s.serviceFocus = false
		s.pendingServiceFocus = false
		return s, nil
	case "down", "j":
		if maxScroll := s.serviceScrollMax(); s.serviceScroll < maxScroll {
			s.serviceScroll = min(maxScroll, s.serviceScroll+1)
		}
		return s, nil
	case "pgup":
		s.serviceScroll = max(0, s.serviceScroll-5)
		return s, nil
	case "pgdown":
		maxScroll := s.serviceScrollMax()
		s.serviceScroll = min(maxScroll, s.serviceScroll+5)
		return s, nil
	case "enter", " ":
		if s.Busy || s.editor.IsSaving() || len(actions) == 0 {
			return s, nil
		}
		s.editor = s.editor.CancelEdit()
		s.args = []string{actions[s.serviceActionIdx]}
		return s.begin("service")
	}
	return s, nil
}

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
