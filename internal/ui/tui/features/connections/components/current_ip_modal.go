package components

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	currentIPModalMaxWidth   = 76
	currentIPModalMinWidth   = 24
	currentIPModalTwoColMinW = 56
	currentIPModalColGap     = 2
	currentIPModalCardInset  = 2 // 1 border + 1 horizontal padding
)

// CurrentIPModalLayout holds the rendered exit IP info popup dialog and its local clickable regions.
type CurrentIPModalLayout struct {
	Dialog       dialog.Dialog
	RefreshX     int // local X inside dialog, -1 if not clickable
	RefreshY     int // local Y inside dialog
	RefreshWidth int
	MaskX        int // local X inside dialog, -1 if not clickable
	MaskY        int // local Y inside dialog
	MaskWidth    int
	MaxScroll    int
}

// LayoutCurrentIPModal renders the full exit IP information dialog (inspired by Clash Verge Rev)
// so all fields remain readable even when the single-line summary bar is clipped on narrow screens.
func LayoutCurrentIPModal(state CurrentIPBarState, width, height, scroll int) CurrentIPModalLayout {
	res := CurrentIPModalLayout{
		RefreshX: -1,
		RefreshY: -1,
		MaskX:    -1,
		MaskY:    -1,
	}
	if width <= 0 || height <= 0 {
		return res
	}

	viewportHeight := max(1, height-1)
	panelWidth := min(currentIPModalMaxWidth, width-4)
	if panelWidth < currentIPModalMinWidth {
		panelWidth = max(12, width-2)
	}
	if panelWidth > width {
		panelWidth = width
	}
	innerWidth := max(1, panelWidth-2*currentIPModalCardInset)

	palette := theme.Current()
	titleStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Bright)).Bold(true)
	countryCodeStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Primary)).Bold(true)
	countryStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Bright)).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Muted))
	valueStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Foreground))
	ipStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Bright)).Bold(true)
	asnStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Secondary)).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Muted))
	warnStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Warning)).Bold(true)
	dangerStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Danger))
	refreshStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Primary)).Bold(true)

	// Header row (body[0] -> localY = 1)
	title := titleStyle.Render(i18n.T("conns.ip.modal_title"))
	var actionRendered string
	if state.Loading {
		actionRendered = warnStyle.Render("⟳")
	} else {
		actionText := "↻"
		actionRendered = refreshStyle.Render(actionText)
		actionW := lipgloss.Width(actionText)
		if innerWidth >= actionW+2 {
			res.RefreshWidth = actionW
			res.RefreshX = panelWidth - currentIPModalCardInset - actionW
			res.RefreshY = 1
		}
	}

	divider := mutedStyle.Render(strings.Repeat("─", innerWidth))
	headerLines := []string{
		common.CardHeading(innerWidth, title, actionRendered),
		divider,
	}

	// Build scrollable content lines and track the IP mask toggle span within contentLines.
	var contentLines []string
	maskLineIdx := -1
	maskColStart := -1
	maskColWidth := 0

	switch {
	case state.Info != nil:
		contentLines, maskLineIdx, maskColStart, maskColWidth = buildCurrentIPModalBody(
			state.Info, state.Masked, innerWidth,
			countryCodeStyle, countryStyle, labelStyle, valueStyle, ipStyle, asnStyle, mutedStyle,
		)
	case state.Loading:
		contentLines = wrapStyledLines("⟳ "+i18n.T("conns.ip.loading"), innerWidth, warnStyle)
	case state.Err != "":
		contentLines = append(contentLines, wrapStyledLines("✗ "+i18n.T("conns.ip.failed"), innerWidth, dangerStyle)...)
		if detail := strings.TrimSpace(state.Err); detail != "" {
			contentLines = append(contentLines, "")
			contentLines = append(contentLines, wrapStyledLines(detail, innerWidth, valueStyle)...)
		}
	default:
		contentLines = wrapStyledLines("— "+i18n.T("conns.ip.unavailable"), innerWidth, mutedStyle)
	}

	// Footer lines: auto-refresh countdown on left, country code & coordinates on right.
	footerLines := buildCurrentIPModalFooter(state, innerWidth, mutedStyle, warnStyle)

	// Fit scrollable body between fixed header and footer.
	fixedRows := 2 + len(headerLines) + 1 + len(footerLines) // top/bottom border + header + footer divider + footer
	maxContentLines := max(1, viewportHeight-fixedRows)
	maxScroll := max(0, len(contentLines)-maxContentLines)
	res.MaxScroll = maxScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	end := min(len(contentLines), scroll+maxContentLines)
	visibleContent := contentLines[scroll:end]

	if maskLineIdx >= scroll && maskLineIdx < end && maskColStart >= 0 && maskColWidth > 0 {
		res.MaskY = 1 + len(headerLines) + (maskLineIdx - scroll)
		res.MaskX = currentIPModalCardInset + maskColStart
		res.MaskWidth = maskColWidth
	}

	body := make([]string, 0, len(headerLines)+len(visibleContent)+1+len(footerLines))
	body = append(body, headerLines...)
	body = append(body, visibleContent...)
	body = append(body, divider)
	body = append(body, footerLines...)

	panel := common.RenderCard(panelWidth, palette.Primary, body)
	res.Dialog = dialog.New(panel, width, viewportHeight).Clipped()
	return res
}

