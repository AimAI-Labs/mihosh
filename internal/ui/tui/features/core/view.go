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

// The view and mouse routing share these bounds, including wrapped rows.
type actionHit struct {
	x, y, width, height int
	index               int
}

type pageLayout struct {
	editorY                 int
	systemdY                int
	serviceMaxScroll        int
	content                 string
	pinnedRows              int
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

func (s State) actionPanel(width int, title string, items []actionItem, firstIndex int) (string, []actionHit, []releaseControl) {
	t := theme.Current()
	muted := lipgloss.NewStyle().Foreground(theme.Resolve(t.Muted))
	selected := lipgloss.NewStyle().Foreground(theme.Resolve(t.Primary)).Background(theme.Resolve(t.Selected)).Bold(true)
	innerW := max(1, width-4)
	var rows []string
	var hits []actionHit
	var controls []releaseControl
	for index, item := range items {
		i := firstIndex + index
		prefix, titleKey := "  ", "core.action."+item.action
		if id, ok := strings.CutPrefix(item.action, "api:"); ok {
			titleKey = "settings.action." + id
		}
		if i == s.selected {
			prefix = "› "
		}
		label := prefix + i18n.T(titleKey)
		if item.key != "" && item.key != "enter" {
			label += " [" + item.key + "]"
		}
		reason := s.switchUnavailable(item.action)
		style := lipgloss.NewStyle()
		if i == s.selected {
			style = selected
		}
		if reason != "" {
			style = style.Foreground(theme.Resolve(t.Muted))
		}
		renderLabel := func(avail int) string {
			return style.Render(common.PadString(ansi.Truncate(label, avail, "…"), avail))
		}

		rowY := len(rows) + 1
		if enabled, isSwitch := s.switchValue(item.action); isSwitch && reason == "" {
			switchStr, offOffset, offW, onOffset, onW := renderInlineSwitch(enabled, innerW)
			switchW := lipgloss.Width(switchStr)
			ctrlX, ctrlY := 2, rowY
			if left := innerW - switchW - 1; left >= 8 {
				rows = append(rows, renderLabel(left)+" "+switchStr)
				ctrlX += left + 1
			} else {
				indent := min(2, innerW-switchW)
				rows = append(rows, renderLabel(innerW), strings.Repeat(" ", indent)+switchStr)
				ctrlX, ctrlY = ctrlX+indent, ctrlY+1
			}
			controls = append(controls,
				releaseControl{actionHit: actionHit{x: ctrlX + offOffset, y: ctrlY, width: offW, height: 1, index: 0}, action: "switch:" + item.action},
				releaseControl{actionHit: actionHit{x: ctrlX + onOffset, y: ctrlY, width: onW, height: 1, index: 1}, action: "switch:" + item.action},
			)
		} else {
			rows = append(rows, renderLabel(innerW))
		}
		if reason != "" {
			rows = append(rows, muted.Render("  "+ansi.Truncate(i18n.T(reason), innerW-2, "…")))
		}
		hits = append(hits, actionHit{x: 1, y: rowY, width: width - 2, height: len(rows) + 1 - rowY, index: i})
	}
	return panel(i18n.T(title), strings.Join(rows, "\n"), width), hits, controls
}

func (s State) statusLine(width int) string {
	t := theme.Current()
	status, color := "core.state.external", t.Info
	switch {
	case s.Status.Mode == "managed":
		status, color = "core.state.stopped", t.Warning
		if s.Status.Running {
			status, color = "core.state.running", t.Success
			if s.controllerUnavailable() {
				status, color = "core.state.running_unreachable", t.Warning
			}
		}
	case s.connected:
		status, color = "core.state.connected", t.Success
	case s.Status.Version == "":
		status = "core.state.welcome"
	}
	parts := []string{lipgloss.NewStyle().Foreground(theme.Resolve(color)).Bold(true).Render("● " + i18n.T(status))}
	if s.Status.RecoveryPending {
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.Resolve(t.Warning)).Render(i18n.T("core.state.recovery_pending")))
	}
	if version := s.coreVersion(); version != "" {
		parts = append(parts, strings.Join(strings.Fields(ansi.Strip(version)), " "))
	}
	if s.Status.MixedPort > 0 {
		parts = append(parts, i18n.T("core.mixed_port_label")+" "+fmt.Sprint(s.Status.MixedPort))
	}
	return ansi.Truncate("  "+strings.Join(parts, " · "), width, "…")
}

func (s State) coreVersion() string {
	if s.Status.Mode == "managed" {
		return s.Status.Version
	}
	return s.externalVersion
}

func (s State) controllerUnavailable() bool {
	return s.Status.Mode == "managed" && s.Status.Running && s.connectionChecked && !s.connected
}

