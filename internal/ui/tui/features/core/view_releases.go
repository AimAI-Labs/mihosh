package core

import (
	"fmt"
	"strings"

	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type releaseControl struct {
	actionHit
	action string
}

func (s State) releaseBadge(r coreinfra.Release) string {
	if s.Status.Mode == "managed" && s.Status.Version == r.Version() {
		return i18n.T("core.catalog.current")
	}
	if s.releaseInstalled(r.Version()) {
		return i18n.T("core.catalog.installed")
	}
	if r.Prerelease {
		return i18n.T("core.catalog.prerelease")
	}
	return i18n.T("core.catalog.stable")
}

func (s State) releaseColor(r coreinfra.Release) lipgloss.Color {
	t := theme.Current()
	if s.Status.Mode == "managed" && s.Status.Version == r.Version() {
		return t.Success
	}
	if s.releaseInstalled(r.Version()) {
		return t.Info
	}
	if r.Prerelease {
		return t.Warning
	}
	return t.Foreground
}

func releaseMetadata(r coreinfra.Release) (string, string) {
	date := i18n.T("core.catalog.date_unknown")
	if !r.PublishedAt.IsZero() {
		date = r.PublishedAt.Format("2006-01-02")
	}
	size := i18n.T("core.catalog.size_unknown")
	if len(r.Assets) > 0 && r.Assets[0].Size > 0 {
		size = fmt.Sprintf("%.1f MiB", float64(r.Assets[0].Size)/(1<<20))
	}
	return date, size
}

func releaseInfo(r coreinfra.Release) string {
	date, size := releaseMetadata(r)
	return date + " · " + size
}

type releaseColumns struct {
	version, status, date, size int
}

// Measure only visible rows: the catalogue can contain all upstream history.
func (s State) releaseColumns(start, end, width int) releaseColumns {
	var c releaseColumns
	// Keep status columns steady when the current release scrolls out of view.
	statusWidth := 0
	for _, key := range []string{"current", "installed", "prerelease", "stable"} {
		statusWidth = max(statusWidth, lipgloss.Width("["+i18n.T("core.catalog."+key)+"]"))
	}
	if s.Status.Mode == "managed" {
		c.version = lipgloss.Width(s.Status.Version)
	}
	for i := start; i < end; i++ {
		r, ok := s.releaseAt(i)
		if !ok {
			continue
		}
		date, size := releaseMetadata(r)
		c.version = max(c.version, lipgloss.Width(r.Version()))
		c.status = max(c.status, lipgloss.Width("["+s.releaseBadge(r)+"]"))
		c.date = max(c.date, lipgloss.Width(date))
		c.size = max(c.size, lipgloss.Width(size))
	}
	c.status = max(c.status, min(statusWidth, max(0, width-4-c.version)))
	if 2+c.version+2+c.status+2+c.date+2+c.size > width {
		c.size = 0
	}
	if 2+c.version+2+c.status+2+c.date > width {
		c.date = 0
	}
	c.version = max(1, min(c.version, width-2-2-c.status))
	return c
}

func (s State) releaseRow(r coreinfra.Release, marker string, c releaseColumns, width int) string {
	date, size := releaseMetadata(r)
	row := marker + common.PadString(ansi.Truncate(r.Version(), c.version, "…"), c.version) + "  " +
		common.PadString("["+s.releaseBadge(r)+"]", c.status)
	if c.date > 0 {
		row += "  " + common.PadString(date, c.date)
	}
	if c.size > 0 {
		row += "  " + common.PadString(size, c.size)
	}
	return ansi.Truncate(row, width, "…")
}

func (s State) releaseCaption() string {
	if !s.catalog.loaded {
		return i18n.T("core.catalog.cached")
	}
	caption := i18n.Tf("core.catalog.loaded", s.releaseCount())
	status := ""
	if !s.catalog.page.HasNext {
		status = "core.catalog.complete"
	}
	if s.catalog.failed {
		status = "core.catalog.load_failed"
	} else if s.Busy && s.task == "releases" {
		status = "core.catalog.loading"
	}
	if status != "" {
		caption += " · " + i18n.T(status)
	}
	return caption
}

func (s State) releaseStart(visible int) int {
	start := s.catalog.scroll
	if s.followSelection {
		if s.selected < start {
			start = s.selected
		}
		if s.selected >= start+visible {
			start = s.selected - visible + 1
		}
	}
	return max(0, min(start, s.releaseCount()-visible))
}

func (s State) releaseLayout(width, height int) pageLayout {
	var result pageLayout
	t := theme.Current()
	selected := lipgloss.NewStyle().Foreground(theme.Resolve(t.Primary)).Background(theme.Resolve(t.Selected)).Bold(true)
	visible := max(1, height-3)
	result.listRows = visible
	start := s.releaseStart(visible)
	var rows []string
	count := s.releaseCount()
	if count == 0 {
		rows = append(rows, strings.Split(wrap(i18n.T("core.catalog.empty"), width-4), "\n")...)
	}
	rowWidth := width - 4
	var scrollbar string
	if rowWidth >= 22 {
		scrollbar = common.BuildVerticalScrollbar(visible, count, start, true)
		if scrollbar != "" {
			rowWidth -= 2
		}
	}
	columns := s.releaseColumns(start, min(count, start+visible), rowWidth)
	for i := start; i < min(count, start+visible); i++ {
		r, ok := s.releaseAt(i)
		if !ok {
			continue
		}
		marker := "  "
		if s.tab == 1 && i == s.selected {
			marker = "› "
		}
		label := s.releaseRow(r, marker, columns, rowWidth)
		rc := theme.Resolve(s.releaseColor(r))
		label = lipgloss.NewStyle().Foreground(rc).Render(label)
		if s.tab == 1 && i == s.selected {
			label = selected.Foreground(rc).Render(common.PadString(label, rowWidth))
		}
		result.releases = append(result.releases, actionHit{x: 1, y: len(rows) + 1, width: width - 2, height: 1, index: i})
		rows = append(rows, label)
	}
	if scrollbar != "" {
		tracks := strings.Split(scrollbar, "\n")
		for i := range min(len(rows), len(tracks)) {
			rows[i] = common.PadString(rows[i], rowWidth) + " " + tracks[i]
		}
	}
	network := ansi.Truncate("  "+i18n.T("core.action.network")+" [e]", width-4, "…")
	result.controls = append(result.controls, releaseControl{
		actionHit: actionHit{x: 2, y: len(rows) + 1, width: lipgloss.Width(network), height: 1},
		action:    "network",
	})
	rows = append(rows, common.HighlightStyle().Render(network))
	result.content = panel(i18n.T("core.catalog.title"), strings.Join(rows, "\n"), width)
	return result
}

func (s State) clickReleasePage(page pageLayout, x, y int) (State, tea.Cmd) {
	for _, hit := range page.releases {
		if x >= hit.x && x < hit.x+hit.width && y >= hit.y && y < hit.y+hit.height {
			if s.tab != 1 {
				s = s.selectTab(1)
			}
			next := s.selectRelease(hit.index)
			return next.maybeLoadMore()
		}
	}
	return s, nil
}

func (s State) scrollReleasePage(msg messages.PageMouseScrollMsg) (State, tea.Cmd) {
	page := s.layout(msg.Width, max(1, msg.Height-footerHeight))
	s.catalog.scroll = s.releaseStart(page.listRows)
	if msg.Up {
		s.catalog.scroll = max(0, s.catalog.scroll-1)
	} else {
		s.catalog.scroll = min(max(0, s.releaseCount()-page.listRows), s.catalog.scroll+1)
	}
	s.followSelection = false
	if !msg.Up && s.catalog.scroll+page.listRows >= s.releaseCount()-2 {
		if !s.catalog.failed && s.catalog.page.HasNext {
			return s.releaseAction("next")
		}
	}
	return s, nil
}
