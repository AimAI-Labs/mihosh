package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func coreKey(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

func selectTestAction(t *testing.T, state State, action string) State {
	t.Helper()
	for index, item := range state.actions() {
		if item.action == action {
			state.selected = index
			return state
		}
	}
	t.Fatalf("action %q is missing", action)
	return state
}

func TestCoreFirstUseExplainsNextStepWithoutConnectionDiagnostics(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	s := State{followSelection: true}
	s, _ = s.Update(messages.CoreConnectionMsg{Err: errors.New("dial tcp: connection refused")})
	view := ansi.Strip(s.Render(120, 28))
	for _, want := range []string{"欢迎使用", "一键开始使用", "已有内核", "内核详情"} {
		if !strings.Contains(view, want) {
			t.Errorf("first-use guidance missing %q", want)
		}
	}
	for _, raw := range []string{"true", "false", "connection refused", "SHA-256", "TUN"} {
		if strings.Contains(view, raw) {
			t.Errorf("advanced detail %q leaked into first-use view", raw)
		}
	}
	if s.Err != nil {
		t.Fatal("an unavailable external API must not be treated as a failed user operation")
	}
}

func TestCoreEnterFollowsSelectionAndServiceMenuCapturesKeys(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	svc := service.NewCoreService()
	defer svc.Close()
	s := NewState(svc, nil, nil)
	s.Status = service.CoreStatus{Mode: "managed", Version: "v1", Running: true}
	for _, item := range s.actions() {
		if item.action == "profile" || item.key == "o" {
			t.Fatal("duplicate subscription entry remains in the daily menu")
		}
	}
	s, cmd := s.Update(coreKey("o"))
	if s.Editing || s.Busy || cmd != nil {
		t.Fatal("removed subscription shortcut still performs an action")
	}
	s, _ = s.Update(coreKey("3"))
	s = selectTestAction(t, s, "service")
	s.selected--
	s, _ = s.Update(coreKey("down"))
	s, cmd = s.Update(coreKey("enter"))
	if !s.Editing || s.form != "service" || s.Busy || cmd != nil {
		t.Fatal("selected background service entry did not open its menu")
	}
	s, cmd = s.Update(coreKey("y"))
	if s.Busy || cmd != nil || len(s.fields) != 0 {
		t.Fatal("service menu allowed an underlying network action or text input")
	}
	s, _ = s.Update(coreKey("down"))
	if s.selected != 1 {
		t.Fatal("service selection did not move")
	}
	s, cmd = s.Update(coreKey("enter"))
	defer s.Close()
	if s.Editing || !s.Busy || cmd == nil {
		t.Fatal("selected service action was not queued")
	}
	// Do not execute the command: real system services must not be touched by UI tests.
}

func TestCoreWheelScrollsActionsAndKeepsHeaderAndSelection(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{40, 80, 120} {
			for _, tab := range []int{0, 2} {
				s := State{tab: tab, followSelection: true, connected: true,
					Status: service.CoreStatus{Mode: "managed", Version: "v1", Running: true}}
				const height = 14
				contentHeight := height - footerHeight
				page := s.layout(width, contentHeight)
				wheel := messages.PageMouseScrollMsg{X: 4, Y: page.pinnedRows - 1, Width: width, Height: height}
				initial := strings.Split(ansi.Strip(s.Render(width, height)), "\n")
				same, cmd := s.Update(wheel)
				if same.scroll != 0 || !same.followSelection || cmd != nil {
					t.Fatal("wheel over the status row scrolled the actions")
				}
				wheel.Y++
				for range lipgloss.Height(page.content) {
					next, cmd := s.Update(wheel)
					if next.selected != s.selected || next.Busy || next.Editing || cmd != nil {
						t.Fatal("content wheel changed or ran the selected action")
					}
					s = next
				}
				if s.scroll != lipgloss.Height(page.content)-contentHeight || s.followSelection {
					t.Fatal("wheel did not reach the end of the action list")
				}
				scrolled := strings.Split(ansi.Strip(s.Render(width, height)), "\n")
				if strings.Join(initial[:page.pinnedRows], "\n") != strings.Join(scrolled[:page.pinnedRows], "\n") {
					t.Fatalf("%s width=%d tab=%d: the header moved while scrolling", language, width, tab)
				}
				hit := page.tabs[0]
				clicked, _ := s.Update(messages.PageMouseClickMsg{X: hit.x, Y: hit.y, Width: width, Height: height})
				if clicked.tab != 0 || clicked.scroll != 0 {
					t.Fatal("the pinned tab could not be clicked after scrolling")
				}

				s, _ = s.Update(coreKey("down"))
				page = s.layout(width, contentHeight)
				offset := s.viewportOffset(page, contentHeight)
				if y := page.actions[s.selected].y - offset; s.selected != 1 || y < page.pinnedRows || y >= contentHeight {
					t.Fatal("keyboard navigation did not follow and reveal the selected action")
				}
			}
		}
	}
}