func (s State) coreDetails(width int) string {
	value := func(text string) string {
		if text == "" {
			return i18n.T("core.unknown_version")
		}
		return text
	}
	version := value(s.coreVersion())
	if s.Status.Mode == "managed" && s.Status.Version == "" {
		version = i18n.T("core.not_installed")
	}
	binaryPath := s.Status.BinaryPath
	if binaryPath == "" {
		binaryPath = i18n.T("core.path_unavailable")
	}
	var rows []string
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
	t := theme.Current()
	warning := lipgloss.NewStyle().Foreground(theme.Resolve(t.Warning))
	if s.controllerUnavailable() {
		rows = append(rows, warning.Render(wrap(i18n.T("core.connection_unavailable"), width-4)))
	}
	if s.Status.RecoveryPending {
		rows = append(rows, warning.Render(wrap(i18n.T("core.hint.recovery"), width-4)))
	}
	return panel(i18n.T("core.action.details"), strings.Join(rows, "\n"), width)
}

func (s State) layout(width, height int) pageLayout {
	width = max(24, width)
	if s.Editing {
		return s.formLayout(width)
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
	parts := []string{panel("", tabText, width), ""}
	if s.isDownloadsPage() {
		return s.downloadsLayout(width, height, parts, result)
	}
	top := lipgloss.Height(strings.Join(parts, "\n"))
	if s.tab == 4 {
		editorState := s.editor.ToPageState(s.editor.Config, nil)
		if s.serviceFocus && s.hasServiceCard() {
			editorState.SelectedSetting = -1
		}
		parts = append(parts, preferences.RenderEmbedded(editorState, width))
		if s.hasServiceCard() {
			systemdY := lipgloss.Height(strings.Join(parts, "\n")) + 1
			panelView, controls, maxScroll := s.systemdPanel(width, height-systemdY)
			for i := range controls {
				controls[i].y += systemdY
			}
			parts = append(parts, "", panelView)
			result.systemdY = systemdY
			result.serviceMaxScroll = maxScroll
			result.controls = controls
		}
		result.content, result.editorY = strings.Join(parts, "\n"), top
		return result
	}
	parts[len(parts)-1] = s.statusLine(width)
	result.pinnedRows = top
	run, proxy, maintenance := s.advancedActions()
	appendPanel := func(title string, items []actionItem, panelWidth, x, y int) string {
		content, hits, controls := s.actionPanel(panelWidth, title, items, len(result.actions))
		for i := range hits {
			hits[i].x += x
			hits[i].y += y
		}
		for i := range controls {
			controls[i].x += x
			controls[i].y += y
		}
		result.actions = append(result.actions, hits...)
		result.controls = append(result.controls, controls...)
		return content
	}

	leftWidth, rightWidth := width, width
	proxyX, proxyY := 0, top
	if width >= 100 {
		leftWidth = width * 43 / 100
		rightWidth = width - leftWidth
		proxyX = leftWidth
	}
	running := appendPanel("core.group.running", run, leftWidth, 0, top)
	if proxyX == 0 {
		proxyY += lipgloss.Height(running)
	}
	switches := appendPanel("core.group.proxy", proxy, rightWidth, proxyX, proxyY)
	tools := appendPanel("core.group.maintenance", maintenance, rightWidth, proxyX, proxyY+lipgloss.Height(switches))
	right := lipgloss.JoinVertical(lipgloss.Left, switches, tools)
	if proxyX != 0 {
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, running, right))
	} else {
		parts = append(parts, running, right)
	}
	result.content = strings.Join(parts, "\n")
	return result
}

