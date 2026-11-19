package core

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"
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
	editorY                 int
	content                 string
	actions                 []actionHit
	tabs                    []actionHit
	focusY                  int
	releases                []actionHit
	geoBounds               actionHit
	controls                []releaseControl
	listRows                int
	scrollOffset, maxScroll int
}

func wrap(text string, width int) string {
	return ansi.Wrap(text, max(1, width), "")
}

func panel(title, body string, width int) string {
	return common.RenderTokyoPanel(title, body, max(24, width))
}

func renderInlineSwitch(enabled bool, width int) (control string, offOffset, offWidth, onOffset, onWidth int) {
	t := theme.Current()
	offLabel := " " + i18n.T("core.disabled") + " "
	onLabel := " " + i18n.T("core.enabled") + " "
	offStyle := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted)).Background(theme.Resolve(t.Background))
	onStyle := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted)).Background(theme.Resolve(t.Background))
	if enabled {
		onStyle = lipgloss.NewStyle().Foreground(theme.Resolve(t.Background)).Background(theme.Resolve(t.Success)).Bold(true)
	} else {
		offStyle = lipgloss.NewStyle().Foreground(theme.Resolve(t.Bright)).Background(theme.Resolve(t.Overlay)).Bold(true)
	}
	muted := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted))
	offW := lipgloss.Width(offLabel)
	onW := lipgloss.Width(onLabel)
	rendered := muted.Render("[") + offStyle.Render(offLabel) + muted.Render("│") + onStyle.Render(onLabel) + muted.Render("]")
	if lipgloss.Width(rendered) > width {
		rendered = ansi.Truncate(rendered, max(1, width), "…")
	}
	return rendered, 1, offW, 1 + offW + 1, onW
}

func (s State) actionPanel(width int, serviceMenu bool) (string, []actionHit, []releaseControl) {
	t := theme.Current()
	muted := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted))
	selected := lipgloss.NewStyle().Foreground(theme.Resolve(t.Primary)).Background(theme.Resolve(t.Selected)).Bold(true)
	innerW := max(1, width-4)
	var rows []string
	var hits []actionHit
	var controls []releaseControl
	items := s.actions()
	if serviceMenu {
		items = nil
		for _, action := range serviceActions {
			items = append(items, actionItem{action: action})
		}
	}
	for i, item := range items {
		prefix, titleKey, descKey := "  ", "core.action."+item.action, "core.desc."+item.action
		if id, ok := strings.CutPrefix(item.action, "api:"); ok {
			titleKey, descKey = "settings.action."+id, ""
		}
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
		var desc string
		if descKey != "" {
			desc = "  " + strings.ReplaceAll(wrap(i18n.T(descKey), width-6), "\n", "\n  ")
		}
		if enabled, isSwitch := s.switchValue(item.action); isSwitch && !serviceMenu {
			switchStr, offOff, offW, onOff, onW := renderInlineSwitch(enabled, innerW)
			switchW := lipgloss.Width(switchStr)
			if innerW >= switchW+1+8 {
				leftAvail := innerW - switchW - 1
				leftText := common.PadString(ansi.Truncate(label, leftAvail, "…"), leftAvail)
				if i == s.selected {
					leftText = selected.Render(leftText)
				}
				rowY := len(rows) + 1
				ctrlX := 2 + leftAvail + 1
				controls = append(controls,
					releaseControl{actionHit: actionHit{x: ctrlX + offOff, y: rowY, width: offW, height: 1, index: 0}, action: "switch:" + item.action},
					releaseControl{actionHit: actionHit{x: ctrlX + onOff, y: rowY, width: onW, height: 1, index: 1}, action: "switch:" + item.action},
				)
				hits = append(hits, actionHit{x: 1, y: rowY, width: width - 2, height: 1 + lipgloss.Height(desc), index: i})
				rows = append(rows, leftText+" "+switchStr)
				rows = append(rows, strings.Split(muted.Render(desc), "\n")...)
				continue
			}
			labelLine := ansi.Truncate(label, innerW, "…")
			if i == s.selected {
				labelLine = selected.Render(common.PadString(labelLine, innerW))
			}
			indent := "  "
			if innerW < switchW+2 {
				indent = ""
			}
			rowY := len(rows) + 1
			ctrlY := rowY + 1
			ctrlX := 2 + lipgloss.Width(indent)
			controls = append(controls,
				releaseControl{actionHit: actionHit{x: ctrlX + offOff, y: ctrlY, width: offW, height: 1, index: 0}, action: "switch:" + item.action},
				releaseControl{actionHit: actionHit{x: ctrlX + onOff, y: ctrlY, width: onW, height: 1, index: 1}, action: "switch:" + item.action},
			)
			hits = append(hits, actionHit{x: 1, y: rowY, width: width - 2, height: 2 + lipgloss.Height(desc), index: i})
			rows = append(rows, labelLine, indent+switchStr)
			rows = append(rows, strings.Split(muted.Render(desc), "\n")...)
			continue
		}
		label = ansi.Truncate(label, innerW, "…")
		if i == s.selected {
			label = selected.Render(common.PadString(label, innerW))
		}
		height := 1
		if desc != "" {
			height += lipgloss.Height(desc)
		}
		hit := actionHit{x: 1, y: len(rows) + 1, width: width - 2, height: height, index: i}
		hits = append(hits, hit)
		rows = append(rows, label)
		if desc != "" {
			rows = append(rows, strings.Split(muted.Render(desc), "\n")...)
		}
	}
	title := "core.tab." + tabs[s.tab]
	if serviceMenu {
		title = "core.action.service"
	}
	return panel(i18n.T(title), strings.Join(rows, "\n"), width), hits, controls
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
		lipgloss.NewStyle().Foreground(theme.Resolve(color)).Bold(true).Render("● " + i18n.T(status)),
		i18n.T(mode),
	}
	if s.Status.Mode == "managed" {
		rows = append(rows, i18n.T("core.version")+": "+version)
	}
	if s.connectionChecked && !s.connected && s.Status.Mode == "managed" && s.Status.Running {
		rows = append(rows, lipgloss.NewStyle().Foreground(theme.Resolve(t.Warning)).Render(wrap(i18n.T("core.connection_unavailable"), width-4)))
	}
	switch s.tab {
	case 0, 2:
		value := func(text string) string {
			if text == "" {
				return i18n.T("core.unknown_version")
			}
			return text
		}
		if s.Status.Mode != "managed" {
			version = value(s.externalVersion)
		}
		binaryPath := s.Status.BinaryPath
		if binaryPath == "" {
			binaryPath = i18n.T("core.path_unavailable")
		}
		rows = rows[:1]
		for _, field := range []struct{ key, value string }{
			{"core.version", version},
			{"core.binary_path", binaryPath},
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
			if s.Status.Runtime == "service" {
				state := "core.service.status_not_installed"
				if s.Status.ServiceInstalled {
					state = "core.service.status_stopped"
					if s.Status.ServiceRunning {
						state = "core.service.status_running"
					}
				}
				rows = append(rows, wrap(i18n.T("settings.sys_status.panel_title")+": "+i18n.T(state), width-4))
				autoStart := "core.disabled"
				if s.Status.AutoStart {
					autoStart = "core.enabled"
				}
				rows = append(rows, wrap(i18n.T("core.autostart")+": "+i18n.T(autoStart), width-4))
			}
		} else if s.editor.SysStatus.LastOutput != "" {
			rows = append(rows, "", wrap(s.editor.SysStatus.LastOutput, width-4))
		}
		if s.Status.RecoveryPending {
			rows = append(rows, lipgloss.NewStyle().Foreground(theme.Resolve(t.Warning)).Render(wrap(i18n.T("core.hint.recovery"), width-4)))
		}
	case 1:
		tag := s.Release.Tag
		if tag == "" {
			tag = i18n.T("core.unknown_version")
		}
		rows = append(rows, "", i18n.T("core.available")+": "+tag,
			wrap(i18n.Tf("core.installed_count", len(s.Status.Installed)), width-4))
	default:
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
			rows = append(rows, lipgloss.NewStyle().Foreground(theme.Resolve(t.Warning)).Render(wrap(i18n.T("core.hint.recovery"), width-4)))
		}
	}
	return panel(i18n.T("core.overview"), strings.Join(rows, "\n"), width)
}

