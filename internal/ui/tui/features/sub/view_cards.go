package sub

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func renderList(state PageState, grid common.CardGrid) string {
	if len(state.FilteredIdx) == 0 {
		return common.MutedStyle().Render(i18n.T("sub.empty"))
	}
	list := grid.Render(func(i, width, height int) string {
		p := state.Subs[state.FilteredIdx[i]]
		active, updating, selected := p.UID == state.ActiveUID, p.UID == state.UpdatingUID, i == grid.Window.Selected
		if state.ListMode {
			return renderSubRow(p, i+1, active, updating, selected, width)
		}
		return renderSubCard(p, active, updating, selected, width, height)
	})
	list = lipgloss.PlaceHorizontal(state.Width-subScrollWidth, lipgloss.Left, list)
	bar := common.BuildVerticalScrollbar((grid.Rows.End-grid.Rows.Start)*grid.CardHeight,
		grid.TotalRows*grid.CardHeight, grid.Rows.Start*grid.CardHeight, false)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, bar)
}

func renderSubCard(p profile.Profile, active, updating, selected bool, width, height int) string {
	palette := theme.Current()
	border, titleColor := palette.Border, palette.Bright
	if selected {
		border, titleColor = palette.Primary, palette.Primary
	}
	if active {
		border = palette.Success
	}
	title := strings.Join(strings.Fields(ansi.Strip(p.Name)), " ")
	title = lipgloss.NewStyle().Foreground(theme.Resolve(titleColor)).Bold(true).Render(title)
	if height < 3 {
		line := lipgloss.NewStyle().Width(width).Render(ansi.Truncate(title, width, "…"))
		return lipgloss.PlaceVertical(height, lipgloss.Top, line)
	}

	kind, source := i18n.T("sub.card.local"), p.Source.Path
	action := ""
	if p.Source.Kind == profile.SourceRemote {
		kind, source = i18n.T("sub.card.remote"), ""
		if u, err := url.Parse(p.Source.URL); err == nil {
			source = u.Hostname()
		}
		action = lipgloss.NewStyle().Foreground(theme.Resolve(palette.Primary)).Render("↻")
		if updating {
			action = lipgloss.NewStyle().Foreground(theme.Resolve(palette.Info)).Render("⟳")
		}
	}
	if source != "" {
		kind += " · " + strings.Join(strings.Fields(ansi.Strip(source)), " ")
	}
	updated := i18n.T("sub.never_updated")
	if p.UpdatedAt > 0 {
		updated = i18n.Tf("sub.card.updated", relativeTime(p.UpdatedAt))
		updated += " · " + time.Unix(p.UpdatedAt, 0).Local().Format("2006-01-02 15:04")
	}
	if updating {
		updated = i18n.T("sub.updating")
	}

	inner := max(1, width-4)
	traffic, expiry, transfer, progress := "", "", "", ""
	if u := p.Usage; p.Source.Kind == profile.SourceRemote && u != nil {
		var details []string
		if u.TotalBytes > 0 {
			ratio := min(1, max(0, float64(u.UsedBytes)/float64(u.TotalBytes)))
			traffic = i18n.Tf("sub.card.traffic", utils.FormatBytes(u.UsedBytes), utils.FormatBytes(u.TotalBytes)) +
				fmt.Sprintf(" · %.0f%%", ratio*100)
			details = append(details, i18n.Tf("sub.card.remaining", utils.FormatBytes(max(0, u.TotalBytes-u.UsedBytes))))
			filled := int(float64(inner) * ratio)
			progress = lipgloss.NewStyle().Foreground(theme.Resolve(palette.Primary)).Render(strings.Repeat("━", filled)) +
				lipgloss.NewStyle().Foreground(theme.Resolve(palette.Border)).Render(strings.Repeat("─", inner-filled))
		} else if u.UsedBytes > 0 {
			traffic = i18n.Tf("sub.card.used", utils.FormatBytes(u.UsedBytes))
		}
		if u.UploadBytes != nil {
			details = append(details, "↑"+utils.FormatBytes(*u.UploadBytes))
		}
		if u.DownloadBytes != nil {
			details = append(details, "↓"+utils.FormatBytes(*u.DownloadBytes))
		}
		transfer = strings.Join(details, " · ")
		if u.ExpiresAt > 0 {
			expires := time.Unix(u.ExpiresAt, 0).Local()
			if expires.Before(time.Now()) {
				expiry = common.WarningStyle().Render(i18n.Tf("sub.card.expired", expires.Format("2006-01-02 15:04")))
			} else {
				expiry = i18n.Tf("sub.card.expires", expires.Format("2006-01-02 15:04"))
			}
		}
	}
	lines := []string{common.CardHeading(inner, title, action), kind, updated, traffic, expiry, transfer, progress}
	return common.RenderCard(width, border, lines[:height-2])
}
