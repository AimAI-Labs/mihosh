package common

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

func resolve(c lipgloss.Color) lipgloss.TerminalColor { return theme.Resolve(c) }

func TabActiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).
		Background(resolve(Secondary())).Foreground(resolve(Bright())).
		Padding(0, 1)
}

func TabInactiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(resolve(Highlight())).Foreground(resolve(Muted())).
		Padding(0, 1)
}

func BoldStyle() lipgloss.Style { return lipgloss.NewStyle().Bold(true) }
func ActiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(resolve(Active())).Bold(true)
}
func InactiveStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(resolve(Primary())) }
func DimStyle() lipgloss.Style      { return lipgloss.NewStyle().Foreground(resolve(Dim())) }
func MutedStyle() lipgloss.Style    { return lipgloss.NewStyle().Foreground(resolve(Muted())) }
func HighlightStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(resolve(Highlight())).Foreground(resolve(Bright()))
}
func ErrorStyle() lipgloss.Style   { return lipgloss.NewStyle().Foreground(resolve(Danger())) }
func SuccessStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(resolve(Success())) }
func WarningStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(resolve(Warning())) }
func TableHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(resolve(Muted())).Bold(true)
}
func TableBorderStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(resolve(Border())) }
func SelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(resolve(Active())).Bold(true)
}
func PageHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).
		Foreground(resolve(Warning())).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(resolve(Border()))
}
