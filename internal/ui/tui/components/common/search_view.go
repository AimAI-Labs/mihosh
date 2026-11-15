package common

import "github.com/charmbracelet/lipgloss"

func RenderSearchEngineBadge(engine SearchEngine) string {
	switch engine {
	case SearchRegex:
		return lipgloss.NewStyle().Foreground(resolve(TokyoPurple())).Render(" [RE]")
	case SearchFuzzy:
		return lipgloss.NewStyle().Foreground(resolve(TokyoYellow())).Render(" [Fuzzy]")
	default:
		return ""
	}
}
