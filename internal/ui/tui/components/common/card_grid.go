package common

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const CardGap = 2

// CardGrid lays out complete rows of cards and shares coordinates with mouse hit testing.
type CardGrid struct {
	Window     ListWindow
	CardHeight int
	Width      int
	Widths     []int
	Rows       ListWindow
	TotalRows  int
}

func LayoutCardGrid(total, selected, scrollTop, height, preferredHeight int, widths ...int) CardGrid {
	columns := len(widths)
	cardHeight := max(1, min(height, preferredHeight))
	totalRows := (total + columns - 1) / columns
	selected = max(0, min(selected, total-1))
	rows := ResolveListWindow(totalRows, selected/columns, scrollTop/columns, height/cardHeight)
	width := CardGap * (columns - 1)
	for _, w := range widths {
		width += w
	}
	return CardGrid{
		Window:     ListWindow{Selected: selected, Start: rows.Start * columns, End: min(total, rows.End*columns)},
		CardHeight: cardHeight,
		Width:      width,
		Widths:     widths,
		Rows:       rows,
		TotalRows:  totalRows,
	}
}

func (g CardGrid) ItemAt(x, y int) int {
	if x < 0 || y < 0 {
		return -1
	}
	for col, width := range g.Widths {
		if x < width {
			index := g.Window.Start + (y/g.CardHeight)*len(g.Widths) + col
			if index < g.Window.End {
				return index
			}
			return -1
		}
		x -= width + CardGap
		if x < 0 {
			return -1
		}
	}
	return -1
}

// CardOrigin returns the top-left coordinate and width of a visible card.
func (g CardGrid) CardOrigin(index int) (x, y, width int) {
	column := (index - g.Window.Start) % len(g.Widths)
	for _, w := range g.Widths[:column] {
		x += w + CardGap
	}
	return x, ((index - g.Window.Start) / len(g.Widths)) * g.CardHeight, g.Widths[column]
}

func (g CardGrid) Render(card func(index, width, height int) string) string {
	columns := len(g.Widths)
	var rows []string
	for start := g.Window.Start; start < g.Window.End; start += columns {
		var row []string
		for index := start; index < min(start+columns, g.Window.End); index++ {
			if index != start {
				row = append(row, strings.Repeat(" ", CardGap))
			}
			row = append(row, card(index, g.Widths[index-start], g.CardHeight))
		}
		rows = append(rows, lipgloss.PlaceHorizontal(g.Width, lipgloss.Left, lipgloss.JoinHorizontal(lipgloss.Top, row...)))
	}
	return strings.Join(rows, "\n")
}

// CardHeading reserves the last cells of the title row for its action.
func CardHeading(width int, title, action string) string {
	if action == "" {
		return ansi.Truncate(title, width, "…")
	}
	actionWidth := lipgloss.Width(action)
	title = ansi.Truncate(title, max(0, width-actionWidth-1), "…")
	return title + strings.Repeat(" ", max(0, width-lipgloss.Width(title)-actionWidth)) + action
}

func RenderCard(width int, border, background lipgloss.Color, rows []string) string {
	inner := max(1, width-4)
	bodyStyle := lipgloss.NewStyle().
		Background(theme.Resolve(background)).
		Foreground(theme.Resolve(theme.Current().Foreground))
	reset := ansi.SGR(0)
	base := strings.TrimSuffix(bodyStyle.Inline(true).Render(""), reset)
	// Resume the card's colors after a colored label returns the terminal to defaults.
	restore := strings.NewReplacer(reset, reset+base, ansi.ResetStyle, ansi.ResetStyle+base)
	for i, row := range rows {
		rows[i] = restore.Replace(ansi.Truncate(row, inner, "…"))
	}
	return bodyStyle.Border(theme.Frame()).
		BorderForeground(theme.Resolve(border)).
		BorderBackground(theme.Resolve(background)).
		Padding(0, 1).Width(width - 2).Render(strings.Join(rows, "\n"))
}
