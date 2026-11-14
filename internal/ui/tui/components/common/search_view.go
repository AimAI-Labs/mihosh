package common

import "github.com/charmbracelet/lipgloss"

func RenderSearchEngineBadge(engine SearchEngine) string {
	switch engine {
	case SearchRegex:
		return lipgloss.NewStyle().Foreground(TokyoPurple()).Render(" [RE]")
	case SearchFuzzy:
		return lipgloss.NewStyle().Foreground(TokyoYellow()).Render(" [Fuzzy]")
	default:
		return ""
	}
}
