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

type uninstallDialog struct {
	dialog.Dialog
	innerWidth   int
	toggleRowY   int
	confirmRowY  int
	confirmStart int
	confirmEnd   int
	cancelRowY   int
	cancelStart  int
	cancelEnd    int
	hintTop      int
	hints        [][]common.InlineHelpHint
}

func buildUninstallModal(state PageState, width, height int) uninstallDialog {
	m := state.UninstallModal
	modalWidth := max(24, min(76, max(width-8, min(width, 40))))
	innerWidth := modalWidth - 4
	d := uninstallDialog{
		innerWidth:  innerWidth,
		toggleRowY:  -1,
		confirmRowY: -1,
		cancelRowY:  -1,
		hintTop:     -1,
	}

	title := i18n.T("settings.uninstall.modal_title")
	subtitleStyle := common.DimStyle()
	separator := common.DimStyle().Render(strings.Repeat("─", innerWidth))
	mutedStyle := common.MutedStyle()
	normalStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoForeground()))
	dangerStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.Danger()))
	successStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.Success()))
	warnStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.Warning()))

	var lines []string
	appendWrapped := func(text string, style lipgloss.Style) {
		wrapped := ansi.Wrap(text, innerWidth, "")
		for _, line := range strings.Split(wrapped, "\n") {
			lines = append(lines, style.Render(line))
		}
	}
	appendItem := func(label, value string, valueStyle lipgloss.Style) {
		prefix := mutedStyle.Render(label + ": ")
		prefixW := lipgloss.Width(prefix)
		avail := max(1, innerWidth-prefixW)
		lines = append(lines, prefix+valueStyle.Render(common.TruncateDisplay(value, avail)))
	}

	appendWrapped(i18n.T("settings.uninstall.modal_subtitle"), subtitleStyle)
	lines = append(lines, separator)

	switch {
	case m.Loading:
		appendWrapped(i18n.T("settings.uninstall.loading"), lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoCyan())))
	case m.Plan != nil:
		p := m.Plan
		if p.ExecutablePath != "" {
			binVal := p.ExecutablePath
			binStyle := normalStyle
			if p.BinaryHomebrew {
				binVal += " (" + i18n.T("settings.uninstall.badge_homebrew") + ")"
				binStyle = warnStyle
			} else if !p.BinaryWritable {
				binVal += " (" + i18n.T("settings.uninstall.badge_sudo") + ")"
				binStyle = warnStyle
			}
			appendItem(i18n.T("settings.uninstall.label_binary"), binVal, binStyle)
		}
		if p.ServiceInstalled {
			appendItem(i18n.T("settings.uninstall.label_service"), i18n.Tf("settings.uninstall.value_service_remove", p.ServiceUnit), normalStyle)
		}
		if len(p.ShellFiles) > 0 {
			appendItem(i18n.T("settings.uninstall.label_shell"), i18n.Tf("settings.uninstall.value_shell_remove", strings.Join(p.ShellFiles, ", ")), normalStyle)
		}
		if p.NetworkRecovery {
			appendItem(i18n.T("settings.uninstall.label_network"), i18n.T("settings.uninstall.value_network_recover"), normalStyle)
		}
		switch {
		case !p.ConfigExists:
			appendItem(i18n.T("settings.uninstall.label_config"), i18n.T("settings.uninstall.value_config_missing"), mutedStyle)
		case m.KeepConfig:
			appendItem(i18n.T("settings.uninstall.label_config"), i18n.Tf("settings.uninstall.value_config_keep", p.ConfigDir), successStyle)
		default:
			appendItem(i18n.T("settings.uninstall.label_config"), i18n.Tf("settings.uninstall.value_config_delete", p.ConfigDir), dangerStyle)
		}
	}

	lines = append(lines, separator)

	// Interactive Keep-Config toggle row.
	d.toggleRowY = 1 + len(lines)
	check := "[ ]"
	if m.KeepConfig {
		check = "[✓]"
	}
	toggleText := fmt.Sprintf("%s %s", check, i18n.T("settings.uninstall.toggle_keep_config"))
	if !m.KeepConfig {
		toggleText = fmt.Sprintf("%s %s", check, i18n.T("settings.uninstall.toggle_keep_config_warn"))
	}
	toggleText = common.TruncateDisplay(toggleText, innerWidth)
	toggleStyle := normalStyle
	if !m.KeepConfig {
		toggleStyle = dangerStyle
	}
	if m.Focus == uninstallFocusKeepConfig && !m.Busy {
		toggleStyle = lipgloss.NewStyle().
			Background(theme.Resolve(common.TokyoSelected())).
			Foreground(theme.Resolve(common.TokyoCyan())).
			Bold(true)
		if !m.KeepConfig {
			toggleStyle = toggleStyle.Foreground(theme.Resolve(common.Danger()))
		}
		toggleText = common.PadString(toggleText, innerWidth)
	}
	lines = append(lines, toggleStyle.Render(toggleText))
	lines = append(lines, "")

	// Confirm / Cancel buttons.
	confirmText := " " + i18n.T("settings.uninstall.btn_confirm") + " "
	cancelText := " " + i18n.T("settings.uninstall.btn_cancel") + " "
	confirmW := lipgloss.Width(confirmText)
	cancelW := lipgloss.Width(cancelText)

	confirmStyle := lipgloss.NewStyle().
		Foreground(theme.Resolve(common.Danger())).
		Background(theme.Resolve(common.TokyoSelected())).
		Bold(true)
	if m.Focus == uninstallFocusConfirm && !m.Busy {
		confirmStyle = lipgloss.NewStyle().
			Foreground(theme.Resolve(common.Background())).
			Background(theme.Resolve(common.Danger())).
			Bold(true)
	}
	cancelStyle := lipgloss.NewStyle().
		Foreground(theme.Resolve(common.TokyoMuted())).
		Background(theme.Resolve(common.Background()))
	if m.Focus == uninstallFocusCancel && !m.Busy {
		cancelStyle = lipgloss.NewStyle().
			Foreground(theme.Resolve(common.TokyoCyan())).
			Background(theme.Resolve(common.TokyoSelected())).
			Bold(true)
	}

	if confirmW+2+cancelW <= innerWidth {
		rowY := 1 + len(lines)
		d.confirmRowY, d.confirmStart, d.confirmEnd = rowY, 0, confirmW
		d.cancelRowY, d.cancelStart, d.cancelEnd = rowY, confirmW+2, confirmW+2+cancelW
		lines = append(lines, confirmStyle.Render(confirmText)+"  "+cancelStyle.Render(cancelText))
	} else {
		confirmClamped := common.TruncateDisplay(confirmText, innerWidth)
		cancelClamped := common.TruncateDisplay(cancelText, innerWidth)
		d.confirmRowY, d.confirmStart, d.confirmEnd = 1+len(lines), 0, lipgloss.Width(confirmClamped)
		lines = append(lines, confirmStyle.Render(confirmClamped))
		d.cancelRowY, d.cancelStart, d.cancelEnd = 1+len(lines), 0, lipgloss.Width(cancelClamped)
		lines = append(lines, cancelStyle.Render(cancelClamped))
	}

	if m.Busy {
		lines = append(lines, "")
		appendWrapped(i18n.T("settings.uninstall.running"), lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoCyan())))
	} else if m.ErrText != "" {
		lines = append(lines, "")
		appendWrapped(m.ErrText, common.ErrorStyle())
	}

	if !m.Busy {
		hints := []common.InlineHelpHint{
			{Key: "Space", Desc: i18n.T("settings.uninstall.hint_toggle"), Action: " "},
			{Key: "Tab", Desc: i18n.T("settings.uninstall.hint_switch"), Action: "tab"},
			{Key: "Enter", Desc: i18n.T("settings.uninstall.hint_confirm"), Action: "enter", Tone: common.InlineHelpToneDanger},
			{Key: "Esc", Desc: i18n.T("settings.uninstall.hint_cancel"), Action: "esc"},
		}
		d.hints = common.WrapInlineHints(hints, innerWidth)
		lines = append(lines, "")
		d.hintTop = 1 + len(lines)
		for _, row := range d.hints {
			lines = append(lines, common.FormatInlineHintRow(row))
		}
	}

	content := common.RenderBorderedPanel(title, strings.Join(lines, "\n"), modalWidth, common.Danger(), common.Danger())
	d.Dialog = dialog.New(content, width, height).Clipped()
	return d
}
