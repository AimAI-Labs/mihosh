package rules

import (
	"fmt"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

func deleteConfirmation(state PageState, width, height int) dialog.Dialog {
	target := state.DeleteTarget
	index := lipgloss.NewStyle().Foreground(common.Success()).Render(fmt.Sprintf("%d.", state.DeleteTargetIndex+1))
	ruleType := lipgloss.NewStyle().Foreground(getAdjustedRuleTypeColor(target.Type, nil)).Bold(true).Render(target.Type)
	payload := lipgloss.NewStyle().Foreground(common.Secondary()).Render(target.Payload)
	proxy := lipgloss.NewStyle().Foreground(common.Bright()).Render("→ " + target.Proxy)
	if target.NoResolve {
		ruleType += lipgloss.NewStyle().Foreground(common.Warning()).Render(" [no-resolve]")
	}
	return dialog.DeleteConfirmation(width, height, i18n.T("rules.delete_title"), i18n.T("rules.delete_confirm_hint"),
		"", index+" "+ruleType+" "+payload, proxy).Below(rulesHeaderHeight + 1)
}

func renderDeleteConfirmOverlay(background string, state PageState, width, height int) string {
	return deleteConfirmation(state, width, height).Overlay(background, true)
}

func ResolveDeleteConfirmBounds(state PageState, width, height int) (left, top, right, bottom int) {
	return deleteConfirmation(state, width, height).Bounds()
}
