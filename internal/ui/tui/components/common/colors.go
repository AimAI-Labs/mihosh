package common

import (
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// 颜色访问函数——每次调用读取当前主题，支持热切换
func Background() lipgloss.Color { return theme.Current().Background }
func Surface() lipgloss.Color    { return theme.Current().Surface }
func Overlay() lipgloss.Color    { return theme.Current().Overlay }
func Primary() lipgloss.Color   { return theme.Current().Primary }
func Secondary() lipgloss.Color { return theme.Current().Secondary }
func Success() lipgloss.Color   { return theme.Current().Success }
func Warning() lipgloss.Color   { return theme.Current().Warning }
func Danger() lipgloss.Color    { return theme.Current().Danger }
func Info() lipgloss.Color      { return theme.Current().Info }
func Muted() lipgloss.Color     { return theme.Current().Muted }
func Dim() lipgloss.Color       { return theme.Current().Dim }
func Highlight() lipgloss.Color { return theme.Current().Selected }
func Selected() lipgloss.Color  { return theme.Current().Selected }
func Bright() lipgloss.Color    { return theme.Current().Bright }
func Active() lipgloss.Color    { return theme.Current().Active }
func Orange() lipgloss.Color    { return theme.Current().Orange }
func Purple() lipgloss.Color    { return theme.Current().Purple }
func Gray() lipgloss.Color      { return theme.Current().Muted } // 别名
func Border() lipgloss.Color    { return theme.Current().Border }

// Tokyo 系列别名（减少调用点改动量）
func TokyoForeground() lipgloss.Color { return theme.Current().Foreground }
func TokyoMuted() lipgloss.Color      { return theme.Current().Muted }
func TokyoBlue() lipgloss.Color       { return theme.Current().Primary }
func TokyoCyan() lipgloss.Color       { return theme.Current().Info }
func TokyoGreen() lipgloss.Color      { return theme.Current().Success }
func TokyoRed() lipgloss.Color        { return theme.Current().Danger }
func TokyoYellow() lipgloss.Color     { return theme.Current().Warning }
func TokyoPurple() lipgloss.Color     { return theme.Current().Secondary }
func TokyoPanel() lipgloss.Color      { return theme.Current().Background }
func TokyoSelected() lipgloss.Color   { return theme.Current().Selected }