func TestCoreExistingConnectionOffersConnectionFormWithoutStartingCore(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()
	s := NewState(svc, nil, nil)
	s, _ = s.Update(messages.CoreConnectionMsg{})
	s, _ = s.Update(coreKey("enter"))
	if !s.Editing || s.form != "external" || s.Busy {
		t.Fatal("existing connection must offer its settings before any managed-core setup")
	}
}

func TestCoreTabsCanBeSelectedByMouse(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	s := State{tab: 1, followSelection: true}
	view := ansi.Strip(s.Render(120, 28))
	for _, label := range []string{"1 Daily use", "2 Versions", "3 Advanced"} {
		if strings.Contains(view, label) {
			t.Fatalf("section still has a numeric prefix: %q", label)
		}
	}
	for y, line := range strings.Split(view, "\n") {
		if at := strings.Index(line, i18n.T("core.tab.advanced")); at >= 0 {
			next, cmd := s.Update(messages.PageMouseClickMsg{X: lipgloss.Width(line[:at]), Y: y, Width: 120, Height: 28})
			if next.tab != 0 || next.Busy || next.Editing || cmd != nil {
				t.Fatal("tab click did not change section without running an action")
			}
			return
		}
	}
	t.Fatal("advanced tab was not visible")
}

func TestCoreTaskProgressIsVisibleAfterStartingAnOffscreenAction(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	s := State{tab: 1, selected: 6, scroll: 30, followSelection: true}
	s, _ = s.begin("import")
	defer s.Close()
	s.Progress = coreinfra.Progress{Route: "accelerator:gh-proxy.org", Downloaded: 1 << 20, Total: 4 << 20}
	view := ansi.Strip(s.Render(80, 20))
	for _, want := range []string{"Task progress", "GitHub mirror", "1.0 MiB / 4.0 MiB"} {
		if !strings.Contains(view, want) {
			t.Errorf("progress missing %q after starting a scrolled action", want)
		}
	}
}

func TestCoreVisibleMouseTargetsMatchAfterViewportScrolling(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	svc := service.NewCoreService()
	defer svc.Close()
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{64, 80, 100, 120} {
			for _, height := range []int{12, 28} {
				for _, action := range []string{"network", "configure"} {
					s := NewState(svc, nil, nil)
					if action == "network" {
						s = s.selectTab(1)
					} else {
						s = selectTestAction(t, s.selectTab(2), action)
					}
					label := i18n.T("core.action." + action)
					view := ansi.Strip(s.Render(width, height))
					found := false
					for y, line := range strings.Split(view, "\n") {
						at := strings.Index(line, label)
						if at < 0 {
							continue
						}
						x := lipgloss.Width(line[:at]) + 1
						next, _ := s.Update(messages.PageMouseClickMsg{X: x, Y: y, Width: width, Height: height})
						if !next.Editing || next.form != action {
							t.Fatalf("%s %dx%d: click on visible %s did not open its form", language, width, height, action)
						}
						found = true
						break
					}
					if !found {
						t.Fatalf("%s %dx%d: selected %s is not visible:\n%s", language, width, height, action, view)
					}
				}
			}
		}
	}
}

