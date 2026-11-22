package components

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

const maskedIPPlaceholder = "••••••••"

// CurrentIPBarState holds the display state for the single-line current IP info bar.
type CurrentIPBarState struct {
	Info      *model.IPInfo
	Loading   bool
	Err       string
	Masked    bool
	Countdown int
}

// CurrentIPBarLayout holds the rendered single-line bar and clickable X spans.
type CurrentIPBarLayout struct {
	Content      string
	IPStartX     int // inclusive display column, -1 if not clickable
	IPEndX       int // exclusive display column
	RefreshStart int // inclusive display column, -1 if not clickable
	RefreshEnd   int // exclusive display column
}

// RenderCurrentIPBar renders the single-line current exit IP summary bar.
func RenderCurrentIPBar(state CurrentIPBarState, width int) string {
	return LayoutCurrentIPBar(state, width).Content
}

// LayoutCurrentIPBar renders the single-line current exit IP summary bar and records
// clickable ranges for toggling IP visibility and triggering manual refresh.
func LayoutCurrentIPBar(state CurrentIPBarState, width int) CurrentIPBarLayout {
	layout := CurrentIPBarLayout{
		IPStartX:     -1,
		IPEndX:       -1,
		RefreshStart: -1,
		RefreshEnd:   -1,
	}
	if width <= 0 {
		return layout
	}

	palette := theme.Current()
	labelStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Info)).Bold(true)
	countryCodeStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Primary)).Bold(true)
	textStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Foreground))
	ipStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Bright)).Bold(true)
	asnStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Secondary))
	mutedStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Muted))
	warnStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Warning)).Bold(true)
	dangerStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Danger))
	refreshIconStyle := lipgloss.NewStyle().Foreground(theme.Resolve(palette.Primary)).Bold(true)

	// Align with the 1-cell border + 1-cell padding of adjacent boxes by adding a 1-cell side margin when space allows.
	leftMargin := ""
	rightMargin := ""
	innerWidth := width
	if width >= 12 {
		leftMargin = " "
		rightMargin = " "
		innerWidth = width - 2
	}

	// Build right-aligned refresh / countdown indicator.
	var rightStr string
	var rightClickable bool
	switch {
	case state.Loading:
		if state.Info != nil {
			rightStr = warnStyle.Render("⟳")
		}
	case state.Countdown > 0:
		rightStr = refreshIconStyle.Render("↻") + mutedStyle.Render(fmt.Sprintf(" %ds", state.Countdown))
		rightClickable = true
	default:
		rightStr = refreshIconStyle.Render("↻")
		rightClickable = true
	}

	rightWidth := lipgloss.Width(rightStr)
	if rightWidth > innerWidth {
		rightStr = ""
		rightWidth = 0
		rightClickable = false
	}

	minGap := 2
	if rightWidth == 0 {
		minGap = 0
	}
	leftAvail := innerWidth - rightWidth - minGap
	if leftAvail < 0 {
		if rightWidth > 0 && innerWidth > 0 {
			minGap = 1
			leftAvail = innerWidth - rightWidth - minGap
		}
		if leftAvail < 0 {
			rightStr = ""
			rightWidth = 0
			rightClickable = false
			leftAvail = innerWidth
		}
	}

	prefixText := i18n.T("conns.ip.label")
	prefix := labelStyle.Render(prefixText)
	sep := mutedStyle.Render(" · ")
	sepW := lipgloss.Width(sep)

	var leftStr string
	switch {
	case state.Info != nil:
		leftStr, layout.IPStartX, layout.IPEndX = buildCurrentIPSegments(
			state.Info, state.Masked, leftAvail, len(leftMargin),
			prefix, sep, sepW,
			countryCodeStyle, textStyle, ipStyle, asnStyle, mutedStyle,
		)
	case state.Loading:
		msg := "⟳ " + i18n.T("conns.ip.loading")
		leftStr = fitPrefixAndStatus(prefix, msg, leftAvail, warnStyle)
	case state.Err != "":
		msg := "✗ " + i18n.T("conns.ip.failed")
		detail := strings.TrimSpace(state.Err)
		if detail != "" {
			full := msg + ": " + detail
			if common.DisplayWidth(prefixText)+1+common.DisplayWidth(full) <= leftAvail {
				msg = full
			} else if rem := leftAvail - common.DisplayWidth(prefixText) - 1; rem > common.DisplayWidth(msg)+4 {
				msg = common.TruncateDisplay(full, rem)
			}
		}
		leftStr = fitPrefixAndStatus(prefix, msg, leftAvail, dangerStyle)
	default:
		leftStr = fitPrefixAndStatus(prefix, "— "+i18n.T("conns.ip.unavailable"), leftAvail, mutedStyle)
	}

	leftWidth := lipgloss.Width(leftStr)
	if leftWidth > leftAvail {
		leftStr = common.TruncateDisplay(leftStr, leftAvail)
		leftWidth = lipgloss.Width(leftStr)
		if layout.IPStartX >= 0 && layout.IPEndX > len(leftMargin)+leftWidth {
			layout.IPEndX = len(leftMargin) + leftWidth
			if layout.IPEndX <= layout.IPStartX {
				layout.IPStartX, layout.IPEndX = -1, -1
			}
		}
	}

	gap := innerWidth - leftWidth - rightWidth
	if gap < 0 {
		gap = 0
	}

	if rightClickable && rightWidth > 0 {
		layout.RefreshStart = len(leftMargin) + leftWidth + gap
		layout.RefreshEnd = layout.RefreshStart + rightWidth
	}

	layout.Content = leftMargin + leftStr + strings.Repeat(" ", gap) + rightStr + rightMargin
	return layout
}

