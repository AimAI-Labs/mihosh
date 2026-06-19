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

func StatusStyle() lipgloss.Style  { return lipgloss.NewStyle().Foreground(Gray()) }
func ErrorStyle() lipgloss.Style   { return lipgloss.NewStyle().Foreground(Danger()) }
func TestingStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(Warning()) }
func DividerStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(Border()) }

func TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(Primary()).Padding(0, 1)
}
func SubtitleStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(Gray()) }

func SelectedItemStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(Secondary()).Bold(true) }
func NormalItemStyle() lipgloss.Style   { return lipgloss.NewStyle().Foreground(Text()) }
func DisabledItemStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(Gray()) }

func TableHeaderStyle() lipgloss.Style { return lipgloss.NewStyle().Bold(true).Foreground(Primary()) }
func TableRowStyle() lipgloss.Style    { return lipgloss.NewStyle().Foreground(Text()) }
func TableAltRowStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(Dim()) }

func InputStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Primary()).
		Border(lipgloss.RoundedBorder()).BorderForeground(Primary()).Padding(0, 1)
}
func InputLabelStyle() lipgloss.Style { return lipgloss.NewStyle().Bold(true).Foreground(Gray()) }
func FooterStyle() lipgloss.Style     { return lipgloss.NewStyle().Foreground(Gray()).Padding(0, 1) }
