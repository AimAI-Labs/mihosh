package sub

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type subLayoutMenu struct {
	cards, list string
	start       int
}

func subLayoutTabs(width int) subLayoutMenu {
	cards, list := " "+i18n.T("sub.layout.cards")+" ", " "+i18n.T("sub.layout.list")+" "
	return subLayoutMenu{cards: cards, list: list, start: width - 1 - lipgloss.Width(cards+"│"+list)}
}

func (m subLayoutMenu) render(listMode bool) string {
	if m.start < 1 {
		return ""
	}
	palette := theme.Current()
	tab := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Primary))
	active := tab.Background(theme.Resolve(palette.Selected)).Bold(true)
	cards, list := tab.Render(m.cards), tab.Render(m.list)
	if listMode {
		list = active.Render(m.list)
	} else {
		cards = active.Render(m.cards)
	}
	return cards + common.MutedStyle().Render("│") + list
}

func subLayoutHit(x, y, width int) (listMode, hit bool) {
	menu := subLayoutTabs(width)
	if y != 1 || menu.start < 1 || x < menu.start || x >= width-1 {
		return false, false
	}
	separator := menu.start + lipgloss.Width(menu.cards)
	return x > separator, x != separator
}

func renderSubRow(p profile.Profile, position int, active, updating, selected bool, width int) string {
	palette := theme.Current()
	cell := func(text string, span int, color lipgloss.Color) string {
		text = strings.Join(strings.Fields(ansi.Strip(text)), " ")
		return lipgloss.NewStyle().Foreground(theme.Resolve(color)).Width(span).Render(ansi.Truncate(text, span, "…"))
	}
	cursor, mark := common.SymbolSelectInactive, "  "
	if selected {
		cursor = lipgloss.NewStyle().Foreground(theme.Resolve(palette.Primary)).Render(common.SymbolSelectActive)
	}
	if active {
		mark = lipgloss.NewStyle().Foreground(theme.Resolve(palette.Success)).Render("● ")
	}
	prefix := cursor + mark + " " + cell(fmt.Sprintf("%d.", position), 5, palette.Success) + " "
	updated, updateColor := relativeTime(p.UpdatedAt), palette.Muted
	if updating {
		updated, updateColor = i18n.T("sub.updating"), palette.Info
	}

	const uidWidth, kindWidth, timeWidth = 8, 6, 14
	available := width - lipgloss.Width(prefix)
	flex := available - uidWidth - kindWidth - timeWidth - 4
	var row string
	if flex >= 16 {
		nameWidth := max(8, flex/8)
		kindColor := palette.Primary
		if p.Source.Kind == profile.SourceLocal {
			kindColor = palette.Secondary
		}
		row = strings.Join([]string{
			cell(p.Name, nameWidth, palette.Bright),
			cell(p.UID, uidWidth, palette.Muted),
			cell(p.Source.Kind.String(), kindWidth, kindColor),
			cell(p.Source.Display(), flex-nameWidth, palette.Secondary),
			cell(updated, timeWidth, updateColor),
		}, " ")
	} else if available >= timeWidth+9 {
		row = cell(p.Name, available-timeWidth-1, palette.Bright) + " " + cell(updated, timeWidth, updateColor)
	} else {
		row = cell(p.Name, max(1, available), palette.Bright)
	}
	row = prefix + row
	if selected {
		row = lipgloss.NewStyle().Background(theme.Resolve(palette.Selected)).Render(row)
	}
	return ansi.Truncate(row, width, "…")
}