func buildCurrentIPModalBody(
	info *model.IPInfo,
	masked bool,
	innerWidth int,
	countryCodeStyle lipgloss.Style,
	countryStyle lipgloss.Style,
	labelStyle lipgloss.Style,
	valueStyle lipgloss.Style,
	ipStyle lipgloss.Style,
	asnStyle lipgloss.Style,
	mutedStyle lipgloss.Style,
) ([]string, int, int, int) {
	countryCode := strings.ToUpper(strings.TrimSpace(firstNonEmpty(info.CountryCode, info.CountryCodeApi)))
	countryName := strings.TrimSpace(info.Country)
	ipText := firstNonEmpty(info.IP, info.Query, "-")
	asn := firstNonEmpty(formatCurrentIPASN(info), "-")
	isp := firstNonEmpty(strings.TrimSpace(info.ISP), "-")
	org := firstNonEmpty(strings.TrimSpace(info.Organization), strings.TrimSpace(info.Org), strings.TrimSpace(info.ASNOrganization), "-")
	location := formatModalIPLocation(info)
	timezone := firstNonEmpty(strings.TrimSpace(info.Timezone), "-")

	ipLabel := i18n.T("conns.ip.field.ip") + ": "
	var ipValPlain, ipValRendered string
	if masked {
		ipValPlain = maskedIPPlaceholder + " ◎"
		ipValRendered = mutedStyle.Render(ipValPlain)
	} else {
		ipValPlain = ipText + " ◉"
		ipValRendered = ipStyle.Render(ipText) + mutedStyle.Render(" ◉")
	}

	leftW := max(22, innerWidth*42/100)
	rightW := innerWidth - leftW - currentIPModalColGap
	useTwoCols := innerWidth >= currentIPModalTwoColMinW &&
		rightW >= 24 &&
		lipgloss.Width(ipLabel+ipValPlain) <= leftW

	if useTwoCols {
		var leftRows []string
		leftRows = append(leftRows, renderCountryHeader(countryCode, countryName, leftW, countryCodeStyle, countryStyle, mutedStyle)...)
		maskRowIdx := len(leftRows)
		maskColStart := lipgloss.Width(ipLabel)
		maskColWidth := lipgloss.Width(ipValPlain)
		leftRows = append(leftRows, labelStyle.Render(ipLabel)+ipValRendered)
		leftRows = append(leftRows, wrapKVRow(i18n.T("conns.ip.field.asn"), asn, leftW, labelStyle, asnStyle)...)

		var rightRows []string
		rightRows = append(rightRows, wrapKVRow(i18n.T("conns.ip.field.isp"), isp, rightW, labelStyle, valueStyle)...)
		rightRows = append(rightRows, wrapKVRow(i18n.T("conns.ip.field.org"), org, rightW, labelStyle, valueStyle)...)
		rightRows = append(rightRows, wrapKVRow(i18n.T("conns.ip.field.location"), location, rightW, labelStyle, valueStyle)...)
		rightRows = append(rightRows, wrapKVRow(i18n.T("conns.ip.field.timezone"), timezone, rightW, labelStyle, valueStyle)...)

		rowCount := max(len(leftRows), len(rightRows))
		rows := make([]string, 0, rowCount)
		gap := strings.Repeat(" ", currentIPModalColGap)
		for i := 0; i < rowCount; i++ {
			leftLine := ""
			if i < len(leftRows) {
				leftLine = leftRows[i]
			}
			rightLine := ""
			if i < len(rightRows) {
				rightLine = rightRows[i]
			}
			pad := leftW - lipgloss.Width(leftLine)
			if pad < 0 {
				leftLine = ansi.Truncate(leftLine, leftW, "")
				pad = 0
			}
			rows = append(rows, leftLine+strings.Repeat(" ", pad)+gap+rightLine)
		}
		return rows, maskRowIdx, maskColStart, maskColWidth
	}

	// Single-column layout for narrow terminals or long IPv6 addresses: wraps every field cleanly.
	var rows []string
	rows = append(rows, renderCountryHeader(countryCode, countryName, innerWidth, countryCodeStyle, countryStyle, mutedStyle)...)

	maskRowIdx := len(rows)
	var maskColStart, maskColWidth int
	if lipgloss.Width(ipLabel+ipValPlain) <= innerWidth {
		rows = append(rows, labelStyle.Render(ipLabel)+ipValRendered)
		maskColStart = lipgloss.Width(ipLabel)
		maskColWidth = lipgloss.Width(ipValPlain)
	} else {
		ipStyleToUse := ipStyle
		if masked {
			ipStyleToUse = mutedStyle
		}
		wrappedIP := wrapKVRow(i18n.T("conns.ip.field.ip"), ipValPlain, innerWidth, labelStyle, ipStyleToUse)
		rows = append(rows, wrappedIP...)
		maskColStart = 0
		maskColWidth = innerWidth
	}

	rows = append(rows, wrapKVRow(i18n.T("conns.ip.field.asn"), asn, innerWidth, labelStyle, asnStyle)...)
	rows = append(rows, wrapKVRow(i18n.T("conns.ip.field.isp"), isp, innerWidth, labelStyle, valueStyle)...)
	rows = append(rows, wrapKVRow(i18n.T("conns.ip.field.org"), org, innerWidth, labelStyle, valueStyle)...)
	rows = append(rows, wrapKVRow(i18n.T("conns.ip.field.location"), location, innerWidth, labelStyle, valueStyle)...)
	rows = append(rows, wrapKVRow(i18n.T("conns.ip.field.timezone"), timezone, innerWidth, labelStyle, valueStyle)...)
	return rows, maskRowIdx, maskColStart, maskColWidth
}

