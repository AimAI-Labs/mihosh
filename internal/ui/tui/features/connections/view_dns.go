package connections

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

type dnsTypeHit struct {
	x, y, width int
	index       int
}

type dnsPopupLayout struct {
	typeHits []dnsTypeHit
}

func (d dnsQueryView) popup(width, height int) (dialog.Dialog, dnsPopupLayout) {
	panelWidth := max(36, min(96, width-4))
	innerWidth := max(20, panelWidth-4)
	viewportHeight := max(10, height-2)

	t := theme.Current()
	titleStyle := lipgloss.NewStyle().Foreground(theme.Resolve(t.Bright)).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted))
	primaryStyle := lipgloss.NewStyle().Foreground(theme.Resolve(t.Primary)).Bold(true)
	activePill := lipgloss.NewStyle().Background(theme.Resolve(t.Primary)).Foreground(theme.Resolve(t.Background)).Bold(true)
	inactivePill := lipgloss.NewStyle().Background(theme.Resolve(t.Selected)).Foreground(theme.Resolve(t.Foreground))
	warnBadge := lipgloss.NewStyle().Background(theme.Resolve(t.Warning)).Foreground(theme.Resolve(t.Background)).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(theme.Resolve(t.Danger))
	okStyle := lipgloss.NewStyle().Foreground(theme.Resolve(t.Success)).Bold(true)
	dimStyle := common.DimStyle()

	var body []string
	body = append(body, titleStyle.Render(i18n.T("conns.dns.title")))
	body = append(body, dimStyle.Render(strings.Repeat("─", innerWidth)))

	// Domain input row
	in := d.input
	in.Width = max(12, innerWidth-lipgloss.Width(i18n.T("conns.dns.domain_label"))-4)
	body = append(body, labelStyle.Render(i18n.T("conns.dns.domain_label")+": ")+in.View())

	// Record type selector pills (wrapped if narrow, sharing coordinates with mouse clicks)
	var typeHits []dnsTypeHit
	typePrefix := i18n.T("conns.dns.type_label") + ": "
	line := labelStyle.Render(typePrefix)
	x := lipgloss.Width(typePrefix)
	rowIdx := len(body)
	for i, rt := range model.DNSRecordTypes {
		pillText := " " + rt + " "
		pillW := lipgloss.Width(pillText)
		if x > 0 && x+1+pillW > innerWidth {
			body = append(body, line)
			rowIdx = len(body)
			line = ""
			x = 0
		}
		if x > 0 {
			line += " "
			x++
		}
		// Panel has 1 border + 1 left padding = 2 columns inset; 1 top border row.
		typeHits = append(typeHits, dnsTypeHit{x: 2 + x, y: 1 + rowIdx, width: pillW, index: i})
		if i == d.typeIdx {
			line += activePill.Render(pillText)
		} else {
			line += inactivePill.Render(pillText)
		}
		x += pillW
	}
	body = append(body, line)
	body = append(body, dimStyle.Render(strings.Repeat("─", innerWidth)))

	// Result area
	var resultLines []string
	switch {
	case d.loading:
		resultLines = append(resultLines, primaryStyle.Render(i18n.Tf("conns.dns.querying", d.lastDomain, d.lastType)))
	case d.err != nil:
		resultLines = append(resultLines, errStyle.Render(d.err.Error()))
	case d.resp != nil:
		statusName := model.DNSStatusName(d.resp.Status)
		statusRendered := okStyle.Render(statusName)
		if d.resp.Status != 0 {
			statusRendered = errStyle.Render(statusName)
		}
		summary := labelStyle.Render(i18n.T("conns.dns.status")+": ") + statusRendered +
			dimStyle.Render(fmt.Sprintf("  (%s %s)", d.lastDomain, d.lastType))
		if d.resp.HasFakeIP() {
			summary += "  " + warnBadge.Render(" "+i18n.T("conns.dns.fakeip_hit")+" ")
		}
		resultLines = append(resultLines, summary, "")
		if len(d.resp.Answer) == 0 {
			resultLines = append(resultLines, dimStyle.Render(i18n.T("conns.dns.no_answers")))
		} else {
			header := fmt.Sprintf("%-24s %-6s %-6s %s",
				i18n.T("conns.dns.col_name"),
				i18n.T("conns.dns.col_type"),
				i18n.T("conns.dns.col_ttl"),
				i18n.T("conns.dns.col_data"),
			)
			resultLines = append(resultLines, primaryStyle.Render(common.TruncateDisplay(header, innerWidth)))
			for _, ans := range d.resp.Answer {
				typeName := model.DNSRecordTypeName(ans.Type)
				tag := model.ClassifyDNSIP(ans.Data)
				nameCol := common.TruncateDisplay(ans.Name, 24)
				row := fmt.Sprintf("%-24s %-6s %-6s %s", nameCol, typeName, fmt.Sprintf("%ds", ans.TTL), ans.Data)
				if tag != "" {
					badge := " [" + tag + "]"
					if tag == "fake-ip" {
						row += " " + warnBadge.Render(" Fake-IP ")
					} else {
						row += errStyle.Render(badge)
					}
				}
				resultLines = append(resultLines, common.TruncateDisplay(row, innerWidth))
			}
		}
	default:
		resultLines = append(resultLines, dimStyle.Render(i18n.T("conns.dns.empty_hint")))
	}

	maxResultLines := max(4, viewportHeight-len(body)-3)
	scroll := d.scroll
	maxScroll := max(0, len(resultLines)-maxResultLines)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	end := min(len(resultLines), scroll+maxResultLines)
	body = append(body, resultLines[scroll:end]...)

	panel := common.RenderCard(panelWidth, t.Primary, body)
	return dialog.New(panel, width, viewportHeight).Clipped(), dnsPopupLayout{typeHits: typeHits}
}

func (s State) clickDNS(pageX, pageY, pageWidth, pageHeight int) (State, tea.Cmd) {
	popup, layout := s.dns.popup(pageWidth, pageHeight)
	left, top, right, bottom := popup.Bounds()
	if pageX < left || pageX >= right || pageY < top || pageY >= bottom {
		s.closeDNSQuery()
		return s, nil
	}
	localX, localY := pageX-left, pageY-top
	for _, hit := range layout.typeHits {
		if localY == hit.y && localX >= hit.x && localX < hit.x+hit.width {
			s.dns.typeIdx = hit.index
			if strings.TrimSpace(s.dns.input.Value()) != "" {
				return s.submitDNSQuery()
			}
			return s, nil
		}
	}
	return s, nil
}
