package connections

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
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

func layoutConnectionList(state PageState) connectionListLayout {
	parts := []string{RenderConnModeSwitchComponent(state.ViewMode, state.Width, state.InlineMode, state.Paused, state.PausedUpdates), ""}
	filterLine := ""
	if state.FilterMode {
		filterLine = state.FilterInput
	} else if state.FilterText != "" {
		filterLine = common.DimStyle().Render(i18n.Tf("conns.filter_inactive", state.FilterText))
	}
	if filterLine != "" {
		parts = append(parts, filterLine)
	}
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(common.TokyoBlue())
	parts = append(parts, components.RenderTableHeader(headerStyle, state.Width),
		common.TableBorderStyle().Render(strings.Repeat("─", max(state.Width-2, 1))))
	layout := connectionListLayout{
		header:      strings.Join(parts, "\n"),
		connections: filterConnections(connectionsByViewMode(state), state.FilterText),
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