func (s State) layout(width, height int) pageLayout {
	width = max(24, width)
	if s.Editing && s.form != "service" {
		return s.formLayout(width)
	}
	if s.Editing {
		content, hits, controls := s.actionPanel(width, true)
		return pageLayout{content: content, actions: hits, controls: controls}
	}
	var result pageLayout
	selectedTab := s.tab
	if s.isAdvancedPage() {
		selectedTab = 0
	}
	if s.isDownloadsPage() {
		selectedTab = 1
	}
	tabText, hits := renderCoreTabRow(width, 1, selectedTab, []coreTabItem{{0, "advanced"}, {1, "downloads"}, {4, "mihomo"}})
	result.tabs = hits
	parts := []string{panel(i18n.T("core.heading"), tabText, width), ""}
	if s.isDownloadsPage() {
		return s.downloadsLayout(width, height, parts, result)
	}
	top := lipgloss.Height(strings.Join(parts, "\n"))
	if s.tab == 4 {
		parts = append(parts, preferences.RenderEmbedded(s.editor.ToPageState(s.editor.Config, nil), width, max(1, height-top)))
		result.content, result.editorY = strings.Join(parts, "\n"), top
		return result
	}
	leftWidth, rightWidth := width, width
	if width >= 100 {
		leftWidth = width * 43 / 100
		rightWidth = width - leftWidth
	}
	left := s.overview(leftWidth)
	right, hits, controls := s.actionPanel(rightWidth, false)
	for i := range hits {
		hits[i].y += top
		if width >= 100 {
			hits[i].x += leftWidth
		} else {
			hits[i].y += lipgloss.Height(left) + 1
		}
	}
	for i := range controls {
		controls[i].y += top
		if width >= 100 {
			controls[i].x += leftWidth
		} else {
			controls[i].y += lipgloss.Height(left) + 1
		}
	}
	result.actions = hits
	result.controls = controls
	if width >= 100 {
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, left, right))
	} else {
		parts = append(parts, left, "", right)
	}
	result.content = strings.Join(parts, "\n")
	return result
}

