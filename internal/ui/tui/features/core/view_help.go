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
	if s.tab == 4 && !s.Editing && !s.Busy {
		return s.editor.InlineHelp(width)
	}
	hints := []common.InlineHelpHint{
		{Key: "←→", Desc: i18n.T("core.inline.sections")},
		{Key: "↑↓", Desc: i18n.T("core.inline.select")},
		{Key: "Enter", Desc: i18n.T("core.inline.run")},
	}
	if s.tab == 1 {
		hints = append([]common.InlineHelpHint{{Key: "Enter", Desc: i18n.T("core.inline.download")}}, hints[:2]...)
		hints = append(hints, common.InlineHelpHint{Key: "a", Desc: i18n.T("core.inline.activate")})
	} else if s.tab == 3 {
		hints[2].Desc = i18n.T("core.geo.update")
		hints = append(hints, common.InlineHelpHint{Key: "z", Desc: i18n.T("core.action.restart")})
	}
	if s.Editing {
		hints = []common.InlineHelpHint{
			{Key: "Tab", Desc: i18n.T("core.inline.field")},
			{Key: "Enter", Desc: i18n.T("core.inline.save")},
			{Key: "Esc", Desc: i18n.T("core.inline.cancel")},
		}
		switch s.form {
		case "service":
			hints[0] = common.InlineHelpHint{Key: "↑↓", Desc: i18n.T("core.inline.select")}
			hints[1].Desc = i18n.T("core.inline.run")
		case "configure", "recover", "geo-update":
			hints = hints[1:]
			hints[0].Desc = i18n.T("core.inline.run")
		}
	}
	help := common.InlineHelpHint{Key: "?", Desc: i18n.T("core.inline.help")}
	if s.Editing {
		help = common.InlineHelpHint{Key: "Esc", Desc: i18n.T("core.inline.cancel")}
		hints = hints[:max(0, len(hints)-1)]
		if s.Busy && s.form != "network" {
			hints = []common.InlineHelpHint{{Key: "…", Desc: i18n.T("core.inline.wait")}}
		}
	} else if s.Busy {
		hints = []common.InlineHelpHint{{Key: "←→", Desc: i18n.T("core.inline.sections")}, {Key: "↑↓", Desc: i18n.T("core.inline.select")}}
		help = s.cancelHint()
	} else if s.task != "" || s.Err != nil {
		hints = []common.InlineHelpHint{{Key: "←→", Desc: i18n.T("core.inline.sections")}, {Key: "↑↓", Desc: i18n.T("core.inline.select")}}
		help = common.InlineHelpHint{Key: "Enter/Esc", Desc: i18n.T("core.inline.dismiss")}
	}
	for len(hints) > 0 && lipgloss.Width(common.FormatInlineHintRow(append(hints, help))) > width-1 {
		hints = hints[:len(hints)-1]
	}
	return ansi.Truncate(common.FormatInlineHintRow(append(hints, help)), max(1, width-1), "")
}
