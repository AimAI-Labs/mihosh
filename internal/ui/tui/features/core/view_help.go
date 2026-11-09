package core

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

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
		}
	} else if s.Busy {
		hints = []common.InlineHelpHint{{Key: "Esc", Desc: i18n.T("core.inline.cancel")}}
	}
	help := common.InlineHelpHint{Key: "?", Desc: i18n.T("core.inline.help")}
	if s.Editing || s.taskDialog {
		help = common.InlineHelpHint{Key: "Esc", Desc: i18n.T("core.inline.cancel")}
		hints = hints[:max(0, len(hints)-1)]
	}
	for len(hints) > 0 && lipgloss.Width(common.FormatInlineHintRow(append(hints, help))) > width-1 {
		hints = hints[:len(hints)-1]
	}
	return ansi.Truncate(common.FormatInlineHintRow(append(hints, help)), max(1, width-1), "")
}
