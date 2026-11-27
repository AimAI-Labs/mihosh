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

func choiceDialogColors(form string) (border, title lipgloss.Color) {
	switch form {
	case "stop":
		return common.Danger(), common.Danger()
	case "restart", "recover":
		return common.Warning(), common.Warning()
	default:
		return common.TokyoBlue(), common.TokyoForeground()
	}
}

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
	innerWidth := max(1, width-4)
	borderColor, titleColor := choiceDialogColors(s.form)
	divider := lipgloss.NewStyle().Foreground(theme.Resolve(t.Border)).Render(strings.Repeat("─", innerWidth))

	var rows []string
	for _, line := range strings.Split(wrap(desc, innerWidth), "\n") {
		rows = append(rows, common.TokyoTextStyle().Render(line))
	}
	rows = append(rows, divider)

	confirmLabel := " " + i18n.T("core.dialog.confirm") + " "
	cancelLabel := " " + i18n.T("core.dialog.cancel") + " "
	confirmW := lipgloss.Width(confirmLabel)
	cancelW := lipgloss.Width(cancelLabel)

	confirmStyle := lipgloss.NewStyle().Foreground(theme.Resolve(t.Primary)).Background(theme.Resolve(t.Selected)).Bold(true)
	if s.Busy {
		confirmStyle = lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted)).Background(theme.Resolve(t.Selected))
	} else {
		switch s.form {
		case "stop":
			confirmStyle = lipgloss.NewStyle().Foreground(theme.Resolve(t.Background)).Background(theme.Resolve(t.Danger)).Bold(true)
		case "restart", "recover":
			confirmStyle = lipgloss.NewStyle().Foreground(theme.Resolve(t.Background)).Background(theme.Resolve(t.Warning)).Bold(true)
		}
	}
	cancelStyle := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted)).Background(theme.Resolve(t.Surface))

	var controls []releaseControl
	if confirmW+2+cancelW <= innerWidth {
		btnY := len(rows) + 1
		controls = []releaseControl{
			{actionHit: actionHit{x: 2, y: btnY, width: confirmW, height: 1}, action: "confirm"},
			{actionHit: actionHit{x: 4 + confirmW, y: btnY, width: cancelW, height: 1}, action: "cancel"},
		}
		rows = append(rows, confirmStyle.Render(confirmLabel)+"  "+cancelStyle.Render(cancelLabel))
	} else {
		confirmClamped := ansi.Truncate(confirmLabel, innerWidth, "")
		cancelClamped := ansi.Truncate(cancelLabel, innerWidth, "")
		controls = []releaseControl{
			{actionHit: actionHit{x: 2, y: len(rows) + 1, width: lipgloss.Width(confirmClamped), height: 1}, action: "confirm"},
			{actionHit: actionHit{x: 2, y: len(rows) + 2, width: lipgloss.Width(cancelClamped), height: 1}, action: "cancel"},
		}
		rows = append(rows, confirmStyle.Render(confirmClamped), cancelStyle.Render(cancelClamped))
	}
	if s.formErr != nil {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(theme.Resolve(t.Danger)).Render(wrap("✗ "+s.formErr.Error(), innerWidth)))
	}
	return pageLayout{content: common.RenderBorderedPanel(title, strings.Join(rows, "\n"), width, borderColor, titleColor), controls: controls}
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

func formDialogPlacement(content string, width, height int) dialog.Dialog {
	areaHeight := max(1, height-1)
	dlgHeight := min(lipgloss.Height(content), areaHeight)
	top := max(0, (areaHeight-dlgHeight)/2)
	return dialog.New(content, width, height).Below(top).Clipped()
}

// The renderer and mouse handler use the same clipped, centered dialog bounds.
func (s State) formDialog(width, height int) (pageLayout, int) {
	maxPopup := 76
	if isChoiceForm(s.form) {
		maxPopup = 58
	}
	popupWidth := max(24, min(maxPopup, width-4))
	if width < popupWidth {
		popupWidth = max(24, width)
	}
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
	page.content = strings.Join(append(append([]string{lines[0]}, body...), lines[len(lines)-1]), "\n")
	x, y, _, _ := formDialogPlacement(page.content, width, height).Bounds()
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
	return formDialogPlacement(page.content, width, height).Overlay(base, true)
}

func (s State) clickForm(msg messages.PageMouseClickMsg) (State, tea.Cmd) {
	if msg.Width <= 0 || msg.Height <= 0 {
		return s, nil
	}
	page, _ := s.formDialog(msg.Width, msg.Height)
	left, top, right, bottom := formDialogPlacement(page.content, msg.Width, msg.Height).Bounds()
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
	left, top, right, bottom := formDialogPlacement(page.content, msg.Width, msg.Height).Bounds()
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
