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
	rows := strings.Split(wrap(i18n.T("core.desc."+s.form), width-4), "\n")
	rows = append(rows, "")
	var controls []releaseControl
	if s.form == "system-proxy" || s.form == "tun" || s.form == "shell-proxy" {
		x := 2
		var options []string
		for i, key := range []string{"core.disabled", "core.enabled"} {
			label := " " + i18n.T(key) + " "
			style := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted))
			if i == s.choice {
				style = style.Foreground(theme.Resolve(t.Primary)).Background(theme.Resolve(t.Selected)).Bold(true)
			}
			controls = append(controls, releaseControl{actionHit: actionHit{x: x, y: len(rows) + 1, width: lipgloss.Width(label), height: 1, index: i}, action: "choice"})
			options = append(options, style.Render(label))
			x += lipgloss.Width(label) + 1
		}
		rows = append(rows, strings.Join(options, "│"))
	} else {
		label := i18n.T("core.dialog.confirm")
		controls = append(controls, releaseControl{actionHit: actionHit{x: 2, y: len(rows) + 1, width: lipgloss.Width(label), height: 1}, action: "confirm"})
		rows = append(rows, lipgloss.NewStyle().Foreground(theme.Resolve(t.Primary)).Background(theme.Resolve(t.Selected)).Bold(true).Render(label))
	}
	if s.formErr != nil {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(theme.Resolve(t.Danger)).Render(wrap(s.formErr.Error(), width-4)))
	}
	return pageLayout{content: panel(i18n.T("core.action."+s.form), strings.Join(rows, "\n"), width), controls: controls}
}

// The renderer and mouse handler use the same clipped, centered dialog bounds.
func (s State) formDialog(width, height int) (pageLayout, int) {
	popupWidth := max(24, min(76, width*2/3))
	popupWidth = max(24, min(width, popupWidth))
	var page pageLayout
	switch s.form {
	case "service":
		page.content, page.actions = s.actionPanel(popupWidth, true)
		page.focusY = page.actions[s.selected].y
	case "system-proxy", "tun", "shell-proxy", "configure", "recover":
		page = s.choiceLayout(popupWidth)
		page.focusY = page.controls[0].y
	default:
		page = s.formLayout(popupWidth)
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
			if s.form == "service" {
				s.selected = hit.index
				return s.submitService()
			}
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
	if s.form == "service" {
		s.selected = max(0, min(len(serviceActions)-1, s.selected+delta))
		s.dialogScroll = page.scrollOffset
		s.formFollowFocus = true
		return s, nil
	}
	s.dialogScroll = max(0, min(page.maxScroll, page.scrollOffset+delta))
	s.formFollowFocus = false
	return s, nil
}
