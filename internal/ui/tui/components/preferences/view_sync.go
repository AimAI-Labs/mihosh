package preferences

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type syncDialog struct {
	dialog.Dialog
	innerWidth, bodyTop  int
	menuStart, menuCount int
	hintTop              int
	hints                [][]common.InlineHelpHint
}

func buildSyncModal(state PageState, width, height int) syncDialog {
	m := state.SyncModal
	modalWidth := max(24, min(92, max(width-8, min(width, 40))))
	innerWidth := modalWidth - 4
	d := syncDialog{innerWidth: innerWidth}

	title := i18n.T("settings.sync.modal_title")
	subtitle := common.DimStyle().Render(common.TruncateDisplay(i18n.T("settings.sync.modal_subtitle"), innerWidth))
	separator := common.DimStyle().Render(strings.Repeat("─", innerWidth))
	pgSummary := common.MutedStyle().Render(formatSyncSummaryLine(state.Config, innerWidth))

	enter := common.InlineHelpHint{Key: "Enter", Desc: i18n.T("core.dialog.confirm")}
	back := common.InlineHelpHint{Key: "Esc", Desc: i18n.T("settings.sync.back")}
	hints := []common.InlineHelpHint{enter, back}
	bodyHeight := 2
	switch m.Mode {
	case syncModeMenu:
		bodyHeight = syncMenuItemCount
		enter.Desc = i18n.T("core.inline.run")
		back.Desc = i18n.T("settings.sync.close")
		hints = []common.InlineHelpHint{{Key: "↑↓", Desc: i18n.T("core.inline.select")}, enter, back}
	case syncModePGConfig:
		bodyHeight = 4
		hints[0].Desc = i18n.T("core.inline.save")
	}
	var status string
	if m.StatusText != "" {
		style := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoGreen()))
		if m.StatusErr {
			style = common.ErrorStyle()
		} else if m.Busy {
			style = lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoCyan()))
		}
		status = style.Render(common.TruncateDisplay(m.StatusText, innerWidth))
	}
	if m.Busy {
		hints = []common.InlineHelpHint{{Key: "Esc", Desc: i18n.T("settings.sync.close")}}
	}
	d.hints = common.WrapInlineHints(hints, innerWidth)
	available := height - 2 - len(d.hints)
	if status != "" {
		available--
	}
	var sections []string
	if available > bodyHeight+1 {
		sections = append(sections, subtitle)
	}
	if available > bodyHeight {
		sections = append(sections, pgSummary)
	}
	if available > bodyHeight+2 {
		sections = append(sections, separator)
	}
	d.bodyTop = 1 + len(sections)

	switch m.Mode {
	case syncModeMenu:
		items := []string{
			i18n.T("settings.sync.menu_export"),
			i18n.T("settings.sync.menu_import"),
			i18n.T("settings.sync.menu_push"),
			i18n.T("settings.sync.menu_pull"),
			i18n.T("settings.sync.menu_status"),
			i18n.T("settings.sync.menu_pg_config"),
		}
		selectedStyle := lipgloss.NewStyle().
			Background(theme.Resolve(common.TokyoSelected())).
			Foreground(theme.Resolve(common.TokyoCyan())).
			Bold(true)
		normalStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoForeground()))

		d.menuCount = min(len(items), max(1, available-len(sections)))
		d.menuStart = max(0, m.Cursor-d.menuCount+1)
		for i := d.menuStart; i < d.menuStart+d.menuCount; i++ {
			prefix := "  "
			if i == m.Cursor {
				prefix = "▸ "
			}
			line := common.TruncateDisplay(fmt.Sprintf("%s%d. %s", prefix, i+1, items[i]), innerWidth)
			if i == m.Cursor {
				sections = append(sections, selectedStyle.Render(common.PadString(line, innerWidth)))
			} else {
				sections = append(sections, normalStyle.Render(line))
			}
		}

	case syncModeExportPath, syncModeImportPath:
		labelKey := "settings.sync.export_path_label"
		if m.Mode == syncModeImportPath {
			labelKey = "settings.sync.import_path_label"
		}
		label := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoCyan())).Bold(true).Render(i18n.T(labelKey))
		inputBox := renderModalInputLine(m.InputValue, m.InputCursor, innerWidth, true)
		sections = append(sections, label, inputBox)

	case syncModePGConfig:
		dsnLabel := i18n.T("settings.sync.dsn_label")
		keyLabel := i18n.T("settings.sync.key_label")
		activeLabelStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoCyan())).Bold(true)
		mutedLabelStyle := common.MutedStyle()

		var l0, l1 string
		if m.PGField == 0 {
			l0 = activeLabelStyle.Render("▸ " + dsnLabel)
			l1 = mutedLabelStyle.Render("  " + keyLabel)
		} else {
			l0 = mutedLabelStyle.Render("  " + dsnLabel)
			l1 = activeLabelStyle.Render("▸ " + keyLabel)
		}
		f0 := renderModalInputLine(m.DSNValue, m.FormCursor, innerWidth, m.PGField == 0)
		f1 := renderModalInputLine(m.KeyValue, m.FormCursor, innerWidth, m.PGField == 1)
		sections = append(sections, l0, f0, l1, f1)
	}

	if status != "" {
		sections = append(sections, status)
	}
	d.hintTop = 1 + len(sections)
	for _, row := range d.hints {
		sections = append(sections, common.FormatInlineHintRow(row))
	}

	content := common.RenderBorderedPanel(title, strings.Join(sections, "\n"), modalWidth, common.TokyoBlue(), common.TokyoForeground())
	d.Dialog = dialog.New(content, width, height).Clipped()
	return d
}

func renderModalInputLine(val string, cursor, width int, focused bool) string {
	display := val
	visibleWidth := modalInputWidth(width)
	if focused {
		runes := []rune(val)
		c := max(0, min(cursor, len(runes)))
		start := modalInputStart(runes, c, visibleWidth)
		display = string(runes[start:c]) + "▏" + string(runes[c:])
	}
	display = ansi.Truncate(display, visibleWidth, "")
	style := lipgloss.NewStyle().
		Foreground(theme.Resolve(common.TokyoForeground())).
		Padding(0, 1)
	if focused {
		style = style.Background(theme.Resolve(common.TokyoSelected()))
	}
	return style.Render(common.PadString(display, visibleWidth))
}

func modalInputWidth(width int) int { return max(1, width-2) }

func modalInputStart(value []rune, cursor, width int) int {
	start, space := cursor, width-1
	for start > 0 {
		space -= common.DisplayWidth(string(value[start-1]))
		if space < 0 {
			break
		}
		start--
	}
	return start
}