func buildCurrentIPModalFooter(
	state CurrentIPBarState,
	innerWidth int,
	mutedStyle lipgloss.Style,
	warnStyle lipgloss.Style,
) []string {
	var leftRendered string
	switch {
	case state.Loading:
		leftRendered = warnStyle.Render("⟳ " + i18n.T("conns.ip.refreshing"))
	case state.Countdown > 0:
		leftRendered = mutedStyle.Render(i18n.Tf("conns.ip.auto_refresh", state.Countdown))
	default:
		leftRendered = mutedStyle.Render("—")
	}

	var rightPlain string
	if state.Info != nil {
		countryCode := strings.ToUpper(strings.TrimSpace(firstNonEmpty(state.Info.CountryCode, state.Info.CountryCodeApi)))
		lat, lon, hasCoord := coordinates(state.Info)
		switch {
		case countryCode != "" && hasCoord:
			rightPlain = fmt.Sprintf("%s, %.2f, %.2f", countryCode, lat, lon)
		case hasCoord:
			rightPlain = fmt.Sprintf("%.2f, %.2f", lat, lon)
		case countryCode != "":
			rightPlain = countryCode
		}
	}

	if rightPlain == "" {
		return []string{leftRendered}
	}

	rightRendered := mutedStyle.Render(rightPlain)
	leftW := lipgloss.Width(leftRendered)
	rightW := lipgloss.Width(rightRendered)
	if leftW+2+rightW <= innerWidth {
		return []string{leftRendered + strings.Repeat(" ", innerWidth-leftW-rightW) + rightRendered}
	}
	lines := []string{leftRendered}
	lines = append(lines, wrapStyledLines(rightPlain, innerWidth, mutedStyle)...)
	return lines
}

