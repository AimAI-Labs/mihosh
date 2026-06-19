package common

import "github.com/charmbracelet/lipgloss"

func TabActiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).
		Background(Secondary()).Foreground(Bright()).
		Padding(0, 1)
}

func TabInactiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(Highlight()).Foreground(Muted()).
		Padding(0, 1)
}

func BoldStyle() lipgloss.Style      { return lipgloss.NewStyle().Bold(true) }
func ActiveStyle() lipgloss.Style    { return lipgloss.NewStyle().Foreground(Active()).Bold(true) }
func InactiveStyle() lipgloss.Style  { return lipgloss.NewStyle().Foreground(Primary()) }
func DimStyle() lipgloss.Style       { return lipgloss.NewStyle().Foreground(Dim()) }
func MutedStyle() lipgloss.Style     { return lipgloss.NewStyle().Foreground(Muted()) }
func HighlightStyle() lipgloss.Style { return lipgloss.NewStyle().Background(Highlight()).Foreground(Bright()) }
func ErrorStyle() lipgloss.Style     { return lipgloss.NewStyle().Foreground(Danger()) }
func SuccessStyle() lipgloss.Style   { return lipgloss.NewStyle().Foreground(Success()) }
func WarningStyle() lipgloss.Style   { return lipgloss.NewStyle().Foreground(Warning()) }
func TableHeaderStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(Muted()).Bold(true) }
func TableBorderStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(Border()) }
func SelectedStyle() lipgloss.Style    { return lipgloss.NewStyle().Foreground(Active()).Bold(true) }
func PageHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).
		Foreground(Warning()).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(Border())
}
