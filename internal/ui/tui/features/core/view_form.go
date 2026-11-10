package core

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
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
	if s.form == "system-proxy" || s.form == "tun" {
		x := 2
		var options []string
		for i, key := range []string{"core.disabled", "core.enabled"} {
			label := " " + i18n.T(key) + " "
			style := lipgloss.NewStyle().Foreground(t.Muted)
			if i == s.choice {
				style = style.Foreground(t.Primary).Background(t.Selected).Bold(true)
			}
			controls = append(controls, releaseControl{actionHit: actionHit{x: x, y: len(rows) + 1, width: lipgloss.Width(label), height: 1, index: i}, action: "choice"})
			options = append(options, style.Render(label))
			x += lipgloss.Width(label) + 1
		}
		rows = append(rows, strings.Join(options, "│"))
	} else {
		label := i18n.T("core.dialog.confirm")
		controls = append(controls, releaseControl{actionHit: actionHit{x: 2, y: len(rows) + 1, width: lipgloss.Width(label), height: 1}, action: "confirm"})
		rows = append(rows, lipgloss.NewStyle().Foreground(t.Primary).Background(t.Selected).Bold(true).Render(label))
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
	case "system-proxy", "tun", "configure", "recover":
		page = s.choiceLayout(popupWidth)
		page.focusY = page.controls[0].y
	default:
		page = s.formLayout(popupWidth)
	}
	lines := strings.Split(page.content, "\n")
	budget := max(1, height-5)
	body := lines[1 : len(lines)-1]
	offset := max(0, min(page.focusY-budget, len(body)-budget))
	body = body[offset:min(len(body), offset+budget)]
	// Retain the frame and a fixed hint when scrolling a tall form or menu.
	page.content = strings.Join(append(append([]string{lines[0]}, body...),
		common.TokyoBlueStyle().Render("│ ")+common.PadString(ansi.Truncate(s.inlineHelp(popupWidth-3), popupWidth-4, ""), popupWidth-4)+common.TokyoBlueStyle().Render(" │"),
		lines[len(lines)-1]), "\n")
	x, y := max(0, (width-popupWidth)/2), max(0, (height-lipgloss.Height(page.content))/2)
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
	page, popupWidth := s.formDialog(width, height)
	return overlayPopup(base, page.content, width, height, popupWidth)
}

func (s State) clickForm(msg messages.PageMouseClickMsg) (State, tea.Cmd) {
	if msg.X < 0 || msg.X >= msg.Width || msg.Y < 0 || msg.Y >= msg.Height {
		return s, nil
	}
	page, _ := s.formDialog(msg.Width, msg.Height)
	for _, hit := range page.actions {
		if msg.X >= hit.x && msg.X < hit.x+hit.width && msg.Y >= hit.y && msg.Y < hit.y+hit.height {
			if s.form == "service" {
				s.selected = hit.index
				return s.submitService()
			}
			s.fields[s.focus].Blur()
			s.focus = hit.index
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
