package settings

import (
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

type ActionButton struct {
	ID    string
	Label string
	Width int // display width including padding and margin
}

// GetActionButtons returns the list of buttons and their physical widths.
func GetActionButtons() []ActionButton {
	// 模拟和 view.go 一致的宽度计算
	// padding = 2, margin = 1 -> 总附加宽度 = 3
	calcWidth := func(key string) int {
		return lipgloss.Width(i18n.T(key)) + 3
	}

	return []ActionButton{
		{"upgrade_auto", i18n.T("settings.action.upgrade_auto"), calcWidth("settings.action.upgrade_auto")},
		{"upgrade_release", i18n.T("settings.action.upgrade_release"), calcWidth("settings.action.upgrade_release")},
		{"upgrade_alpha", i18n.T("settings.action.upgrade_alpha"), calcWidth("settings.action.upgrade_alpha")},
		{"restart", i18n.T("settings.action.restart"), calcWidth("settings.action.restart")},
		{"reload", i18n.T("settings.action.reload"), calcWidth("settings.action.reload")},
		{"update_geo", i18n.T("settings.action.update_geo"), calcWidth("settings.action.update_geo")},
		{"flush_dns", i18n.T("settings.action.flush_dns"), calcWidth("settings.action.flush_dns")},
		{"flush_fakeip", i18n.T("settings.action.flush_fakeip"), calcWidth("settings.action.flush_fakeip")},
	}
}

// LayoutActionButtons calculates the row layout.
// Returns a slice of rows, where each row is a slice of buttons.
func LayoutActionButtons(width int) [][]ActionButton {
	contentWidth := width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	btns := GetActionButtons()
	var rows [][]ActionButton
	var currentRow []ActionButton
	currentX := 0

	for _, btn := range btns {
		if len(currentRow) > 0 && currentX+btn.Width > contentWidth {
			rows = append(rows, currentRow)
			currentRow = []ActionButton{btn}
			currentX = btn.Width
		} else {
			currentRow = append(currentRow, btn)
			currentX += btn.Width
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}

	return rows
}
