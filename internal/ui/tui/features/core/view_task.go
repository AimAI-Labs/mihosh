package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Reserve the same space while idle so tasks never move the viewport.
const footerHeight = 4

func (s State) downloading() bool {
	return s.Busy && (s.task == "install" || s.task == "upgrade" || s.task == "setup" || s.task == "geo-update")
}

func (s State) cancelHint() common.InlineHelpHint {
	desc := "core.inline.cancel"
	if !s.Editing && s.downloading() {
		desc = "core.inline.cancel_download"
	}
	return common.InlineHelpHint{Key: "Esc", Desc: i18n.T(desc)}
}

func (s State) cancelDownloadAt(x, y, width, height int) bool {
	if !s.downloading() || s.cancel == nil || y != height-1 || x < 0 || x >= width || width <= 0 || height <= 0 {
		return false
	}
	footer := strings.Split(ansi.Strip(s.footer(width, height, true)), "\n")
	row := footer[len(footer)-1]
	hint := ansi.Truncate(ansi.Strip(common.FormatInlineHintRow([]common.InlineHelpHint{s.cancelHint()})), max(1, width-1), "")
	index := strings.LastIndex(row, hint)
	return index >= 0 && x >= lipgloss.Width(row[:index]) && x < lipgloss.Width(row[:index])+lipgloss.Width(hint)
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

func coreActionLabel(action string) string {
	switch {
	case strings.HasPrefix(action, "service-"):
		action = "service"
	case action == "tun-offline":
		action = "tun"
	}
	return i18n.T("core.action." + action)
}

// OperationNotice converts a finished CoreOperationMsg into a global notice.
func (s State) OperationNotice(msg messages.CoreOperationMsg) (messages.NoticeMsg, bool) {
	if msg.Action == "releases" {
		if msg.Err != nil {
			caption := i18n.T("core.catalog.load_failed")
			return messages.NoticeMsg{Text: caption, Detail: caption + ": " + msg.Err.Error(), Level: messages.NoticeWarning}, true
		}
		return messages.NoticeMsg{}, false
	}
	label := coreActionLabel(msg.Action)
	if errors.Is(msg.Err, context.Canceled) {
		return messages.NoticeMsg{Text: label + " · " + i18n.T("core.task.cancelled"), Level: messages.NoticeWarning}, true
	}
	if msg.Err != nil {
		detail := label + " · " + msg.Err.Error()
		return messages.NoticeMsg{Text: detail, Detail: detail, Level: messages.NoticeError}, true
	}
	text := label + " · " + i18n.T("core.task.complete")
	detail := ""
	if msg.Action == "shell-proxy" && msg.ShellPath != "" {
		text += " · " + i18n.Tf("core.shell.source_hint", service.ShellProxySourceCommand(msg.ShellPath))
		detail = text
	}
	return messages.NoticeMsg{Text: text, Detail: detail, Level: messages.NoticeSuccess}, true
}

// ClearTaskNotice clears completed task result state once handed off to the global notice bar.
func (s State) ClearTaskNotice() State {
	if !s.Busy {
		s.task = ""
		s.Err = nil
		s.Progress = service.Progress{}
	}
	return s
}

func (s State) InlineHelpHints() []common.InlineHelpHint {
	if s.Editing {
		return nil
	}
	return s.inlineHelpHints()
}

func (s State) InlineHelpFirstWidth(width int) int {
	return max(1, width-1)
}

func (s State) footer(width, height int, showHelp bool) string {
	rendered, _ := s.footerRender(width, height, showHelp)
	return rendered
}

func (s State) footerRender(width, height int, showHelp bool) (string, int) {
	t := theme.Current()
	width = max(1, width-1)
	body := ""
	color := t.Primary
	showTask := s.task != "" && s.task != "releases"
	if s.Busy && showTask {
		body = i18n.T("core.task") + " · " + s.progressText(width)
	} else if showHelp && !s.Busy && (showTask || s.Err != nil) {
		body = i18n.T("core.task.complete")
		color = t.Success
		if errors.Is(s.Err, context.Canceled) {
			body, color = i18n.T("core.task.cancelled"), t.Warning
		} else if s.Err != nil {
			body, color = s.Err.Error(), t.Danger
		}
		if showTask {
			body = coreActionLabel(s.task) + " · " + body
		}
		if s.task == "shell-proxy" && s.Err == nil && s.shellPath != "" {
			body += "\n" + i18n.Tf("core.shell.source_hint", service.ShellProxySourceCommand(s.shellPath))
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
	lineStyle := lipgloss.NewStyle().Width(width).Foreground(theme.Resolve(color))
	if !showHelp {
		// In Model.View(), completed notices live in the global notice row below
		// pageContent[:footer]. Keep only live progress / catalog lines above the
		// notice row in top-to-bottom order and leave the final row clear for help.
		var upper []string
		if body != "" && budget > 0 {
			wrapped := strings.Split(wrap(body, width), "\n")
			if len(wrapped) > budget {
				wrapped = wrapped[:budget]
				wrapped[budget-1] = ansi.Truncate(wrapped[budget-1], max(0, width-1), "") + "…"
			}
			upper = wrapped
		}
		upper = append(make([]string, max(0, budget-len(upper))), upper...)
		for i := range upper {
			upper[i] = lineStyle.Render(ansi.Truncate(upper[i], width, "…"))
		}
		return strings.Join(append(upper, ""), "\n"), width
	}
	var lines []string
	bottomNotice := ""
	if body != "" {
		wrapped := strings.Split(wrap(body, width), "\n")
		if len(wrapped) == 1 {
			bottomNotice = wrapped[0]
		} else {
			if len(wrapped) > budget+1 {
				wrapped = wrapped[:budget+1]
				wrapped[len(wrapped)-1] = ansi.Truncate(wrapped[len(wrapped)-1], max(0, width-1), "") + "…"
			}
			lines = wrapped[:len(wrapped)-1]
			bottomNotice = wrapped[len(wrapped)-1]
		}
	}
	lines = append(make([]string, max(0, budget-len(lines))), lines...)
	lines = lines[:budget]
	for i := range lines {
		lines[i] = lineStyle.Render(ansi.Truncate(lines[i], width, "…"))
	}
	helpWidth := width
	if bottomNotice != "" {
		control := common.InlineHelpHint{Key: "Enter/Esc", Desc: i18n.T("core.inline.dismiss")}
		if s.Busy || s.Editing {
			control = s.cancelHint()
		}
		helpWidth = min(width, max(width/2, lipgloss.Width(common.FormatInlineHintRow([]common.InlineHelpHint{control}))))
	}
	help := s.inlineHelp(helpWidth + 1)
	leftWidth := max(0, width-lipgloss.Width(help)-2)
	left := lipgloss.NewStyle().Foreground(theme.Resolve(color)).Render(ansi.Truncate(bottomNotice, leftWidth, "…"))
	row := common.PadString(left, max(0, width-lipgloss.Width(help))) + help
	return strings.Join(append(lines, row), "\n"), helpWidth
}