func TestCoreAllSectionsAndFormsRespectTerminalBounds(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	svc := service.NewCoreService()
	defer svc.Close()
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{12, 24, 64, 99, 100, 120} {
			for _, height := range []int{8, 24} {
				var states []State
				for tab := range tabs {
					s := NewState(svc, nil, nil).selectTab(tab)
					s.Status = service.CoreStatus{Mode: "managed", Version: "v1", Running: true, SystemProxy: true, TUN: true, RecoveryPending: true}
					s.Err = errors.New(strings.Repeat("a long error message ", 12))
					states = append(states, s)
				}
				for _, form := range []string{"network", "version", "import", "external", "service"} {
					s, _ := NewState(svc, nil, nil).editForm(form)
					states = append(states, s)
				}
				for _, s := range states {
					view := s.Render(width, height)
					if lipgloss.Height(view) > height {
						t.Fatalf("%s %dx%d: view overflow", language, width, height)
					}
					for _, line := range strings.Split(view, "\n") {
						if lipgloss.Width(line) > width {
							t.Fatalf("%s %dx%d: line overflow %q", language, width, height, line)
						}
						if strings.Contains(line, "core.desc.") || strings.Contains(line, "core.service.desc.") {
							t.Fatalf("missing translation: %q", line)
						}
					}
				}
			}
		}
	}
}

func TestCoreHelpStaysAtBottomRightWhileScrolling(t *testing.T) {
	i18n.Init()
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{24, 80, 120} {
			s := State{tab: 2, selected: 4, followSelection: true}
			lines := strings.Split(ansi.Strip(s.Render(width, 16)), "\n")
			if len(lines) != 16 || !strings.HasSuffix(lines[15], "? "+i18n.T("core.inline.help")) {
				t.Fatalf("%s %d: help is not fixed at bottom right: %q", language, width, lines)
			}
			if lipgloss.Width(lines[15]) != width-1 {
				t.Fatalf("help row does not fill the footer: %q", lines[15])
			}
		}
	}
	i18n.SetLanguageOverride("")
}

func TestCoreStatusLineAndDetails(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	s := State{followSelection: true, connectionChecked: true, Status: service.CoreStatus{
		Mode: "managed", Running: true, Version: "v1.2.3", BinaryPath: "/core/mihomo",
		ConfigPath: "/runtime/config.yaml", Controller: "127.0.0.1:9090", MixedPort: 7890,
	}}
	for _, width := range []int{80, 99, 100, 120} {
		view := ansi.Strip(s.Render(width, 24))
		status := strings.Split(view, "\n")[s.layout(width, 24-footerHeight).pinnedRows-1]
		for _, want := range []string{i18n.T("core.state.running_unreachable"), "v1.2.3", "7890"} {
			if !strings.Contains(status, want) {
				t.Errorf("width %d: status line missing %q: %s", width, want, status)
			}
		}
		for _, key := range []string{"core.action.system-proxy", "core.action.tun", "core.action.shell-proxy"} {
			if !strings.Contains(view, i18n.T(key)) {
				t.Errorf("width %d: proxy switch %q is missing from the first screen", width, key)
			}
		}
		if strings.Contains(view, "/core/mihomo") || strings.Contains(view, "/runtime/config.yaml") {
			t.Fatal("full paths should be in the details dialog")
		}
	}
	details, cmd := s.Update(coreKey("enter"))
	if !details.Editing || details.form != "details" || cmd != nil {
		t.Fatal("the running core should default to opening details")
	}
	view := ansi.Strip(details.Render(120, 30))
	for _, want := range []string{"/core/mihomo", "/runtime/config.yaml", "127.0.0.1:9090", i18n.T("core.connection_unavailable")} {
		if !strings.Contains(view, want) {
			t.Errorf("details missing %q", want)
		}
	}
	details, _ = details.Update(messages.PageResizeMsg{Width: 40, Height: 10})
	for range 30 {
		details, _ = details.Update(coreKey("down"))
	}
	page, _ := details.formDialog(40, 10)
	if page.maxScroll == 0 || details.dialogScroll != page.maxScroll {
		t.Fatal("details keyboard scrolling did not stop at the end")
	}
	details, _ = details.Update(coreKey("k"))
	if details.dialogScroll != page.maxScroll-1 {
		t.Fatal("keyboard did not scroll back from the end")
	}
	details, _ = details.Update(coreKey("esc"))
	if details.Editing || details.dialogScroll != 0 {
		t.Fatal("closing details did not clear dialog state")
	}

	s.Status.Mode = "external"
	s, _ = s.Update(messages.CoreConnectionMsg{Version: "v9.8.7"})
	view = ansi.Strip(s.coreDetails(70))
	if !strings.Contains(view, "v9.8.7") || strings.Contains(view, "v1.2.3") {
		t.Fatal("external details did not use the connected core version")
	}
	if !strings.Contains(view, "/core/mihomo") {
		t.Fatal("external executable path is not visible")
	}
	s.Status.BinaryPath = ""
	if !strings.Contains(ansi.Strip(s.coreDetails(70)), i18n.T("core.path_unavailable")) {
		t.Fatal("unavailable executable path is not explained")
	}
}