func (s State) downloadsLayout(width, height int, parts []string, result pageLayout) pageLayout {
	top := lipgloss.Height(strings.Join(parts, "\n"))
	const networkKey = " [e]"
	networkLabel := ansi.Truncate(i18n.T("core.action.network"), width-4-lipgloss.Width(networkKey), "…") + networkKey
	labelWidth := lipgloss.Width(networkLabel)
	networkX := width - 2 - labelWidth
	parts[len(parts)-1] = strings.Repeat(" ", networkX) + common.HighlightStyle().Render(networkLabel)
	result.pinnedRows = top
	result.controls = []releaseControl{{
		actionHit: actionHit{x: networkX, y: top - 1, width: labelWidth, height: 1},
		action:    "network",
	}}
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

func (s State) serviceQuickStyle(action string, focused bool) lipgloss.Style {
	t := theme.Current()
	if focused && !s.Busy {
		bg := t.Primary
		switch action {
		case "stop":
			bg = t.Warning
		case "uninstall":
			bg = t.Danger
		}
		return lipgloss.NewStyle().
			Background(theme.Resolve(bg)).
			Foreground(theme.Resolve(t.Background)).
			Bold(true)
	}
	base := lipgloss.NewStyle().Background(theme.Resolve(t.Selected))
	if s.Busy {
		return base.Foreground(theme.Resolve(t.Muted))
	}
	switch action {
	case "start", "restart", "install":
		return base.Foreground(theme.Resolve(t.Primary))
	case "stop":
		return base.Foreground(theme.Resolve(t.Warning))
	case "uninstall":
		return base.Foreground(theme.Resolve(t.Danger))
	default:
		return base.Foreground(theme.Resolve(t.Foreground))
	}
}

func (s State) serviceQuickMenu(width int) ([]string, []releaseControl) {
	groups := s.serviceQuickGroups()
	muted := common.DimStyle()
	var rows []string
	var controls []releaseControl
	var line string
	x := 0
	actionIdx := 0
	for _, section := range groups {
		var title string
		if section.label != "" {
			title = i18n.T("core.service.group." + section.label)
		}
		titleWidth := lipgloss.Width(title)
		groupWidth := titleWidth
		labels := make([]string, len(section.actions))
		for i, action := range section.actions {
			labels[i] = " " + i18n.T("core.service.quick."+action) + " "
			groupWidth += 1 + lipgloss.Width(labels[i])
		}
		const gap = "   "
		if x > 0 && x+len(gap)+groupWidth > width {
			rows = append(rows, line)
			line, x = "", 0
		}
		if x > 0 {
			line += gap
			x += len(gap)
		}
		line += muted.Render(title)
		x += titleWidth
		for i, action := range section.actions {
			label := labels[i]
			btnW := lipgloss.Width(label)
			if x > 0 && x+1+btnW > width {
				rows = append(rows, line)
				line, x = "", 0
			}
			if x > 0 {
				line += " "
				x++
			}
			if !s.Busy {
				controls = append(controls, releaseControl{
					actionHit: actionHit{x: 2 + x, y: len(rows) + 1, width: btnW, height: 1, index: actionIdx},
					action:    "service:" + action,
				})
			}
			focused := s.serviceFocus && actionIdx == s.serviceActionIdx
			line += s.serviceQuickStyle(action, focused).Render(label)
			x += btnW
			actionIdx++
		}
	}
	return append(rows, line), controls
}

func (s State) systemdPanel(width, availHeight int) (string, []releaseControl, int) {
	innerW := max(1, width-4)
	menuRows, controls := s.serviceQuickMenu(innerW)
	menuRows = append(menuRows, "")
	logHeight := max(3, availHeight-2-len(menuRows))
	body := s.Status.ServiceUnit + "\n" + i18n.T("core.service.status_loading")
	if s.serviceLog.Unit == s.Status.ServiceUnit {
		body = strings.TrimRight(ansi.Strip(s.serviceLog.Output), "\r\n")
		if body == "" && s.serviceLog.Err != nil {
			body = s.serviceLog.Err.Error()
		}
		if body == "" {
			body = s.Status.ServiceUnit
		}
	}
	logWidth := innerW
	logLines := strings.Split(wrap(body, logWidth), "\n")
	if len(logLines) > logHeight && innerW >= 22 {
		logWidth = innerW - 2
		logLines = strings.Split(wrap(body, logWidth), "\n")
	}
	maxScroll := max(0, len(logLines)-logHeight)
	start := max(0, min(s.serviceScroll, maxScroll))
	var scrollbar string
	if len(logLines) > logHeight && innerW >= 22 {
		scrollbar = common.BuildVerticalScrollbar(logHeight, len(logLines), start, true)
	}
	visible := append([]string(nil), logLines[start:min(len(logLines), start+logHeight)]...)
	for len(visible) < logHeight {
		visible = append(visible, "")
	}
	if scrollbar != "" {
		tracks := strings.Split(scrollbar, "\n")
		for i := range min(len(visible), len(tracks)) {
			visible[i] = common.PadString(visible[i], logWidth) + " " + tracks[i]
		}
	}
	rows := append(menuRows, visible...)
	return panel(i18n.T("settings.sys_status.panel_title"), strings.Join(rows, "\n"), width), controls, maxScroll
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
	if s.Editing {
		offset = max(0, page.focusY-height+2)
	} else if s.tab == 4 && s.serviceFocus && page.systemdY > 0 {
		if page.systemdY+3 > offset+height {
			offset = page.systemdY
		}
	} else if s.followSelection && len(page.actions) > 0 {
		index := min(s.selected, len(page.actions)-1)
		hit := page.actions[index]
		bottom := hit.y + hit.height
		if s.tab == 3 && index == len(page.actions)-1 {
			bottom = page.geoBounds.y + page.geoBounds.height
		}
		if bottom > offset+height {
			offset = bottom - height
		}
		if hit.y < offset+page.pinnedRows {
			offset = hit.y - page.pinnedRows
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
	end := min(len(lines), offset+contentHeight)
	if page.pinnedRows == 0 {
		lines = lines[offset:end]
	} else {
		pinned := min(page.pinnedRows, contentHeight)
		lines = append(lines[:pinned], lines[pinned+offset:end]...)
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "…")
	}
	return common.OverlayHelpAtBottomRight(strings.Join(lines, "\n"), footer, width, height)
}
