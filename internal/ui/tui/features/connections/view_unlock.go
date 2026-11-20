package connections

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const unlockGridTop = 4 // Three toolbar lines and one spacer.

type unlockCardLayout struct {
	grid       common.CardGrid
	allButtonX int
	allButtonW int
}

func unlockPanelWidth(width int) int { return max(12, min(116, width-2)) }

func unlockColumns(width int) int {
	return min(3, max(1, (unlockPanelWidth(width)-1+common.CardGap)/(32+common.CardGap)))
}

func (u unlockView) popup(width, height int) (dialog.Dialog, unlockCardLayout) {
	panelWidth := unlockPanelWidth(width)
	columns := unlockColumns(width)
	gridWidth := panelWidth - 1 - (columns-1)*common.CardGap
	widths := make([]int, columns)
	for column := range widths {
		widths[column] = gridWidth / columns
	}
	widths[columns-1] += gridWidth % columns
	grid := common.LayoutCardGrid(len(u.items), u.cursor, 0, height-1-unlockGridTop, 5, widths...)

	palette := theme.Current()
	buttonColor := palette.Primary
	for _, item := range u.items {
		if item.checking {
			buttonColor = palette.Muted
			break
		}
	}
	title := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Bright)).Bold(true).Render(i18n.T("conns.unlock.title"))
	button := "S " + i18n.T("conns.unlock.run_all")
	if panelWidth-4 < lipgloss.Width(button)+4 {
		button = "S"
	}
	buttonWidth := lipgloss.Width(button)
	button = lipgloss.NewStyle().Foreground(theme.Resolve(buttonColor)).Bold(true).Render(button)
	toolbar := common.RenderCard(panelWidth, palette.Border, palette.Surface, []string{common.CardHeading(panelWidth-4, title, button)})

	cards := grid.Render(func(index, width, height int) string {
		return renderUnlockCard(u.items[index], index == grid.Window.Selected, width, height)
	})
	bar := common.BuildVerticalScrollbar((grid.Rows.End-grid.Rows.Start)*grid.CardHeight,
		grid.TotalRows*grid.CardHeight, grid.Rows.Start*grid.CardHeight, false)
	body := lipgloss.JoinVertical(lipgloss.Left, toolbar, "", lipgloss.JoinHorizontal(lipgloss.Top, cards, bar))
	return dialog.New(body, width, max(1, height-1)).Clipped(),
		unlockCardLayout{grid: grid, allButtonX: panelWidth - 2 - buttonWidth, allButtonW: buttonWidth}
}

func renderUnlockCard(item unlockItem, selected bool, width, height int) string {
	palette := theme.Current()
	status, tone, border := "pending", palette.Muted, palette.Border
	switch item.result.Status {
	case model.UnlockAvailable:
		status, tone, border = string(item.result.Status), palette.Success, palette.Success
	case model.UnlockRestricted, model.UnlockFailed:
		status, tone, border = string(item.result.Status), palette.Danger, palette.Danger
	case model.UnlockOriginals, model.UnlockComingSoon, model.UnlockInconclusive:
		status, tone, border = string(item.result.Status), palette.Warning, palette.Warning
	}
	action := "↻"
	if item.checking {
		status, tone, border = "checking", palette.Info, palette.Info
		action = "⟳"
	}
	name, titleColor, background := item.name, palette.Bright, palette.Surface
	if selected {
		name = common.SymbolSelectActive + name
		titleColor, background = palette.Primary, palette.Selected
	}
	title := lipgloss.NewStyle().Foreground(theme.Resolve(titleColor)).Bold(true).Render(name)
	if height < 3 {
		line := lipgloss.NewStyle().Width(width).Background(theme.Resolve(background)).Render(ansi.Truncate(title, width, "…"))
		return lipgloss.PlaceVertical(height, lipgloss.Top, line)
	}
	inner := width - 4
	action = lipgloss.NewStyle().Foreground(theme.Resolve(palette.Primary)).Render(action)
	result := lipgloss.NewStyle().Foreground(theme.Resolve(tone)).Render(i18n.T("conns.unlock." + status))
	if region := strings.TrimSpace(ansi.Strip(item.result.Region)); region != "" && !item.checking {
		chip := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Info)).Render("[" + region + "]")
		result = common.CardHeading(inner, result, chip)
	}
	checked := ""
	if !item.result.CheckedAt.IsZero() && !item.checking {
		format := "2006-01-02 15:04:05"
		if inner < len(format) {
			format = "15:04:05"
		}
		checked = common.MutedStyle().Render(item.result.CheckedAt.Local().Format(format))
		checked = lipgloss.PlaceHorizontal(inner, lipgloss.Right, checked)
	}
	rows := []string{common.CardHeading(inner, title, action), result, checked}
	return common.RenderCard(width, border, background, rows[:height-2])
}
