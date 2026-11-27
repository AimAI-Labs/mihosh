package preferences

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type themeDialog struct {
	dialog.Dialog
	innerWidth int
	listTop    int
	listWidth  int
	listStart  int
	listCount  int
	hintTop    int
	hints      [][]common.InlineHelpHint
}

// renderThemeSummary renders the compact single-row theme summary in the settings list.
func renderThemeSummary(current string, maxWidth int) string {
	if maxWidth < 1 {
		maxWidth = 1
	}
	palette, ok := theme.Get(current)
	if !ok {
		palette = theme.Current()
	}

	badge := lipgloss.NewStyle().
		Foreground(theme.Resolve(common.TokyoCyan())).
		Bold(true).
		Render(current)

	swatches := renderThemeSwatchStrip(palette, []lipgloss.Color{
		palette.Primary,
		palette.Info,
		palette.Success,
		palette.Warning,
		palette.Danger,
	})

	arrow := lipgloss.NewStyle().
		Foreground(theme.Resolve(common.TokyoMuted())).
		Render("▸")

	full := badge + "  " + swatches + "  " + arrow
	if lipgloss.Width(full) <= maxWidth {
		return full
	}
	compact := badge + " " + arrow
	if lipgloss.Width(compact) <= maxWidth {
		return compact
	}
	return ansi.Truncate(badge, maxWidth, "…")
}

func renderThemeSwatchStrip(palette theme.Theme, colors []lipgloss.Color) string {
	var parts []string
	for _, c := range colors {
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.Resolve(c)).Render("■"))
	}
	return strings.Join(parts, "")
}

func buildThemeModal(state PageState, width, height int) themeDialog {
	names := theme.Names()
	modalWidth := max(28, min(86, width-4))
	if width > 0 && width < modalWidth {
		modalWidth = max(24, width)
	}
	innerWidth := max(20, modalWidth-4)
	d := themeDialog{innerWidth: innerWidth}

	title := i18n.T("settings.theme.modal_title")
	hints := []common.InlineHelpHint{
		{Key: "↑↓", Desc: i18n.T("settings.theme.hint_select")},
		{Key: "Enter", Desc: i18n.T("settings.theme.hint_apply"), Tone: common.InlineHelpToneSuccess},
		{Key: "Esc", Desc: i18n.T("settings.theme.hint_cancel")},
	}
	d.hints = common.WrapInlineHints(hints, innerWidth)

	// Outer border takes 2 rows (top + bottom); hint rows + 1 divider/spacer when space allows.
	available := height - 2 - len(d.hints)
	if available < 4 {
		available = 4
	}

	activeTheme := GetSettingValue(state, "theme")
	cursor := max(0, min(state.ThemeModal.Cursor, max(0, len(names)-1)))
	focusedName := activeTheme
	if len(names) > 0 {
		focusedName = names[cursor]
	}
	previewPalette, ok := theme.Get(focusedName)
	if !ok {
		previewPalette = theme.Current()
	}

	splitLayout := innerWidth >= 60 && available >= 9
	var sections []string

	if splitLayout {
		listWidth := 25
		gapWidth := 3
		previewWidth := innerWidth - listWidth - gapWidth
		if previewWidth < 28 {
			previewWidth = 28
			listWidth = max(18, innerWidth-previewWidth-gapWidth)
		}

		listCap := min(len(names), max(9, min(12, available-1)))
		win := common.ResolveListWindow(len(names), cursor, state.ThemeModal.Offset, listCap)
		d.listTop = 1 // 1 row for outer top border
		d.listWidth = listWidth
		d.listStart = win.Start
		d.listCount = win.End - win.Start

		listLines := renderThemeListLines(names, win, cursor, activeTheme, listWidth, listCap)
		previewLines := renderThemePreviewCard(previewPalette, previewWidth)

		bodyRows := max(len(listLines), len(previewLines))
		divider := lipgloss.NewStyle().Foreground(theme.Resolve(common.Border())).Render(" │ ")
		emptyList := strings.Repeat(" ", listWidth)
		emptyPreview := strings.Repeat(" ", previewWidth)

		for i := 0; i < bodyRows; i++ {
			leftLine := emptyList
			if i < len(listLines) {
				leftLine = listLines[i]
			}
			rightLine := emptyPreview
			if i < len(previewLines) {
				rightLine = previewLines[i]
			}
			sections = append(sections, leftLine+divider+rightLine)
		}
	} else {
		// Narrow/short fallback: vertical list + compact 2-line preview footer.
		compactPreviewLines := renderCompactThemePreview(previewPalette, innerWidth)
		listCap := min(len(names), max(3, available-len(compactPreviewLines)-1))
		win := common.ResolveListWindow(len(names), cursor, state.ThemeModal.Offset, listCap)
		d.listTop = 1
		d.listWidth = innerWidth
		d.listStart = win.Start
		d.listCount = win.End - win.Start

		sections = append(sections, renderThemeListLines(names, win, cursor, activeTheme, innerWidth, listCap)...)
		if available >= len(sections)+len(compactPreviewLines)+1 {
			sections = append(sections, common.DimStyle().Render(strings.Repeat("─", innerWidth)))
		}
		sections = append(sections, compactPreviewLines...)
	}

	if available > len(sections) {
		sections = append(sections, common.DimStyle().Render(strings.Repeat("─", innerWidth)))
	}
	d.hintTop = 1 + len(sections)
	for _, row := range d.hints {
		sections = append(sections, common.FormatInlineHintRow(row))
	}

	content := common.RenderBorderedPanel(title, strings.Join(sections, "\n"), modalWidth, common.TokyoBlue(), common.TokyoForeground())
	d.Dialog = dialog.New(content, width, height).Clipped()
	return d
}

