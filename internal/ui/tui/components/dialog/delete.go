package dialog

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// DeleteConfirmation renders a destructive action with its target and warning.
func DeleteConfirmation(width, height int, title, prompt, warning string, target ...string) Dialog {
	panelWidth := max(24, min(54, width-4))
	innerWidth := panelWidth - 4
	rows := make([]string, 0, len(target)+4)
	for _, line := range target {
		rows = append(rows, ansi.Truncate(line, innerWidth, "…"))
	}
	rows = append(rows, common.TokyoMutedStyle().Render(strings.Repeat("─", innerWidth)),
		common.TokyoMutedStyle().Render(ansi.Wrap(prompt, innerWidth, "")))
	if warning != "" {
		rows = append(rows, lipgloss.NewStyle().Foreground(common.Warning()).Render(ansi.Wrap(warning, innerWidth, "")))
	}
	panel := common.RenderBorderedPanel(title, strings.Join(rows, "\n"), panelWidth, common.Danger(), common.Danger())
	return New(panel, width, height).Clipped()
}
