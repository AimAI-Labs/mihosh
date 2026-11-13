package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Reserve the same space while idle so tasks never move the viewport.
const footerHeight = 4

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

func (s State) footer(width, height int) string {
	t := theme.Current()
	width = max(1, width-1)
	body := ""
	color := t.Primary
	showTask := s.task != "" && s.task != "releases"
	if s.Busy && showTask {
		body = i18n.T("core.task") + " · " + s.progressText(width)
	} else if !s.Busy && (showTask || s.Err != nil) {
		body = i18n.T("core.task.complete")
		color = t.Success
		if errors.Is(s.Err, context.Canceled) {
			body, color = i18n.T("core.task.cancelled"), t.Warning
		} else if s.Err != nil {
			body, color = s.Err.Error(), t.Danger
		}
		if showTask {
			body = i18n.T("core.action."+s.task) + " · " + body
		}
	}
	if !s.Busy && s.tab == 1 && s.catalog.failed {
		caption := i18n.T("core.catalog.load_failed")
		if body == "" {
			body, color = caption, t.Warning
		} else if !strings.Contains(body, caption) {
			body += "\n" + caption
		}
	}
	budget := min(footerHeight-1, max(0, height-1))
	lines := strings.Split(wrap(body, width), "\n")
	if len(lines) > budget && budget > 0 {
		lines = lines[:budget]
		lines[budget-1] = ansi.Truncate(lines[budget-1], max(0, width-1), "") + "…"
	}
	lines = append(make([]string, max(0, budget-len(lines))), lines...)
	lines = lines[:budget]
	for i := range lines {
		lines[i] = lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Foreground(color).Render(ansi.Truncate(lines[i], width, "…"))
	}
	help := lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(s.inlineHelp(width + 1))
	return strings.Join(append(lines, help), "\n")
}