func (s State) downloadsLayout(width, height int, parts []string, result pageLayout) pageLayout {
	top := lipgloss.Height(strings.Join(parts, "\n"))
	releaseWidth, geoWidth := width, width
	releaseHeight := max(3, height-top)
	if width >= 100 {
		releaseWidth = width * 43 / 100
		geoWidth = width - releaseWidth
	} else {
		releaseHeight = max(3, releaseHeight/2)
	}
	versions := s
	if s.tab == 3 {
		versions.selected, _ = s.releaseIndex(s.Release.Tag)
	}
	releases := versions.releaseLayout(releaseWidth, releaseHeight)
	result.listRows, result.releases = releases.listRows, releases.releases
	for i := range result.releases {
		result.releases[i].y += top
	}
	geo, hits := s.geoPanel(geoWidth)
	geo += "\n" + s.geoOverview(geoWidth)
	geoX, geoY := releaseWidth, top
	if width >= 100 {
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, releases.content, geo))
	} else {
		geoX, geoY = 0, top+lipgloss.Height(releases.content)+1
		parts = append(parts, releases.content, "", geo)
	}
	for i := range hits {
		hits[i].x += geoX
		hits[i].y += geoY
	}
	result.geoBounds = actionHit{x: geoX, y: geoY, width: geoWidth, height: lipgloss.Height(geo)}
	result.actions, result.content = hits, strings.Join(parts, "\n")
	return result
}

type coreTabItem struct {
	id    int
	label string
}

func renderCoreTabRow(width, y, selected int, items []coreTabItem) (string, []actionHit) {
	t := theme.Current()
	muted := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted))
	active := lipgloss.NewStyle().Foreground(theme.Resolve(t.Primary)).Background(theme.Resolve(t.Selected)).Bold(true)
	var labels []string
	var hits []actionHit
	x, selectedEnd := 0, 0
	for _, item := range items {
		label := " " + i18n.T("core.tab."+item.label) + " "
		w := lipgloss.Width(label)
		hits = append(hits, actionHit{x: x, y: y, width: w, height: 1, index: item.id})
		style := muted
		if selected == item.id {
			selectedEnd = x + w
			style = active
		}
		labels = append(labels, style.Render(label))
		x += w + 1
	}
	inner := width - 4
	offset := max(0, selectedEnd-inner)
	for i := range hits {
		hit := &hits[i]
		start := max(0, hit.x-offset)
		end := min(inner, hit.x+hit.width-offset)
		hit.x, hit.width = start+2, max(0, end-start)
	}
	return ansi.Cut(strings.Join(labels, muted.Render("│")), offset, offset+inner), hits
}

func (s State) formLayout(width int) pageLayout {
	t := theme.Current()
	labels := map[string][]string{
		"network":  {"core.proxy", "core.accelerators", "core.download_timeout"},
		"version":  {"core.version_input"},
		"import":   {"cli.core.file", "core.version_input", "cli.core.sha256"},
		"external": {"core.controller", "core.secret", "core.mixed_port"},
	}[s.form]
	lines := strings.Split(wrap(i18n.T("core.desc."+s.form), width-4), "\n")
	lines = append(lines, "")
	focusY := 0
	var hits []actionHit
	for i, key := range labels {
		if i == s.focus {
			focusY = len(lines) + 2
		}
		field := s.fields[i]
		field.Width = max(1, width-8)
		hits = append(hits, actionHit{x: 2, y: len(lines) + 2, width: width - 4, height: 1, index: i})
		lines = append(lines, i18n.T(key), field.View(), "")
	}
	if s.formErr != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(theme.Resolve(t.Danger)).Render(wrap(s.formErr.Error(), width-4)))
	}
	return pageLayout{content: panel(i18n.T("core.action."+s.form), strings.Join(lines, "\n"), width), focusY: focusY, actions: hits}
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
		if hit.y+hit.height > offset+height {
			offset = hit.y + hit.height - height
		}
		if hit.y < offset {
			offset = hit.y
		}
	} else if s.tab == 3 && s.followSelection {
		offset = max(0, page.geoBounds.y-height+2)
	}
	return min(max(0, offset), max(0, lipgloss.Height(page.content)-height))
}

func (s State) Render(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if s.Editing {
		background := s
		background.Editing, background.Err = false, nil
		background.selected, background.scroll = s.formSelection, s.formScroll
		return s.overlayForm(background.Render(width, height), width, height)
	}
	footer := s.footer(width, height)
	contentHeight := max(1, height-lipgloss.Height(footer))
	page := s.layout(width, contentHeight)
	lines := strings.Split(page.content, "\n")
	offset := s.viewportOffset(page, contentHeight)
	lines = lines[offset:min(len(lines), offset+contentHeight)]
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "…")
	}
	return common.OverlayHelpAtBottomRight(strings.Join(lines, "\n"), footer, width, height)
}