func fitPrefixAndStatus(prefix, status string, avail int, statusStyle lipgloss.Style) string {
	if avail <= 0 {
		return ""
	}
	prefixW := lipgloss.Width(prefix)
	if prefixW+1 >= avail {
		return statusStyle.Render(common.TruncateDisplay(status, avail))
	}
	rem := avail - prefixW - 1
	return prefix + " " + statusStyle.Render(common.TruncateDisplay(status, rem))
}

type ipBarSegment struct {
	rendered    string
	plain       string
	style       lipgloss.Style
	isIP        bool
	truncatable bool
}

func buildCurrentIPSegments(
	info *model.IPInfo,
	masked bool,
	avail int,
	baseX int,
	prefix string,
	sep string,
	sepW int,
	countryCodeStyle lipgloss.Style,
	textStyle lipgloss.Style,
	ipStyle lipgloss.Style,
	asnStyle lipgloss.Style,
	mutedStyle lipgloss.Style,
) (string, int, int) {
	if avail <= 0 {
		return "", -1, -1
	}

	ipText := firstNonEmpty(info.IP, info.Query, "-")
	var ipRendered string
	var eyeIconWidth int
	if masked {
		eyeIconWidth = lipgloss.Width("◎")
		ipRendered = mutedStyle.Render(maskedIPPlaceholder + " ◎")
	} else {
		eyeIconWidth = lipgloss.Width("◉")
		ipRendered = ipStyle.Render(ipText) + mutedStyle.Render(" ◉")
	}

	countryCode := strings.ToUpper(strings.TrimSpace(firstNonEmpty(info.CountryCode, info.CountryCodeApi)))
	countryName := strings.TrimSpace(info.Country)

	var countryFull string
	switch {
	case countryCode != "" && countryName != "":
		countryFull = countryCodeStyle.Render("["+countryCode+"]") + " " + textStyle.Render(countryName)
	case countryName != "":
		countryFull = textStyle.Render(countryName)
	case countryCode != "":
		countryFull = countryCodeStyle.Render("[" + countryCode + "]")
	}

	var countryCompact string
	if countryCode != "" {
		countryCompact = countryCodeStyle.Render("[" + countryCode + "]")
	}

	asn := formatCurrentIPASN(info)
	location := formatCurrentIPLocation(info, countryName)
	network := formatCurrentIPNetwork(info)
	timezone := strings.TrimSpace(info.Timezone)
	lat, lon, hasCoord := coordinates(info)

	var segments []ipBarSegment
	if countryFull != "" {
		segments = append(segments, ipBarSegment{rendered: countryFull})
	}
	segments = append(segments, ipBarSegment{rendered: ipRendered, isIP: true})
	if asn != "" {
		segments = append(segments, ipBarSegment{rendered: asnStyle.Render(asn)})
	}
	if location != "" {
		segments = append(segments, ipBarSegment{
			rendered:    textStyle.Render(location),
			plain:       location,
			style:       textStyle,
			truncatable: true,
		})
	}
	if network != "" {
		segments = append(segments, ipBarSegment{
			rendered:    textStyle.Render(network),
			plain:       network,
			style:       textStyle,
			truncatable: true,
		})
	}
	if timezone != "" {
		segments = append(segments, ipBarSegment{rendered: mutedStyle.Render(timezone)})
	}
	if hasCoord {
		coord := fmt.Sprintf("%.2f, %.2f", lat, lon)
		segments = append(segments, ipBarSegment{rendered: mutedStyle.Render(coord)})
	}

	// Start with "IP " prefix if it leaves room for at least the IP segment.
	var b strings.Builder
	used := 0
	ipStartX, ipEndX := -1, -1

	prefixW := lipgloss.Width(prefix)
	ipW := lipgloss.Width(ipRendered)
	if prefixW+1+ipW <= avail {
		b.WriteString(prefix)
		b.WriteByte(' ')
		used = prefixW + 1
	}

	// If full country + IP won't fit, shrink country to compact [CC] badge so IP always stays visible.
	if len(segments) >= 2 && segments[1].isIP {
		needed := lipgloss.Width(segments[0].rendered) + sepW + ipW
		if used+needed > avail {
			if countryCompact != "" && used+lipgloss.Width(countryCompact)+sepW+ipW <= avail {
				segments[0].rendered = countryCompact
			} else {
				segments = segments[1:]
			}
		}
	}

	count := 0
	for _, seg := range segments {
		segW := lipgloss.Width(seg.rendered)
		addSep := count > 0
		needed := segW
		if addSep {
			needed += sepW
		}

		if used+needed <= avail {
			if addSep {
				b.WriteString(sep)
				used += sepW
			}
			if seg.isIP && eyeIconWidth > 0 {
				ipStartX = baseX + used + segW - eyeIconWidth
				ipEndX = baseX + used + segW
			}
			b.WriteString(seg.rendered)
			used += segW
			count++
			continue
		}

		// If the very first segment (IP) exceeds ultra-narrow width, truncate it directly.
		// The trailing eye icon is clipped away, so clicking the bar opens the full IP dialog instead.
		if count == 0 {
			truncated := common.TruncateDisplay(seg.rendered, avail-used)
			b.WriteString(truncated)
			break
		}

		// Truncate long descriptive segments (location/network) to make full use of remaining width.
		if seg.truncatable && seg.plain != "" {
			rem := avail - used
			if addSep {
				rem -= sepW
			}
			if rem >= 8 {
				if addSep {
					b.WriteString(sep)
				}
				truncated := seg.style.Render(common.TruncateDisplay(seg.plain, rem))
				b.WriteString(truncated)
			}
		}
		break
	}

	return b.String(), ipStartX, ipEndX
}

