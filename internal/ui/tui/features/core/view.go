package core

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The view and mouse routing share these bounds, including wrapped descriptions.
type actionHit struct {
	x, y, width, height int
	index               int
}

type pageLayout struct {
	content  string
	actions  []actionHit
	tabs     []actionHit
	focusY   int
	releases []actionHit
	controls []releaseControl
	listRows int
}

func wrap(text string, width int) string {
	return ansi.Wrap(text, max(1, width), "")
}

func panel(title, body string, width int) string {
	return common.RenderTokyoPanel(title, body, max(24, width))
}

func (s State) actionPanel(width int, serviceMenu bool) (string, []actionHit) {
	t := theme.Current()
	muted := lipgloss.NewStyle().Foreground(t.Muted)
	selected := lipgloss.NewStyle().Foreground(t.Primary).Background(t.Selected).Bold(true)
	var rows []string
	var hits []actionHit
	items := s.actions()
	if serviceMenu {
		items = nil
		for _, action := range serviceActions {
			items = append(items, actionItem{action: action})
		}
	}
	for i, item := range items {
		prefix, titleKey, descKey := "  ", "core.action."+item.action, "core.desc."+item.action
		if serviceMenu {
			titleKey, descKey = "core.service."+item.action, "core.service.desc."+item.action
		}
		if i == s.selected {
			prefix = "› "
		}
		label := prefix + i18n.T(titleKey)
		if item.key != "" && item.key != "enter" {
			label += " [" + item.key + "]"
		}
		label = ansi.Truncate(label, max(1, width-4), "…")
		if i == s.selected {
			label = selected.Render(common.PadString(label, max(1, width-4)))
		}
		desc := "  " + strings.ReplaceAll(wrap(i18n.T(descKey), width-6), "\n", "\n  ")
		hit := actionHit{x: 1, y: len(rows) + 1, width: width - 2, height: 1 + lipgloss.Height(desc), index: i}
		hits = append(hits, hit)
		rows = append(rows, label)
		rows = append(rows, strings.Split(muted.Render(desc), "\n")...)
	}
	title := "core.tab." + tabs[s.tab]
	if serviceMenu {
		title = "core.action.service"
	}
	return panel(i18n.T(title), strings.Join(rows, "\n"), width), hits
}

func (s State) overview(width int) string {
	t := theme.Current()
	status, color := "core.state.external", t.Info
	if s.Status.Mode == "managed" {
		status, color = "core.state.stopped", t.Warning
		if s.Status.Running {
			status, color = "core.state.running", t.Success
		}
	} else if s.connected {
		status, color = "core.state.connected", t.Success
	}
	if s.Status.Version == "" && s.Status.Mode != "managed" && !s.connected {
		status = "core.state.welcome"
	}
	version := s.Status.Version
	if version == "" {
		version = i18n.T("core.not_installed")
	}
	mode := "core.mode.external"
	if s.Status.Mode == "managed" {
		mode = "core.mode.managed"
	} else if status == "core.state.welcome" {
		mode = "core.mode.choose"
	}
	rows := []string{
		lipgloss.NewStyle().Foreground(color).Bold(true).Render("● " + i18n.T(status)),
		i18n.T(mode),
	}
	if s.Status.Mode == "managed" {
		rows = append(rows, i18n.T("core.version")+": "+version)
	}
	if s.connectionChecked && !s.connected && s.Status.Mode == "managed" && s.Status.Running {
		rows = append(rows, lipgloss.NewStyle().Foreground(t.Warning).Render(wrap(i18n.T("core.connection_unavailable"), width-4)))
	}
	if s.tab == 0 {
		value := func(text string) string {
			if text == "" {
				return i18n.T("core.unknown_version")
			}
			return text
		}
		if s.Status.Mode != "managed" {
			version = value(s.externalVersion)
		}
		rows = rows[:1]
		for _, field := range []struct{ key, value string }{
			{"core.version", version},
			{"core.binary_path", value(s.Status.BinaryPath)},
			{"core.config_path", value(s.Status.ConfigPath)},
			{"core.controller_address", value(s.Status.Controller)},
		} {
			rows = append(rows, wrap(i18n.T(field.key)+": "+field.value, width-4))
		}
		if s.Status.MixedPort > 0 {
			rows = append(rows, i18n.T("core.mixed_port_label")+": "+fmt.Sprint(s.Status.MixedPort))
		}
		if s.Status.Mode == "managed" {
			runtime := "core.runtime.session"
			if s.Status.Runtime == "service" {
				runtime = "core.runtime.service"
			}
			rows = append(rows, i18n.T("core.runtime")+": "+i18n.T(runtime))
		}
	} else if s.tab == 1 {
		tag := s.Release.Tag
		if tag == "" {
			tag = i18n.T("core.unknown_version")
		}
		rows = append(rows, "", i18n.T("core.available")+": "+tag,
			wrap(i18n.Tf("core.installed_count", len(s.Status.Installed)), width-4))
	} else {
		runtime := "core.runtime.session"
		if s.Status.Runtime == "service" {
			runtime = "core.runtime.service"
		}
		setting := func(label string, enabled bool) string {
			value := "core.disabled"
			if enabled {
				value = "core.enabled"
			}
			return i18n.T(label) + ": " + i18n.T(value)
		}
		rows = append(rows, "", i18n.T(runtime), setting("core.autostart", s.Status.AutoStart),
			setting("core.system_proxy", s.Status.SystemProxy), setting("core.tun_label", s.Status.TUN),
			"", wrap(i18n.T("core.hint.network"), width-4))
		if s.Status.RecoveryPending {
			rows = append(rows, lipgloss.NewStyle().Foreground(t.Warning).Render(wrap(i18n.T("core.hint.recovery"), width-4)))
		}
	}
	return panel(i18n.T("core.overview"), strings.Join(rows, "\n"), width)
}

