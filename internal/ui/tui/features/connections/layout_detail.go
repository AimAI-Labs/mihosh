package connections

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/charmbracelet/lipgloss"
)

type connectionDetailLayout struct {
	content string
	detail  components.ConnectionDetailLayout
	detailY int
}

func layoutConnectionDetail(state PageState) connectionDetailLayout {
	modeSwitch := RenderConnModeSwitchComponent(state.ViewMode, state.Width, state.InlineMode, state.Paused, state.PausedUpdates)
	layout := connectionDetailLayout{detailY: lipgloss.Height(modeSwitch) + 1}
	// Leave a row beneath the panels for the inline help.
	height := max(5, state.Height-layout.detailY-1)
	layout.detail = components.LayoutConnectionDetailImmersive(
		state.SelectedConnection, state.IPInfo, state.Width, height,
		state.DetailLeftScroll, state.DetailRightScroll, state.DetailFocusPanel,
	)
	layout.content = lipgloss.JoinVertical(lipgloss.Left, modeSwitch, "", layout.detail.Content, "")
	return layout
}