func renderCountryHeader(
	countryCode, countryName string,
	width int,
	codeStyle, nameStyle, mutedStyle lipgloss.Style,
) []string {
	switch {
	case countryCode != "" && countryName != "":
		badge := "[" + countryCode + "]"
		fullW := lipgloss.Width(badge) + 1 + lipgloss.Width(countryName)
		if fullW <= width {
			return []string{codeStyle.Render(badge) + " " + nameStyle.Render(countryName)}
		}
		lines := []string{codeStyle.Render(badge)}
		lines = append(lines, wrapStyledLines(countryName, width, nameStyle)...)
		return lines
	case countryName != "":
		return wrapStyledLines(countryName, width, nameStyle)
	case countryCode != "":
		return []string{codeStyle.Render("[" + countryCode + "]")}
	default:
		return []string{mutedStyle.Render("-")}
	}
}

func wrapKVRow(label, value string, width int, labelStyle, valueStyle lipgloss.Style) []string {
	prefixPlain := label + ": "
	prefix := labelStyle.Render(prefixPlain)
	prefixW := lipgloss.Width(prefixPlain)

	if prefixW+lipgloss.Width(value) <= width {
		return []string{prefix + valueStyle.Render(value)}
	}
	if rem := width - prefixW; rem >= 8 {
		wrapped := strings.Split(ansi.Wrap(value, rem, ""), "\n")
		lines := make([]string, 0, len(wrapped))
		indent := strings.Repeat(" ", prefixW)
		for i, part := range wrapped {
			if i == 0 {
				lines = append(lines, prefix+valueStyle.Render(part))
			} else {
				lines = append(lines, indent+valueStyle.Render(part))
			}
		}
		return lines
	}
	lines := []string{labelStyle.Render(label + ":")}
	valW := max(1, width-2)
	for _, part := range strings.Split(ansi.Wrap(value, valW, ""), "\n") {
		lines = append(lines, "  "+valueStyle.Render(part))
	}
	return lines
}

func wrapStyledLines(text string, width int, style lipgloss.Style) []string {
	if width <= 0 {
		return nil
	}
	wrapped := strings.Split(ansi.Wrap(text, width, ""), "\n")
	lines := make([]string, 0, len(wrapped))
	for _, part := range wrapped {
		lines = append(lines, style.Render(part))
	}
	return lines
}

func formatModalIPLocation(info *model.IPInfo) string {
	city := strings.TrimSpace(info.City)
	region := strings.TrimSpace(firstNonEmpty(info.RegionName, info.Region))
	parts := make([]string, 0, 2)
	if city != "" {
		parts = append(parts, city)
	}
	if region != "" && !strings.EqualFold(region, city) {
		parts = append(parts, region)
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	if country := strings.TrimSpace(info.Country); country != "" {
		return country
	}
	return "-"
}
