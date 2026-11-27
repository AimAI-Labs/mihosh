package connections

import (
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// connectionListLayout owns the visible window and page-local row coordinates.
type connectionListLayout struct {
	header       string
	connections  []model.Connection
	selected     int
	scrollTop    int
	end          int
	rowsY        int
	dataY        int
	listHeight   int
	detailHeight int
}

func resolvePageQuery(state PageState) ConnQuery {
	q := state.Query
	if q.Text == "" && state.FilterText != "" {
		q.Text = state.FilterText
	}
	return q
}

func renderQuerySummary(state PageState, q ConnQuery) string {
	var chips []string
	accent := lipgloss.NewStyle().Foreground(theme.Resolve(theme.Current().Primary)).Bold(true)
	info := lipgloss.NewStyle().Foreground(theme.Resolve(theme.Current().Info)).Bold(true)
	muted := common.DimStyle()

	if q.SortBy != ConnSortDefault {
		dir := "↑"
		if q.SortDesc {
			dir = "↓"
		}
		chips = append(chips, accent.Render(i18n.T("conns.sort.label")+": "+q.SortBy.Label()+" "+dir))
	}
	if q.Protocol != "" {
		chips = append(chips, info.Render(i18n.T("conns.filter.proto_label")+": "+strings.ToUpper(q.Protocol)))
	}
	if q.Field != ConnFilterAll {
		chips = append(chips, info.Render(i18n.T("conns.filter.field_label")+": "+q.Field.Label()))
	}

	var filterPart string
	if state.FilterMode {
		filterPart = state.FilterInput
	} else if q.Text != "" {
		filterPart = muted.Render(i18n.Tf("conns.filter_inactive", q.Text))
	}
	if filterPart != "" {
		chips = append(chips, filterPart)
	}
	return strings.Join(chips, muted.Render("  │  "))
}

func layoutConnectionList(state PageState) connectionListLayout {
	q := resolvePageQuery(state)
	parts := []string{RenderConnModeSwitchComponent(state.ViewMode, state.Width, state.InlineMode, state.Paused, state.PausedUpdates), ""}
	if bar := renderQuerySummary(state, q); bar != "" {
		parts = append(parts, ansi.Truncate(bar, max(1, state.Width), "…"))
	}
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(common.TokyoBlue())
	parts = append(parts, components.RenderTableHeader(headerStyle, state.Width),
		common.TableBorderStyle().Render(strings.Repeat("─", max(state.Width-2, 1))))
	layout := connectionListLayout{
		header:      strings.Join(parts, "\n"),
		connections: queryConnections(connectionsByViewMode(state), q, time.Now()),
	}
	layout.rowsY = lipgloss.Height(layout.header)
	// Keep the existing three-row footer/hint budget and inline split.
	maxDisplay := max(1, state.Height-layout.rowsY-3)
	total := len(layout.connections)
	if state.InlineMode && total > 0 {
		layout.detailHeight = max(5, maxDisplay/2)
		maxDisplay = max(3, maxDisplay-layout.detailHeight)
	}
	window := common.ResolveListWindow(total, state.SelectedIndex, state.ScrollTop, maxDisplay)
	layout.selected, layout.scrollTop, layout.end = window.Selected, window.Start, window.End
	layout.dataY = layout.rowsY
	rowCount := max(1, layout.end-layout.scrollTop)
	if layout.scrollTop > 0 {
		layout.dataY++
		rowCount++
	}
	if layout.end < total {
		rowCount++
	}
	layout.listHeight = max(maxDisplay, rowCount)
	return layout
}

func (l connectionListLayout) hit(x, y, width int) MouseHit {
	if x >= 0 && x < width {
		if y >= l.dataY && y < l.dataY+l.end-l.scrollTop {
			return MouseHit{Target: MouseTargetConnection, Index: l.scrollTop + y - l.dataY}
		}
		if y >= l.rowsY+l.listHeight && y < l.rowsY+l.listHeight+l.detailHeight {
			return MouseHit{Target: MouseTargetInlineDetail, Index: -1}
		}
	}
	return MouseHit{Target: ConnectionsMouseTargetNone, Index: -1}
}