func renderThemeListLines(names []string, win common.ListWindow, cursor int, activeTheme string, width, height int) []string {
	hasScrollbar := len(names) > height
	contentWidth := width
	if hasScrollbar {
		contentWidth = max(6, width-2)
	}

	var scrollbarLines []string
	if hasScrollbar {
		scrollbarLines = strings.Split(common.BuildVerticalScrollbar(height, len(names), win.Start, true), "\n")
	}

	selectedStyle := lipgloss.NewStyle().
		Background(theme.Resolve(common.TokyoSelected())).
		Foreground(theme.Resolve(common.TokyoCyan())).
		Bold(true)
	normalStyle := lipgloss.NewStyle().
		Foreground(theme.Resolve(common.TokyoForeground()))
	activeMarkStyle := lipgloss.NewStyle().
		Foreground(theme.Resolve(common.TokyoGreen())).
		Bold(true)
	selectedMarkStyle := lipgloss.NewStyle().
		Background(theme.Resolve(common.TokyoSelected())).
		Foreground(theme.Resolve(common.TokyoGreen())).
		Bold(true)

	lines := make([]string, 0, height)
	for row := 0; row < height; row++ {
		idx := win.Start + row
		var itemLine string
		if idx >= 0 && idx < len(names) {
			name := names[idx]
			isFocused := idx == cursor
			isActive := name == activeTheme

			prefix := "  "
			if isFocused {
				prefix = "> "
			}
			markWidth := 0
			if isActive {
				markWidth = 2
			}
			nameBudget := max(1, contentWidth-common.DisplayWidth(prefix)-markWidth)
			label := prefix + common.TruncateDisplay(name, nameBudget)
			padLen := max(0, contentWidth-common.DisplayWidth(label)-markWidth)
			padded := label + strings.Repeat(" ", padLen)

			if isFocused {
				if isActive {
					itemLine = selectedStyle.Render(padded) + selectedMarkStyle.Render(" ✓")
				} else {
					itemLine = selectedStyle.Render(padded)
				}
			} else {
				if isActive {
					itemLine = normalStyle.Render(padded) + activeMarkStyle.Render(" ✓")
				} else {
					itemLine = normalStyle.Render(padded)
				}
			}
		} else {
			itemLine = strings.Repeat(" ", contentWidth)
		}

		if hasScrollbar {
			sb := " "
			if row < len(scrollbarLines) {
				sb = scrollbarLines[row]
			}
			itemLine = itemLine + " " + sb
		}
		lines = append(lines, itemLine)
	}
	return lines
}

