package core

import (
	"fmt"
	"runtime"
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

func (s State) releaseDetails(width int, compact bool, budget int) (string, []releaseControl) {
	t := theme.Current()
	muted := lipgloss.NewStyle().Foreground(t.Muted)
	r, ok := s.selectedRelease()
	if !ok {
		return panel(i18n.T("core.catalog.selection"), wrap(i18n.T("core.catalog.empty"), width-4), width), nil
	}
	dense := budget < 12 || (compact && budget < 9)
	heading := r.Version() + " · " + s.releaseBadge(r)
	if dense {
		heading += " · " + releaseInfo(r)
	}
	rows := []string{lipgloss.NewStyle().Foreground(s.releaseColor(r)).Bold(true).Render(ansi.Truncate(heading, width-4, "…"))}
	if !dense {
		rows = append(rows, releaseInfo(r))
	}
	if !compact && !dense {
		rows = append(rows, i18n.T("core.catalog.platform")+": "+runtime.GOOS+"/"+runtime.GOARCH)
		if len(r.Assets) > 0 {
			rows = append(rows, strings.Split(muted.Render(wrap(r.Assets[0].Name, width-4)), "\n")...)
		}
	}
	if !dense {
		rows = append(rows, "")
	}
	var controls []releaseControl
	if budget < 5 {
		return panel(i18n.T("core.catalog.selection"), strings.Join(rows, "\n"), width), nil
	}
	button := func(action, text string, enabled bool) {
		style := muted
		if enabled && !s.Busy {
			style = lipgloss.NewStyle().Foreground(t.Primary).Background(t.Selected).Bold(true)
			controls = append(controls, releaseControl{actionHit: actionHit{x: 1, y: len(rows) + 1, width: width - 2, height: 1}, action: action})
		}
		rows = append(rows, style.Render(ansi.Truncate(text, width-4, "…")))
	}
	installed := s.releaseInstalled(r.Version())
	verified := false
	if len(r.Assets) > 0 {
		_, err := r.TrustedDigest(r.Assets[0], coreinfra.PinnedDigests())
		verified = err == nil
	}
	download := "core.catalog.download"
	if installed {
		download = "core.catalog.downloaded"
	}
	button("install", i18n.T(download), !installed && verified)
	button("activate", i18n.T("core.catalog.activate"), installed && !(s.Status.Mode == "managed" && s.Status.Version == r.Version()))
	if !dense {
		rows = append(rows, strings.Split(muted.Render(wrap(i18n.T("core.catalog.download_hint"), width-4)), "\n")...)
	}
	if !verified && !installed {
		warning := i18n.T("core.catalog.unverified")
		if dense {
			warning = ansi.Truncate(warning, width-4, "…")
		}
		lines := strings.Split(lipgloss.NewStyle().Foreground(t.Warning).Render(wrap(warning, width-4)), "\n")
		rows = append(rows, lines[:min(len(lines), max(0, budget-len(rows)-2))]...)
	}
	return panel(i18n.T("core.catalog.selection"), strings.Join(rows, "\n"), width), controls
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

func (s State) releaseLayout(width, height int, parts []string, result pageLayout) pageLayout {
	t := theme.Current()
	muted := lipgloss.NewStyle().Foreground(t.Muted)
	selected := lipgloss.NewStyle().Foreground(t.Primary).Background(t.Selected).Bold(true)
	leftWidth, rightWidth := width, width
	wide := width >= 100
	if wide {
		leftWidth = width * 43 / 100
		rightWidth = width - leftWidth
	}
	top := lipgloss.Height(strings.Join(parts, "\n"))
	caption := i18n.Tf("core.catalog.loaded", s.releaseCount())
	if s.catalog.loaded {
		status := "core.catalog.more_hint"
		if !s.catalog.page.HasNext {
			status = "core.catalog.complete"
		}
		if s.catalog.failed {
			status = "core.catalog.load_failed"
		} else if s.Busy && s.task == "releases" {
			status = "core.catalog.loading"
		}
		caption += " · " + i18n.T(status)
	}
	if !s.catalog.loaded {
		caption = i18n.T("core.catalog.fallback")
	}
	footerHeight := 3
	detailBudget := height - top - footerHeight
	if !wide {
		detailBudget -= 4 // one visible release and a separating line
	}
	left, controls := s.releaseDetails(leftWidth, !wide, detailBudget)
	listHeight := height - top - footerHeight
	listY, listX := top, 0
	if !wide {
		listY += lipgloss.Height(left) + 1
		listHeight -= lipgloss.Height(left) + 1
	}
	visible := max(1, listHeight-2)
	result.listRows = visible
	start := s.releaseStart(visible)
	var rows []string
	count := s.releaseCount()
	if count == 0 {
		rows = append(rows, strings.Split(wrap(i18n.T("core.catalog.empty"), rightWidth-4), "\n")...)
	}
	if wide {
		listX = leftWidth
	}
	columns := s.releaseColumns(start, min(count, start+visible), rightWidth-4)
	for i := start; i < min(count, start+visible); i++ {
		r, ok := s.releaseAt(i)
		if !ok {
			continue
		}
		marker := "  "
		if i == s.selected {
			marker = "› "
		}
		label := s.releaseRow(r, marker, columns, rightWidth-4)
		label = lipgloss.NewStyle().Foreground(s.releaseColor(r)).Render(label)
		if i == s.selected {
			label = selected.Foreground(s.releaseColor(r)).Render(common.PadString(label, rightWidth-4))
		}
		result.releases = append(result.releases, actionHit{x: listX + 1, y: listY + len(rows) + 1, width: rightWidth - 2, height: 1, index: i})
		rows = append(rows, label)
	}
	right := panel(i18n.T("core.catalog.title"), strings.Join(rows, "\n"), rightWidth)
	if wide {
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, left, right))
	} else {
		parts = append(parts, left, "", right)
	}
	for i := range controls {
		controls[i].y += top
	}
	result.controls = controls
	// Append to one continuous list; loading/retry controls remain below the viewport.
	navY := lipgloss.Height(strings.Join(parts, "\n")) + 1
	more, refresh := i18n.T("core.catalog.load_more"), i18n.T("core.catalog.refresh")
	moreAction := "next"
	if s.catalog.failed {
		more, moreAction = i18n.T("core.catalog.retry"), "retry"
	}
	var buttons []string
	x := 0
	for _, control := range []struct {
		text, action string
		enabled      bool
	}{{more, moreAction, s.catalog.page.HasNext || s.catalog.failed}, {refresh, "refresh", true}} {
		style := muted
		if control.enabled && !s.Busy {
			style = lipgloss.NewStyle().Foreground(t.Primary)
			result.controls = append(result.controls, releaseControl{actionHit: actionHit{x: x, y: navY, width: lipgloss.Width(control.text), height: 1}, action: control.action})
		}
		buttons = append(buttons, style.Render(control.text))
		x += lipgloss.Width(control.text) + 2
	}
	parts = append(parts, "", strings.Join(buttons, "  "), muted.Render(caption))
	result.content = strings.Join(parts, "\n")
	return result
}

func (s State) clickReleasePage(page pageLayout, x, y int) (State, tea.Cmd) {
	if s.Busy && s.task != "releases" {
		return s, nil
	}
	for _, hit := range page.releases {
		if x >= hit.x && x < hit.x+hit.width && y >= hit.y && y < hit.y+hit.height {
			next := s.selectRelease(hit.index)
			return next.maybeLoadMore()
		}
	}
	for _, control := range page.controls {
		if x >= control.x && x < control.x+control.width && y == control.y {
			return s.releaseAction(control.action)
		}
	}
	return s, nil
}

func (s State) scrollReleasePage(msg messages.PageMouseScrollMsg) (State, tea.Cmd) {
	if s.Busy && s.task != "releases" {
		return s, nil
	}
	page := s.layout(msg.Width, max(1, msg.Height-1))
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
