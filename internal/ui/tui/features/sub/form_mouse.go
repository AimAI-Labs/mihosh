package sub

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (s *State) activeForm() *addForm {
	if s.showEditForm {
		if s.editForm == nil {
			f := newAddForm()
			s.editForm = &f
		} else {
			f := *s.editForm
			s.editForm = &f
		}
		return s.editForm
	}
	if s.addForm == nil {
		f := newAddForm()
		s.addForm = &f
	} else {
		f := *s.addForm
		s.addForm = &f
	}
	return s.addForm
}

func (s State) clickForm(x, y, width, height int) (State, tea.Cmd) {
	f := s.activeForm()
	f.resize(width, height)
	_, fieldWidth, _ := formDimensions(width, height)
	if button := f.copyButton(fieldWidth); y == 2 && button != "" {
		left := formLabelWidth() + 1 + f.srcField.Width() + 1
		if x >= left && x < left+lipgloss.Width(button) {
			return s, copySourceCmd(f.srcField.Value())
		}
	}
	switch {
	case y == 0:
		f.fieldCursor = addFieldName
	case y == 1:
		f.fieldCursor = addFieldKind
		if x >= formLabelWidth()+1 {
			remote := x < formLabelWidth()+1+len("remote")+3
			if fieldWidth < 14 || remote != f.kindRemote {
				f.toggleKind()
			}
			f.kindExplicit = true
		}
	case y >= 2 && y < 2+f.srcField.Height():
		f.fieldCursor = addFieldSrc
	default:
		return s, nil
	}
	f.focusCurrent()
	f.errMsg = ""
	return s, nil
}

func (s State) scrollForm(msg messages.PageMouseScrollMsg) (State, tea.Cmd) {
	left, top, right, bottom := resolveFormBounds(s.ToPageState(msg.Width, msg.Height), msg.Width, msg.Height)
	if msg.X < left || msg.X >= right || msg.Y < top || msg.Y >= bottom {
		return s, nil
	}
	f := s.activeForm()
	f.resize(msg.Width, msg.Height)
	f.fieldCursor = addFieldSrc
	f.focusCurrent()
	key := tea.KeyDown
	if msg.Up {
		key = tea.KeyUp
	}
	var cmd tea.Cmd
	f.srcField, cmd = f.srcField.Update(tea.KeyMsg{Type: key})
	return s, cmd
}
