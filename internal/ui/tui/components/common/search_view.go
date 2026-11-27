package common

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const SearchHeaderHeight = 3

type SearchBoxOptions struct {
	Label     string
	Query     SearchQuery
	Active    bool
	Hint      string
	HintRegex string
	HintFuzzy string
	Suffix    string
}

func RenderSearchEngineBadge(engine SearchEngine) string {
	switch engine {
	case SearchRegex:
		return lipgloss.NewStyle().Foreground(resolve(TokyoPurple())).Render(" [RE]")
	case SearchFuzzy:
		return lipgloss.NewStyle().Foreground(resolve(TokyoYellow())).Render(" [Fuzzy]")
	default:
		return ""
	}
}

// RenderSearchBox renders the unified inline search prompt shared across pages.
func RenderSearchBox(opts SearchBoxOptions) string {
	inputStyle := lipgloss.NewStyle().Foreground(resolve(Bright()))
	if opts.Active {
		inputStyle = inputStyle.Background(resolve(Highlight()))
	}

	label := MutedStyle().Render(opts.Label)
	input := inputStyle.Render(opts.Query.Text)
	if opts.Active {
		input += inputStyle.Render("█")
	}

	hint := ""
	if opts.Query.Text == "" {
		var hintText string
		switch opts.Query.Engine {
		case SearchRegex:
			hintText = opts.HintRegex
		case SearchFuzzy:
			hintText = opts.HintFuzzy
		default:
			hintText = opts.Hint
		}
		if hintText != "" {
			hint = MutedStyle().Render(hintText)
		}
	}

	return label + input + hint + RenderSearchEngineBadge(opts.Query.Engine) + opts.Suffix
}

// RenderSearchHeaderBar renders a 3-row bordered header bar with search box on the left,
// optional stats right-aligned before any trailing widget (e.g. layout tabs).
func RenderSearchHeaderBar(searchBox, stats, trailing string, width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(resolve(TokyoBlue()))
	statsStyle := lipgloss.NewStyle().Foreground(resolve(TokyoBlue()))

	innerWidth := max(1, width-2)
	trailingW := lipgloss.Width(trailing)
	if trailingW > innerWidth {
		trailing = PadString(ansi.Truncate(trailing, innerWidth, ""), innerWidth)
		trailingW = innerWidth
	}
	available := max(0, innerWidth-trailingW)

	leftPad := ""
	if available > 1 {
		leftPad = " "
	}
	rightPad := ""
	if trailing == "" && stats != "" && available > 2 {
		rightPad = " "
	}
	usable := max(0, available-len(leftPad)-len(rightPad))

	statsText := ""
	if stats != "" {
		statsText = statsStyle.Render(stats)
	}
	searchW := lipgloss.Width(searchBox)
	statsW := lipgloss.Width(statsText)

	var rowContent string
	switch {
	case statsText != "" && searchW+1+statsW <= usable:
		gap := usable - searchW - statsW
		rowContent = leftPad + searchBox + strings.Repeat(" ", gap) + statsText + rightPad
	case statsText != "" && usable-statsW-1 >= min(max(searchW, 4), 10):
		truncated := ansi.Truncate(searchBox, usable-statsW-1, "…")
		gap := max(1, usable-lipgloss.Width(truncated)-statsW)
		rowContent = leftPad + truncated + strings.Repeat(" ", gap) + statsText + rightPad
	default:
		textBudget := max(0, available-len(leftPad))
		rowContent = leftPad + ansi.Truncate(searchBox, textBudget, "…")
	}

	content := PadString(ansi.Truncate(rowContent, available, ""), available)
	searchRow := borderStyle.Render("│") + content + trailing + borderStyle.Render("│")
	topLine := borderStyle.Render(theme.Frame().TopLeft + strings.Repeat("─", innerWidth) + theme.Frame().TopRight)
	bottomLine := borderStyle.Render(theme.Frame().BottomLeft + strings.Repeat("─", innerWidth) + theme.Frame().BottomRight)

	return topLine + "\n" + searchRow + "\n" + bottomLine
}

// HitSearchHeaderBar reports whether (pageX, pageY) hits the search input area of a 3-row header bar.
func HitSearchHeaderBar(pageX, pageY, width, trailingWidth int) bool {
	if pageY != 1 || width < 3 {
		return false
	}
	endX := width - 1 - max(0, trailingWidth)
	return pageX >= 1 && pageX < endX
}

// HitBarBlankSpace reports whether (pageX, pageY) hits the empty gap between left and right controls on row 1.
func HitBarBlankSpace(pageX, pageY, width, leftWidth, rightWidth int) bool {
	if pageY != 1 || width < 3 {
		return false
	}
	innerWidth := width - 2
	startX := 1 + max(0, leftWidth)
	endX := width - 1
	if rightWidth > 0 && leftWidth+1+rightWidth <= innerWidth {
		endX = 1 + innerWidth - rightWidth
	}
	return pageX >= startX && pageX < endX
}