// renderThemePreviewCard renders a miniature Mihosh interface painted strictly with the preview palette.
func renderThemePreviewCard(p theme.Theme, width int) []string {
	if width < 20 {
		width = 20
	}
	inner := width - 2
	contentW := max(1, inner-2)

	bg := theme.Resolve(p.Background)
	sel := theme.Resolve(p.Selected)
	fg := theme.Resolve(p.Foreground)
	bright := theme.Resolve(p.Bright)
	muted := theme.Resolve(p.Muted)
	primary := theme.Resolve(p.Primary)
	secondary := theme.Resolve(p.Secondary)
	info := theme.Resolve(p.Info)
	success := theme.Resolve(p.Success)
	warning := theme.Resolve(p.Warning)
	danger := theme.Resolve(p.Danger)
	orange := theme.Resolve(p.Orange)
	border := theme.Resolve(p.Border)

	frame := theme.Frame()
	borderStyle := lipgloss.NewStyle().Foreground(primary).Background(bg)
	titleStyle := lipgloss.NewStyle().Foreground(bright).Background(bg).Bold(true)
	bgPadStyle := lipgloss.NewStyle().Background(bg)

	// Top border with preview title
	previewTitle := i18n.Tf("settings.theme.preview_title", p.Name)
	maxTitle := max(1, inner-4)
	previewTitle = common.TruncateDisplay(previewTitle, maxTitle)
	topPrefix := frame.TopLeft + "─ "
	dashCount := max(0, width-common.DisplayWidth(topPrefix)-common.DisplayWidth(previewTitle)-2)
	topLine := borderStyle.Render(topPrefix) +
		titleStyle.Render(previewTitle) +
		borderStyle.Render(" "+strings.Repeat("─", dashCount)+frame.TopRight)

	wrapRow := func(rendered string, rowBg lipgloss.TerminalColor) string {
		if lipgloss.Width(rendered) > contentW {
			rendered = ansi.Truncate(rendered, contentW, "")
		}
		pad := max(0, contentW-lipgloss.Width(rendered))
		padStr := lipgloss.NewStyle().Background(rowBg).Render(strings.Repeat(" ", pad))
		leftEdge := borderStyle.Render("│") + bgPadStyle.Render(" ")
		rightEdge := bgPadStyle.Render(" ") + borderStyle.Render("│")
		return leftEdge + rendered + padStr + rightEdge
	}

	// Row 1: Mode switcher bar
	ruleTab := lipgloss.NewStyle().Background(sel).Foreground(info).Bold(true).Render(" " + i18n.T("settings.theme.preview_rule") + " ")
	sep := lipgloss.NewStyle().Background(bg).Foreground(muted).Render(" │ ")
	globalTab := lipgloss.NewStyle().Background(bg).Foreground(primary).Render(i18n.T("settings.theme.preview_global"))
	directTab := lipgloss.NewStyle().Background(bg).Foreground(primary).Render(i18n.T("settings.theme.preview_direct"))
	rowModes := wrapRow(ruleTab+sep+globalTab+sep+directTab, bg)

	// Row 2: Divider
	rowDivider := wrapRow(lipgloss.NewStyle().Background(bg).Foreground(border).Render(strings.Repeat("─", contentW)), bg)

	// Helper to format a simulated node row with aligned latency
	formatNodeRow := func(selected bool, name, delay string, delayColor lipgloss.TerminalColor) string {
		rowBg := bg
		indicator := lipgloss.NewStyle().Background(rowBg).Render("  ")
		nameStyle := lipgloss.NewStyle().Background(rowBg).Foreground(fg)
		if selected {
			rowBg = sel
			indicator = lipgloss.NewStyle().Background(rowBg).Foreground(info).Bold(true).Render("┃ ")
			nameStyle = lipgloss.NewStyle().Background(rowBg).Foreground(info).Bold(true)
		}
		dot := lipgloss.NewStyle().Background(rowBg).Foreground(delayColor).Render("● ")
		delayStr := lipgloss.NewStyle().Background(rowBg).Foreground(delayColor).Render(delay)
		delayW := common.DisplayWidth(delay)
		nameBudget := max(1, contentW-4-delayW-1)
		trimmedName := common.TruncateDisplay(name, nameBudget)
		gap := max(1, contentW-4-common.DisplayWidth(trimmedName)-delayW)
		spacer := lipgloss.NewStyle().Background(rowBg).Render(strings.Repeat(" ", gap))
		return wrapRow(indicator+dot+nameStyle.Render(trimmedName)+spacer+delayStr, rowBg)
	}

	// Rows 3-5: Simulated nodes showing latency ladder (Success / Warning / Danger)
	rowNode1 := formatNodeRow(true, "HK-01 Fast", "42ms", success)
	rowNode2 := formatNodeRow(false, "JP-02 Edge", "168ms", warning)
	rowNode3 := formatNodeRow(false, "US-03 Backup", "420ms", danger)

	// Row 6: Simulated traffic & status bar
	up := lipgloss.NewStyle().Background(bg).Foreground(success).Render("↑1.2MB/s")
	sp := lipgloss.NewStyle().Background(bg).Render("  ")
	down := lipgloss.NewStyle().Background(bg).Foreground(secondary).Render("↓8.4MB/s")
	act := lipgloss.NewStyle().Background(bg).Foreground(theme.Resolve(p.Active)).Render("● " + i18n.T("settings.theme.preview_active"))
	rowTraffic := wrapRow(up+sp+down+sp+act, bg)

	// Row 7: Semantic palette swatches
	swatchColors := []lipgloss.TerminalColor{primary, secondary, info, success, warning, danger, orange}
	var blocks []string
	for _, c := range swatchColors {
		blocks = append(blocks, lipgloss.NewStyle().Background(bg).Foreground(c).Render("██"))
	}
	swatchRow := strings.Join(blocks, lipgloss.NewStyle().Background(bg).Render(" "))
	rowSwatches := wrapRow(swatchRow, bg)

	bottomLine := borderStyle.Render(frame.BottomLeft + strings.Repeat("─", inner) + frame.BottomRight)

	return []string{
		topLine,
		rowModes,
		rowDivider,
		rowNode1,
		rowNode2,
		rowNode3,
		rowTraffic,
		rowSwatches,
		bottomLine,
	}
}

