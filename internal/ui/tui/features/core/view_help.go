package core

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"runtime"
)

func (s State) ReleaseHelpDetails() (platform, asset string) {
	platform = runtime.GOOS + "/" + runtime.GOARCH
	if r, ok := s.selectedRelease(); ok && len(r.Assets) > 0 {
		asset = r.Assets[0].Name
	}
	return platform, asset
}

func (s State) ReleaseHelpNotes() []string {
	var notes []string
	if r, ok := s.selectedRelease(); ok {
		notes = append(notes, r.Version()+" · "+s.releaseBadge(r), releaseInfo(r))
	}
	return append(notes, s.releaseCaption())
}

func (s State) inlineHelp(width int) string {
	hints := []common.InlineHelpHint{
		{Key: "←→", Desc: i18n.T("core.inline.sections")},
		{Key: "↑↓", Desc: i18n.T("core.inline.select")},
		{Key: "Enter", Desc: i18n.T("core.inline.run")},
	}
	if s.tab == 1 {
		hints[2].Desc = i18n.T("core.inline.download")
		hints = append(hints, common.InlineHelpHint{Key: "a", Desc: i18n.T("core.inline.activate")})
	}
	if s.Editing {
		hints = []common.InlineHelpHint{
			{Key: "Tab", Desc: i18n.T("core.inline.field")},
			{Key: "Enter", Desc: i18n.T("core.inline.save")},
			{Key: "Esc", Desc: i18n.T("core.inline.cancel")},
		}
		if s.form == "service" {
			hints[0] = common.InlineHelpHint{Key: "↑↓", Desc: i18n.T("core.inline.select")}
			hints[1].Desc = i18n.T("core.inline.run")
		} else if s.form == "system-proxy" || s.form == "tun" {
			hints[0] = common.InlineHelpHint{Key: "←→/Tab", Desc: i18n.T("core.inline.select")}
		} else if s.form == "configure" || s.form == "recover" {
			hints = hints[1:]
			hints[0].Desc = i18n.T("core.inline.run")
		}
	}
	help := common.InlineHelpHint{Key: "?", Desc: i18n.T("core.inline.help")}
	if s.Editing {
		help = common.InlineHelpHint{Key: "Esc", Desc: i18n.T("core.inline.cancel")}
		hints = hints[:max(0, len(hints)-1)]
	} else if s.Busy {
		hints = hints[:2]
		help = common.InlineHelpHint{Key: "Esc", Desc: i18n.T("core.inline.cancel")}
	} else if s.task != "" || s.Err != nil {
		hints = hints[:2]
		help = common.InlineHelpHint{Key: "Enter/Esc", Desc: i18n.T("core.inline.dismiss")}
	}
	for len(hints) > 0 && lipgloss.Width(common.FormatInlineHintRow(append(hints, help))) > width-1 {
		hints = hints[:len(hints)-1]
	}
	return ansi.Truncate(common.FormatInlineHintRow(append(hints, help)), max(1, width-1), "")
}
