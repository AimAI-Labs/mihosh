package core

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (s State) choiceLayout(width int) pageLayout {
	t := theme.Current()
	title := i18n.T("core.action." + s.form)
	desc := i18n.T("core.desc." + s.form)
	if id, ok := strings.CutPrefix(s.form, "api:"); ok {
		title = i18n.T("settings.action." + id)
		desc = i18n.T("core.desc." + id)
	}
	if s.form == "geo-update" {
		desc = i18n.Tf("core.desc.geo-update", s.geoConfirmTarget())
	}
	rows := strings.Split(wrap(desc, width-4), "\n")
	rows = append(rows, "")
	confirmLabel := " " + i18n.T("core.dialog.confirm") + " "
	cancelLabel := " " + i18n.T("core.dialog.cancel") + " "
	controls := []releaseControl{
		{actionHit: actionHit{x: 2, y: len(rows) + 1, width: lipgloss.Width(confirmLabel), height: 1}, action: "confirm"},
		{actionHit: actionHit{x: 4 + lipgloss.Width(confirmLabel), y: len(rows) + 1, width: lipgloss.Width(cancelLabel), height: 1}, action: "cancel"},
	}
	confirmBtn := lipgloss.NewStyle().Foreground(theme.Resolve(t.Primary)).Background(theme.Resolve(t.Selected)).Bold(true).Render(confirmLabel)
	cancelBtn := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted)).Render(cancelLabel)
	rows = append(rows, confirmBtn+"  "+cancelBtn)
	if s.formErr != nil {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(theme.Resolve(t.Danger)).Render(wrap(s.formErr.Error(), width-4)))
	}
	return pageLayout{content: panel(title, strings.Join(rows, "\n"), width), controls: controls}
}

func (s State) geoConfirmTarget() string {
	if len(s.args) > 0 {
		for _, f := range s.geoFiles {
			if f.Key == s.args[0] {
				return f.Name
			}
		}
		return s.args[0]
	}
	return "GEO"
}

// The renderer and mouse handler use the same clipped, centered dialog bounds.
func (s State) formDialog(width, height int) (pageLayout, int) {
	popupWidth := max(24, min(76, width*2/3))
	popupWidth = max(24, min(width, popupWidth))
	var page pageLayout
	switch s.form {
	case "details":
		page.content = s.coreDetails(popupWidth)
	default:
		if isChoiceForm(s.form) {
			page = s.choiceLayout(popupWidth)
			page.focusY = page.controls[0].y
		} else {
			page = s.formLayout(popupWidth)
		}
	}
	lines := strings.Split(page.content, "\n")
	budget := max(1, height-5)
	body := lines[1 : len(lines)-1]
	page.maxScroll = max(0, len(body)-budget)
	offset := max(0, min(s.dialogScroll, page.maxScroll))
	if s.formFollowFocus {
		if page.focusY-1 < offset {
			offset = max(0, page.focusY-1)
		}
		if page.focusY > offset+budget {
			offset = min(page.maxScroll, page.focusY-budget)
		}
	}
	page.scrollOffset = offset
	body = body[offset:min(len(body), offset+budget)]
	// Retain the frame and a fixed hint when scrolling a tall form or menu.
	page.content = strings.Join(append(append([]string{lines[0]}, body...),
		common.TokyoBlueStyle().Render("│ ")+common.PadString(ansi.Truncate(s.inlineHelp(popupWidth-3), popupWidth-4, ""), popupWidth-4)+common.TokyoBlueStyle().Render(" │"),
		lines[len(lines)-1]), "\n")
	x, y, _, _ := dialog.New(page.content, width, height).Clipped().Bounds()
	for i := range page.actions {
		hit := &page.actions[i]
		hit.y -= offset
		if hit.y < 1 || hit.y >= len(body)+1 {
			hit.height = 0
		} else {
			hit.height = min(hit.height, len(body)+1-hit.y)
		}
		hit.x, hit.y = hit.x+x, hit.y+y
	}
	for i := range page.controls {
		hit := &page.controls[i].actionHit
		hit.y -= offset
		if hit.y < 1 || hit.y >= len(body)+1 {
			hit.height = 0
		}
		hit.x, hit.y = hit.x+x, hit.y+y
	}
	return page, popupWidth
}

func (s State) overlayForm(base string, width, height int) string {
	page, _ := s.formDialog(width, height)
	return dialog.New(page.content, width, height).Clipped().Overlay(base, true)
}

func (s State) clickForm(msg messages.PageMouseClickMsg) (State, tea.Cmd) {
	if msg.Width <= 0 || msg.Height <= 0 {
		return s, nil
	}
	page, _ := s.formDialog(msg.Width, msg.Height)
	left, top, right, bottom := dialog.New(page.content, msg.Width, msg.Height).Clipped().Bounds()
	if msg.X < left || msg.X >= right || msg.Y < top || msg.Y >= bottom {
		return s.closeForm(), nil
	}
	for _, hit := range page.actions {
		if msg.X >= hit.x && msg.X < hit.x+hit.width && msg.Y >= hit.y && msg.Y < hit.y+hit.height {
			s.fields[s.focus].Blur()
			s.focus = hit.index
			s.formFollowFocus = true
			return s, s.fields[s.focus].Focus()
		}
	}
	for _, control := range page.controls {
		hit := control.actionHit
		if hit.height > 0 && msg.X >= hit.x && msg.X < hit.x+hit.width && msg.Y == hit.y {
			if control.action == "confirm" {
				return s.submitChoice()
			}
			if control.action == "cancel" {
				return s.closeForm(), nil
			}
			s.choice = hit.index
			return s, nil
		}
	}
	return s, nil
}

func (s State) scrollForm(msg messages.PageMouseScrollMsg) (State, tea.Cmd) {
	page, _ := s.formDialog(msg.Width, msg.Height)
	left, top, right, bottom := dialog.New(page.content, msg.Width, msg.Height).Clipped().Bounds()
	if msg.X < left || msg.X >= right || msg.Y < top || msg.Y >= bottom {
		return s, nil
	}
	delta := 1
	if msg.Up {
		delta = -1
	}
	s.dialogScroll = max(0, min(page.maxScroll, page.scrollOffset+delta))
	s.formFollowFocus = false
	return s, nil
}
