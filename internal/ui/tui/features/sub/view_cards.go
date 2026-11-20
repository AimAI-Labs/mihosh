package sub

import (
	"net/url"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func renderList(state PageState, layout subListLayout) string {
	if len(state.FilteredIdx) == 0 {
		return common.MutedStyle().Render(i18n.T("sub.empty"))
	}
	width := state.Width - subScrollWidth
	var cards []string
	for i := layout.window.Start; i < layout.window.End; i++ {
		index := state.FilteredIdx[i]
		if index < 0 || index >= len(state.Subs) {
			continue
		}
		p := state.Subs[index]
		cards = append(cards, renderSubCard(p, p.UID == state.ActiveUID, p.UID == state.UpdatingUID,
			i == layout.window.Selected, width, layout.cardHeight))
	}
	list := strings.Join(cards, "\n")
	bar := common.BuildVerticalScrollbar(len(cards)*layout.cardHeight,
		len(state.FilteredIdx)*layout.cardHeight, layout.window.Start*layout.cardHeight, false)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, bar)
}

func renderSubCard(p profile.Profile, active, updating, selected bool, width, height int) string {
	title := strings.Join(strings.Fields(ansi.Strip(p.Name)), " ")
	if active {
		title = common.SymbolCheck + title
	}
	if updating {
		title = "⟳ " + title
	}
	border := common.Dim()
	if selected {
		border = common.Active()
	}
	if width < 24 || height < 3 {
		line := lipgloss.NewStyle().Width(max(1, width)).Foreground(border).Bold(true).
			Render(ansi.Truncate(title, max(1, width), "…"))
		return lipgloss.PlaceVertical(height, lipgloss.Top, line)
	}

	kind, source := i18n.T("sub.card.local"), p.Source.Path
	if p.Source.Kind == profile.SourceRemote {
		kind, source = i18n.T("sub.card.remote"), ""
		if u, err := url.Parse(p.Source.URL); err == nil {
			source = u.Hostname()
		}
	}
	if source != "" {
		kind += " · " + strings.Join(strings.Fields(ansi.Strip(source)), " ")
	}
	updated := i18n.T("sub.never_updated")
	if p.UpdatedAt > 0 {
		updated = i18n.Tf("sub.card.updated", relativeTime(p.UpdatedAt))
	}
	if updating {
		updated = i18n.T("sub.updating")
	}
	inner := width - 4
	traffic, expiry, progress := "", "", ""
	if u := p.Usage; p.Source.Kind == profile.SourceRemote && u != nil {
		if u.TotalBytes > 0 {
			traffic = i18n.Tf("sub.card.traffic", utils.FormatBytes(u.UsedBytes), utils.FormatBytes(u.TotalBytes))
			barWidth := min(24, inner)
			filled := int(float64(barWidth) * min(1, float64(u.UsedBytes)/float64(u.TotalBytes)))
			progress = common.SuccessStyle().Render(strings.Repeat("━", filled)) +
				common.DimStyle().Render(strings.Repeat("─", barWidth-filled))
		} else if u.UsedBytes > 0 {
			traffic = i18n.Tf("sub.card.used", utils.FormatBytes(u.UsedBytes))
		}
		if u.ExpiresAt > 0 {
			expiry = i18n.Tf("sub.card.expires", time.Unix(u.ExpiresAt, 0).Local().Format("2006-01-02 15:04"))
		}
	}
	lines := []string{kind, updated, traffic, expiry, progress}
	lines = lines[:height-2]
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, inner, "…")
	}
	return common.RenderBorderedPanel(title, strings.Join(lines, "\n"), width, border, common.Bright())
}