func renderCompactThemePreview(p theme.Theme, width int) []string {
	bg := theme.Resolve(p.Background)
	fg := theme.Resolve(p.Foreground)
	padLine := func(s string) string {
		if lipgloss.Width(s) > width {
			s = ansi.Truncate(s, width, "")
		}
		pad := max(0, width-lipgloss.Width(s))
		return s + lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", pad))
	}

	title := lipgloss.NewStyle().Background(bg).Foreground(theme.Resolve(p.Primary)).Bold(true).Render(" " + p.Name + " ")
	var swatches []string
	for _, c := range []lipgloss.Color{p.Primary, p.Secondary, p.Info, p.Success, p.Warning, p.Danger} {
		swatches = append(swatches, lipgloss.NewStyle().Background(bg).Foreground(theme.Resolve(c)).Render("■"))
	}
	line1 := padLine(title + strings.Join(swatches, lipgloss.NewStyle().Background(bg).Render(" ")))

	sp := lipgloss.NewStyle().Background(bg).Foreground(fg).Render(" ")
	l1 := lipgloss.NewStyle().Background(bg).Foreground(theme.Resolve(p.Success)).Render("● 42ms")
	l2 := lipgloss.NewStyle().Background(bg).Foreground(theme.Resolve(p.Warning)).Render("● 168ms")
	l3 := lipgloss.NewStyle().Background(bg).Foreground(theme.Resolve(p.Danger)).Render("● 420ms")
	line2 := padLine(sp + l1 + sp + l2 + sp + l3)
	return []string{line1, line2}
}