func TestCoreExternalSystemProxyExplainsUnavailableState(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	s := selectTestAction(t, State{followSelection: true, connected: true,
		Status: service.CoreStatus{Mode: "external"}}, "system-proxy")
	const width, height = 80, 24
	view := ansi.Strip(s.Render(width, height))
	if !strings.Contains(view, i18n.T("core.switch.managed_only")) {
		t.Fatal("the unavailable proxy switch is missing its explanation")
	}
	page := s.layout(width, height-footerHeight)
	for _, control := range page.controls {
		if control.action == "switch:system-proxy" {
			t.Fatal("the unavailable proxy switch still has active pills")
		}
	}
	hit := page.actions[s.selected]
	for _, input := range []tea.Msg{coreKey("y"), coreKey("enter"), coreKey(" "),
		messages.PageMouseClickMsg{X: hit.x, Y: hit.y - s.viewportOffset(page, height-footerHeight), Width: width, Height: height}} {
		next, cmd := s.Update(input)
		if next.Busy || next.Editing || cmd != nil {
			t.Fatalf("unavailable system proxy was activated by %v", input)
		}
	}
}

func TestCoreMihomoPageShowsManagedStatusLogsBelowOverrides(t *testing.T) {
	i18n.Init()
	s := State{tab: 4, Status: service.CoreStatus{
		Mode: "managed", ServiceUnit: "mihosh-example.service", ServiceInstalled: true, ServiceRunning: true, AutoStart: true,
	}}
	s, _ = s.Update(messages.CoreServiceLogMsg{
		Unit:   s.Status.ServiceUnit,
		Output: "● mihosh-example.service\n   Active: active (running)\n\nNov 20 10:12:00 node mihosh[42]: Started managed core",
	})
	layout40 := s.layout(100, 40)
	if lipgloss.Height(layout40.content) != 40 {
		t.Fatalf("expected systemctl card to fill remaining view height (40), got %d", lipgloss.Height(layout40.content))
	}
	page := ansi.Strip(layout40.content)
	firstLine, _, _ := strings.Cut(page, "\n")
	if strings.Contains(firstLine, i18n.T("core.heading")) {
		t.Fatal("Core navigation has a panel title")
	}
	settings := strings.Index(page, i18n.T("settings.panel_title"))
	card := strings.Index(page, i18n.T("settings.sys_status.panel_title"))
	quickRestart := strings.Index(page, i18n.T("core.service.quick.restart"))
	unit := strings.Index(page, "mihosh-example.service")
	if settings < 0 || card <= settings || quickRestart <= card || unit <= quickRestart ||
		!strings.Contains(page, "Active: active (running)") || !strings.Contains(page, "Started managed core") {
		t.Fatal("managed service card does not show quick actions above command output and recent service logs")
	}
	if strings.Contains(page, i18n.T("core.autostart")+" "+i18n.T("core.enabled")) {
		t.Fatal("managed service card repeats the old startup summary")
	}
	// Scrolling inside the systemctl card in a normal terminal keeps top panels & quick menu fixed while scrolling logs.
	longLogs := "● mihosh-example.service\n   Active: active (running)\n"
	for i := range 15 {
		longLogs += "\nlog-line-" + string(rune('A'+i))
	}
	longLogs += "\nStarted managed core tail"
	sScroll, _ := s.Update(messages.CoreServiceLogMsg{Unit: s.Status.ServiceUnit, Output: longLogs})
	for range 20 {
		sScroll, _ = sScroll.Update(messages.PageMouseScrollMsg{X: 20, Y: 16, Width: 100, Height: 26})
	}
	renderedScroll := ansi.Strip(sScroll.Render(100, 26))
	if !strings.Contains(renderedScroll, i18n.T("settings.panel_title")) ||
		!strings.Contains(renderedScroll, i18n.T("core.service.quick.restart")) ||
		!strings.Contains(renderedScroll, "Started managed core tail") {
		t.Fatalf("internal systemctl card scroll did not preserve header/quick menu while reaching tail:\n%s", renderedScroll)
	}
	// Clicking a systemctl quick action button queues the service action.
	for y, line := range strings.Split(renderedScroll, "\n") {
		if at := strings.Index(line, i18n.T("core.service.quick.restart")); at >= 0 {
			clicked, cmd := sScroll.Update(messages.PageMouseClickMsg{
				X: lipgloss.Width(line[:at]), Y: y, Width: 100, Height: 26,
			})
			defer clicked.Close()
			if !clicked.Busy || clicked.task != "service" || cmd == nil {
				t.Fatal("clicking quick restart button did not start service action")
			}
			break
		}
	}
	wheel := messages.PageMouseScrollMsg{X: 30, Y: 6, Width: 80, Height: 13}
	for range 20 {
		s, _ = s.Update(wheel)
	}
	if !strings.Contains(ansi.Strip(s.Render(80, 13)), "Started managed core") {
		t.Fatal("managed service card is unreachable in a short terminal")
	}
}