func (s State) layout(width, height int) pageLayout {
	width = max(24, width)
	t := theme.Current()
	muted := lipgloss.NewStyle().Foreground(t.Muted)
	if s.Editing && s.form != "service" {
		return s.formLayout(width)
	}
	if s.Editing {
		content, hits := s.actionPanel(width, true)
		return pageLayout{content: content, actions: hits}
	}
	var result pageLayout
	var labels []string
	x := 2
	for i, tab := range tabs {
		label := " " + i18n.T("core.tab."+tab) + " "
		result.tabs = append(result.tabs, actionHit{x: x, y: 1, width: lipgloss.Width(label), height: 1, index: i})
		x += lipgloss.Width(label) + 1
		style := lipgloss.NewStyle().Foreground(t.Muted)
		if s.tab == i {
			style = style.Foreground(t.Primary).Background(t.Selected).Bold(true)
		}
		labels = append(labels, style.Render(label))
	}
	parts := []string{panel(i18n.T("core.heading"), strings.Join(labels, "│"), width), ""}
	if s.Busy && !downloadTask(s.task) && !(s.tab == 1 && s.task == "releases" && s.catalog.loaded) {
		progress := s.progressText(width)
		parts = append(parts, panel(i18n.T("core.task"), wrap(progress, width-4)+"\n"+muted.Render(i18n.T("core.cancel")), width), "")
	}
	if s.Err != nil && !s.taskDialog {
		detail := strings.Split(wrap(s.Err.Error(), width-4), "\n")
		if s.tab == 1 && len(detail) > 2 {
			detail = detail[:2]
			detail[1] = ansi.Truncate(detail[1], width-5, "") + "…"
		} else if len(detail) > 6 {
			detail = append(detail[:6], "…")
		}
		parts = append(parts, common.RenderBorderedPanel(i18n.T("core.error_title"), strings.Join(detail, "\n"), width, t.Danger, t.Danger), "")
	}
	if s.tab == 1 {
		return s.releaseLayout(width, height, parts, result)
	}
	top := lipgloss.Height(strings.Join(parts, "\n"))
	leftWidth, rightWidth := width, width
	if width >= 100 {
		leftWidth = width * 43 / 100
		rightWidth = width - leftWidth
	}
	left := s.overview(leftWidth)
	right, hits := s.actionPanel(rightWidth, false)
	for i := range hits {
		hits[i].y += top
		if width >= 100 {
			hits[i].x += leftWidth
		} else {
			hits[i].y += lipgloss.Height(left) + 1
		}
	}
	result.actions = hits
	if width >= 100 {
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, left, right))
	} else {
		parts = append(parts, left, "", right)
	}
	result.content = strings.Join(parts, "\n")
	return result
}

func (s State) formLayout(width int) pageLayout {
	t := theme.Current()
	labels := map[string][]string{
		"network":  {"core.proxy", "core.accelerators", "core.download_timeout"},
		"profile":  {"core.profile_name", "core.profile_source"},
		"version":  {"core.version_input"},
		"import":   {"cli.core.file", "core.version_input", "cli.core.sha256"},
		"external": {"core.controller", "core.secret", "core.mixed_port"},
		"ports":    {"core.controller_port", "core.mixed_port"},
	}[s.form]
	lines := strings.Split(wrap(i18n.T("core.desc."+s.form), width-4), "\n")
	lines = append(lines, "")
	focusY := 0
	for i, key := range labels {
		if i == s.focus {
			focusY = len(lines) + 2
		}
		field := s.fields[i]
		field.Width = max(1, width-8)
		lines = append(lines, i18n.T(key), field.View(), "")
	}
	if s.Err != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.Danger).Render(wrap(s.Err.Error(), width-4)))
	}
	return pageLayout{content: panel(i18n.T("core.action."+s.form), strings.Join(lines, "\n"), width), focusY: focusY}
}

func (s State) viewportOffset(page pageLayout, height int) int {
	if !s.Editing && s.tab == 1 {
		return 0
	}
	offset := s.scroll
	if s.Editing && s.form != "service" {
		offset = max(0, page.focusY-height+2)
	} else if s.followSelection && len(page.actions) > 0 {
		hit := page.actions[min(s.selected, len(page.actions)-1)]
		if hit.y < offset {
			offset = hit.y
		}
		if hit.y+hit.height > offset+height {
			offset = hit.y + hit.height - height
		}
	}
	return min(max(0, offset), max(0, lipgloss.Height(page.content)-height))
}

func (s State) Render(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	contentHeight := max(1, height-1)
	page := s.layout(width, contentHeight)
	lines := strings.Split(page.content, "\n")
	offset := s.viewportOffset(page, contentHeight)
	lines = lines[offset:min(len(lines), offset+contentHeight)]
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "…")
	}
	base := common.OverlayHelpAtBottomRight(strings.Join(lines, "\n"), s.inlineHelp(width), width, height)
	if s.taskDialog {
		return s.overlayTask(base, width, height)
	}
	return base
}