func formatCurrentIPASN(info *model.IPInfo) string {
	if info.ASN > 0 {
		return fmt.Sprintf("AS%d", info.ASN)
	}
	as := strings.TrimSpace(info.AS)
	if as == "" {
		return ""
	}
	if fields := strings.Fields(as); len(fields) > 0 && strings.HasPrefix(strings.ToUpper(fields[0]), "AS") {
		return strings.ToUpper(fields[0])
	}
	return as
}

func formatCurrentIPLocation(info *model.IPInfo, country string) string {
	city := strings.TrimSpace(info.City)
	region := strings.TrimSpace(firstNonEmpty(info.RegionName, info.Region))
	parts := make([]string, 0, 2)
	if city != "" && !strings.EqualFold(city, country) {
		parts = append(parts, city)
	}
	if region != "" && !strings.EqualFold(region, country) && !strings.EqualFold(region, city) {
		parts = append(parts, region)
	}
	return strings.Join(parts, ", ")
}

func formatCurrentIPNetwork(info *model.IPInfo) string {
	org := cleanKnownNetwork(firstNonEmpty(info.Organization, info.Org, info.ASNOrganization))
	isp := cleanKnownNetwork(info.ISP)
	switch {
	case org != "" && isp != "":
		if strings.EqualFold(org, isp) || strings.Contains(strings.ToLower(org), strings.ToLower(isp)) {
			return org
		}
		if strings.Contains(strings.ToLower(isp), strings.ToLower(org)) {
			return isp
		}
		return isp + " / " + org
	case org != "":
		return org
	default:
		return isp
	}
}

func cleanKnownNetwork(value string) string {
	v := strings.TrimSpace(value)
	if v == "" || strings.EqualFold(v, "unknown") {
		return ""
	}
	return v
}
