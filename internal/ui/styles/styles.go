package styles

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================
//  语义色访问函数——动态读取当前主题，支持热切换
// ============================================================

func Background() lipgloss.Color { return theme.Current().Background }
func Surface() lipgloss.Color    { return theme.Current().Surface }
func Overlay() lipgloss.Color    { return theme.Current().Overlay }
func Primary() lipgloss.Color    { return theme.Current().Primary }
func Secondary() lipgloss.Color  { return theme.Current().Secondary }
func Success() lipgloss.Color    { return theme.Current().Success }
func Warning() lipgloss.Color    { return theme.Current().Warning }
func Danger() lipgloss.Color     { return theme.Current().Danger }
func Border() lipgloss.Color     { return theme.Current().Border }
func Gray() lipgloss.Color       { return theme.Current().Muted }
func Dim() lipgloss.Color        { return theme.Current().Dim }
func Text() lipgloss.Color       { return theme.Current().Foreground }
func Bright() lipgloss.Color     { return theme.Current().Bright }

// ============================================================
//  公共样式预设（函数化，支持主题热切换）
// ============================================================

func StatusStyle() lipgloss.Style  { return lipgloss.NewStyle().Foreground(theme.Resolve(Gray())) }
func ErrorStyle() lipgloss.Style   { return lipgloss.NewStyle().Foreground(theme.Resolve(Danger())) }
func TestingStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Resolve(Warning())) }
func DividerStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Resolve(Border())) }

func TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Resolve(Primary())).Padding(0, 1)
}
func SubtitleStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Resolve(Gray())) }

func SelectedItemStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Resolve(Secondary())).Bold(true)
}
func NormalItemStyle() lipgloss.Style   { return lipgloss.NewStyle().Foreground(theme.Resolve(Text())) }
func DisabledItemStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Resolve(Gray())) }

func TableHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Resolve(Primary()))
}
func TableRowStyle() lipgloss.Style    { return lipgloss.NewStyle().Foreground(theme.Resolve(Text())) }
func TableAltRowStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Resolve(Dim())) }

func InputStyle() lipgloss.Style {
	p := theme.Resolve(Primary())
	return lipgloss.NewStyle().Foreground(p).
		Border(lipgloss.RoundedBorder()).BorderForeground(p).Padding(0, 1)
}
func InputLabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Resolve(Gray()))
}
func FooterStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Resolve(Gray())).Padding(0, 1)
}