func TestCoreServiceQuickMenuMatchesServiceStateAndWraps(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{24, 40, 120} {
			for _, status := range []struct {
				installed, running bool
				actions            string
			}{
				{false, false, "install"},
				{true, false, "start,enable,boot,disable,install,uninstall"},
				{true, true, "stop,restart,enable,boot,disable,install,uninstall"},
			} {
				s := State{Status: service.CoreStatus{ServiceUnit: "example.service", ServiceInstalled: status.installed, ServiceRunning: status.running}}
				card, controls, _ := s.systemdPanel(width, 20)
				rows := strings.Split(ansi.Strip(card), "\n")
				var actions []string
				for _, control := range controls {
					action := strings.TrimPrefix(control.action, "service:")
					actions = append(actions, action)
					label := action
					if action == "install" && status.installed {
						label = "reinstall"
					}
					cell := ansi.Cut(rows[control.y], control.x, control.x+control.width)
					if got, want := strings.TrimSpace(cell), i18n.T("core.service.quick."+label); got != want {
						t.Fatalf("%s width %d: hitbox for %s found %q, want %q", language, width, action, got, want)
					}
				}
				if strings.Join(actions, ",") != status.actions {
					t.Fatalf("controls for installed=%t running=%t: %v", status.installed, status.running, actions)
				}
				for _, row := range rows {
					if lipgloss.Width(row) > width {
						t.Fatalf("%s width %d: service card overflowed: %q", language, width, row)
					}
				}
				s.Busy = true
				if _, hits, _ := s.systemdPanel(width, 20); len(hits) != 0 {
					t.Fatal("service buttons are still clickable during a task")
				}
			}
		}
	}
}
