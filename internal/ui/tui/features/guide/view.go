package guide

import (
	"fmt"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

// Render 渲染引导弹窗 View
func (s State) Render(width, height int) string {
	if !s.Active {
		return ""
	}

	t := theme.Current()

	// 弹窗尺寸计算
	dialogWidth := width * 3 / 5
	if dialogWidth < 50 {
		dialogWidth = 50
	}
	if dialogWidth > width-4 {
		dialogWidth = width - 4
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		MarginBottom(1)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(t.Muted).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Bold(true)

	valStyle := lipgloss.NewStyle().
		Foreground(t.Foreground)

	errStyle := lipgloss.NewStyle().
		Foreground(t.Danger).
		MarginTop(1).
		MarginBottom(1)

	var infoContent string
	if s.Endpoint != "" {
		infoContent += fmt.Sprintf("%s: %s\n", labelStyle.Render(i18n.T("guide.current_endpoint")), valStyle.Render(s.Endpoint))
	}
	if s.SecretMasked != "" {
		infoContent += fmt.Sprintf("%s: %s\n", labelStyle.Render(i18n.T("guide.current_secret")), valStyle.Render(s.SecretMasked))
	}

	var errContent string
	if s.ErrMessage != "" {
		errContent = errStyle.Render(fmt.Sprintf("⚠️ %s", s.ErrMessage))
	}

	// 按钮样式
	normalBtnStyle := lipgloss.NewStyle().
		Foreground(t.Muted)

	activeBtnStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary)

	btn0Text := i18n.T("guide.btn_edit")
	btn1Text := i18n.T("guide.btn_retry")
	btn2Text := i18n.T("guide.btn_close")

	if s.FocusedButton == 0 {
		btn0Text = "▸ " + btn0Text
	} else {
		btn0Text = "  " + btn0Text
	}
	if s.FocusedButton == 1 {
		btn1Text = "▸ " + btn1Text
	} else {
		btn1Text = "  " + btn1Text
	}
	if s.FocusedButton == 2 {
		btn2Text = "▸ " + btn2Text
	} else {
		btn2Text = "  " + btn2Text
	}

	btn0 := normalBtnStyle.Render(btn0Text)
	if s.FocusedButton == 0 {
		btn0 = activeBtnStyle.Render(btn0Text)
	}

	btn1 := normalBtnStyle.Render(btn1Text)
	if s.FocusedButton == 1 {
		btn1 = activeBtnStyle.Render(btn1Text)
	}

	btn2 := normalBtnStyle.Render(btn2Text)
	if s.FocusedButton == 2 {
		btn2 = activeBtnStyle.Render(btn2Text)
	}

	var btnBar string
	if dialogWidth < 65 {
		// 窄屏：垂直堆叠
		btnBar = lipgloss.JoinVertical(lipgloss.Left, btn0, btn1, btn2)
	} else {
		// 宽屏：横向分排
		btnBar = lipgloss.JoinHorizontal(lipgloss.Center, btn0, "   ", btn1, "   ", btn2)
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(i18n.T("guide.title")),
		subtitleStyle.Render(i18n.T("guide.subtitle")),
		infoContent,
		errContent,
		"",
		btnBar,
	)

	dialogBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2).
		Width(dialogWidth).
		Render(body)

	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		dialogBox,
	)
}
