package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func downloadTask(action string) bool {
	return action == "install" || action == "upgrade" || action == "setup"
}

func (s State) progressText(width int) string {
	progress := i18n.T("core.action."+s.task) + " · " + i18n.T("core.busy")
	if s.Progress.Route != "" {
		route := "direct"
		if strings.HasPrefix(s.Progress.Route, "proxy:") {
			route = "proxy"
		} else if strings.HasPrefix(s.Progress.Route, "accelerator:") {
			route = "accelerator"
		}
		progress += "\n" + i18n.T("core.route."+route)
	}
	if s.Progress.Downloaded > 0 {
		progress += fmt.Sprintf(" %.1f MiB", float64(s.Progress.Downloaded)/(1<<20))
		if s.Progress.Total > 0 {
			progress += fmt.Sprintf(" / %.1f MiB", float64(s.Progress.Total)/(1<<20))
		}
	}
	if s.Progress.BytesPerSecond > 0 {
		progress += fmt.Sprintf(" · %.1f KiB/s", float64(s.Progress.BytesPerSecond)/1024)
	}
	if s.Progress.Total > 0 {
		fraction := min(1.0, max(0.0, float64(s.Progress.Downloaded)/float64(s.Progress.Total)))
		cells := max(1, min(30, width-12))
		filled := int(fraction * float64(cells))
		progress += "\n" + strings.Repeat("━", filled) + strings.Repeat("─", cells-filled) + fmt.Sprintf(" %.0f%%", fraction*100)
	}
	return progress
}

func (s State) overlayTask(base string, width, height int) string {
	t := theme.Current()
	popupWidth := min(width, min(76, max(24, width*2/3)))
	body := s.progressText(popupWidth)
	color := t.Primary
	hint := i18n.T("core.cancel")
	if !s.Busy {
		body = i18n.T("core.task.complete")
		color = t.Success
		if errors.Is(s.Err, context.Canceled) {
			body, color = i18n.T("core.task.cancelled"), t.Warning
		} else if s.Err != nil {
			body, color = s.Err.Error(), t.Danger
		}
		hint = i18n.T("core.task.close")
	}
	lines := strings.Split(wrap(body, max(1, popupWidth-4)), "\n")
	budget := max(1, height-4)
	if len(lines) > budget {
		lines = lines[:budget]
		lines[budget-1] = ansi.Truncate(lines[budget-1], max(1, popupWidth-5), "") + "…"
	}
	body = strings.Join(lines, "\n") + "\n" + ansi.Truncate(hint, max(1, popupWidth-4), "…")
	popup := common.RenderBorderedPanel(i18n.T("core.task"), body, popupWidth, color, color)
	popupLines := strings.Split(popup, "\n")
	popupHeight := min(height, len(popupLines))
	x, y := max(0, (width-popupWidth)/2), max(0, (height-popupHeight)/2)
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	for i := range baseLines {
		baseLines[i] = lipgloss.NewStyle().Faint(true).Render(baseLines[i])
	}
	for i := 0; i < popupHeight; i++ {
		left := common.PadString(ansi.Cut(baseLines[y+i], 0, x), x)
		right := ansi.Cut(baseLines[y+i], x+popupWidth, width)
		baseLines[y+i] = left + ansi.Truncate(popupLines[i], popupWidth, "") + right
	}
	return strings.Join(baseLines[:height], "\n")
}
