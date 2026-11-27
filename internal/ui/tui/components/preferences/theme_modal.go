package preferences

import (
	"time"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ThemeModalState holds page-local state for Settings -> Theme Picker modal.
type ThemeModalState struct {
	Open   bool
	Cursor int
	Offset int
}

// ThemeModalOpen reports whether the theme picker modal is currently open.
func (s State) ThemeModalOpen() bool {
	return s.themeModal.Open
}

// OpenThemeModal opens the theme picker modal focused on the currently active theme.
func (s State) OpenThemeModal() State {
	current := theme.CurrentName()
	if s.Config != nil && s.Config.Theme != "" && theme.IsValid(s.Config.Theme) {
		current = s.Config.Theme
	}
	cursor := 0
	for i, name := range theme.Names() {
		if name == current {
			cursor = i
			break
		}
	}
	s.editMode = false
	s.editValue = ""
	s.editCursor = 0
	s.themeModal = ThemeModalState{
		Open:   true,
		Cursor: cursor,
	}
	return s
}

func (s State) handleThemeModalKey(msg tea.KeyMsg) (State, tea.Cmd) {
	names := theme.Names()
	if len(names) == 0 {
		s.themeModal = ThemeModalState{}
		return s, nil
	}

	m := &s.themeModal
	m.Cursor = max(0, min(m.Cursor, len(names)-1))

	switch {
	case key.Matches(msg, common.Keys.Escape), msg.String() == "q":
		s.themeModal = ThemeModalState{}
		return s, nil
	case key.Matches(msg, common.Keys.Up):
		if m.Cursor > 0 {
			m.Cursor--
		}
		return s, nil
	case key.Matches(msg, common.Keys.Down):
		if m.Cursor < len(names)-1 {
			m.Cursor++
		}
		return s, nil
	case key.Matches(msg, common.Keys.Home), msg.String() == "g":
		m.Cursor = 0
		return s, nil
	case key.Matches(msg, common.Keys.End), msg.String() == "G":
		m.Cursor = len(names) - 1
		return s, nil
	case msg.String() == "pgup":
		m.Cursor = max(0, m.Cursor-5)
		return s, nil
	case msg.String() == "pgdown":
		m.Cursor = min(len(names)-1, m.Cursor+5)
		return s, nil
	case key.Matches(msg, common.Keys.Enter):
		return s.applyThemeSelection(names[m.Cursor])
	}
	return s, nil
}

func (s State) handleThemeModalScroll(up bool) State {
	names := theme.Names()
	if len(names) == 0 {
		return s
	}
	if up && s.themeModal.Cursor > 0 {
		s.themeModal.Cursor--
	} else if !up && s.themeModal.Cursor < len(names)-1 {
		s.themeModal.Cursor++
	}
	return s
}

func (s State) handleThemeModalClick(pageX, pageY, pageWidth, pageHeight int) (State, tea.Cmd) {
	modal := buildThemeModal(s.ToPageState(s.Config, nil), pageWidth, pageHeight)
	left, top, right, bottom := modal.Bounds()
	if pageY < top || pageY >= bottom || pageX < left || pageX >= right {
		s.themeModal = ThemeModalState{}
		return s, nil
	}

	x, y := pageX-left-2, pageY-top
	if x < 0 || x >= modal.innerWidth {
		return s, nil
	}

	if row := y - modal.hintTop; row >= 0 && row < len(modal.hints) {
		if hint, ok := common.InlineHintAt(modal.hints[row], x, modal.innerWidth); ok {
			keyMsg, _ := hint.KeyMsg()
			return s.handleThemeModalKey(keyMsg)
		}
	}

	row := y - modal.listTop
	if row >= 0 && row < modal.listCount && x < modal.listWidth {
		names := theme.Names()
		idx := modal.listStart + row
		if idx >= 0 && idx < len(names) {
			alreadySelected := idx == s.themeModal.Cursor
			s.themeModal.Cursor = idx
			s.themeModal.Offset = modal.listStart
			if alreadySelected || s.doubleClickDetector.IsDoubleClick(struct{}{}, 1000+idx, time.Now()) {
				return s.applyThemeSelection(names[idx])
			}
			return s, nil
		}
	}

	return s, nil
}

func (s State) applyThemeSelection(chosen string) (State, tea.Cmd) {
	if s.configSvc == nil || s.configSvc.SetConfigValue("theme", chosen) != nil {
		return s, noticeCmd(i18n.T("settings.toast.save_failed"), messages.NoticeError)
	}
	s.reloadConfig()
	theme.SetTheme(chosen)
	s.themeModal = ThemeModalState{}
	return s, tea.Batch(
		func() tea.Msg { return messages.ThemeChangedMsg{} },
		noticeCmd(i18n.T("settings.toast.save_success_theme"), messages.NoticeSuccess),
	)
}
