package preferences

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

func buildSyncModal(state PageState, width, height int) string {
	m := state.SyncModal
	modalWidth := min(width, max(40, min(92, width-8)))
	innerWidth := max(24, modalWidth-4)

	title := i18n.T("settings.sync.modal_title")
	subtitle := common.DimStyle().Render(common.TruncateDisplay(i18n.T("settings.sync.modal_subtitle"), innerWidth))
	separator := common.DimStyle().Render(strings.Repeat("─", innerWidth))
	pgSummary := common.MutedStyle().Render(formatSyncSummaryLine(state.Config, innerWidth))

	var body string
	var footer string

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

		rows := make([]string, len(items))
		for i, label := range items {
			prefix := "  "
			if i == m.Cursor {
				prefix = "▸ "
			}
			line := common.TruncateDisplay(fmt.Sprintf("%s%d. %s", prefix, i+1, label), innerWidth)
			if i == m.Cursor {
				rows[i] = selectedStyle.Render(common.PadString(line, innerWidth))
			} else {
				rows[i] = normalStyle.Render(line)
			}
		}
		body = strings.Join(rows, "\n")
		footer = i18n.T("settings.sync.footer_menu")

	case syncModeExportPath, syncModeImportPath:
		labelKey := "settings.sync.export_path_label"
		if m.Mode == syncModeImportPath {
			labelKey = "settings.sync.import_path_label"
		}
		label := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoCyan())).Bold(true).Render(i18n.T(labelKey))
		inputBox := renderModalInputLine(m.InputValue, m.InputCursor, innerWidth, true)
		body = lipgloss.JoinVertical(lipgloss.Left, label, inputBox)
		footer = i18n.T("settings.sync.footer_input")

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
		body = lipgloss.JoinVertical(lipgloss.Left, l0, f0, "", l1, f1)
		footer = i18n.T("settings.sync.footer_pg_config")
	}

	sections := []string{subtitle, pgSummary, separator, body}
	if m.StatusText != "" {
		stStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoGreen()))
		if m.StatusErr {
			stStyle = common.ErrorStyle()
		} else if m.Busy {
			stStyle = lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoCyan()))
		}
		sections = append(sections, "", stStyle.Render(common.TruncateDisplay(m.StatusText, innerWidth)))
	}
	sections = append(sections, separator, common.DimStyle().Render(common.TruncateDisplay(footer, innerWidth)))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return common.RenderBorderedPanel(title, content, modalWidth, common.TokyoBlue(), common.TokyoForeground())
}

func renderModalInputLine(val string, cursor, width int, focused bool) string {
	display := val
	if focused {
		runes := []rune(val)
		c := max(0, min(cursor, len(runes)))
		display = string(runes[:c]) + "▏" + string(runes[c:])
	}
	if display == "" {
		display = " "
	}
	display = common.TruncateDisplay(display, max(8, width-2))
	style := lipgloss.NewStyle().
		Foreground(theme.Resolve(common.TokyoForeground())).
		Padding(0, 1)
	if focused {
		style = style.Background(theme.Resolve(common.TokyoSelected()))
	}
	return style.Render(common.PadString(display, max(8, width-2)))
}
