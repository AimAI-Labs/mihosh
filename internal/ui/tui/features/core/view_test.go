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
	for _, want := range []string{"欢迎使用", "一键开始使用", "已有内核", "版本", "内核路径"} {
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

func TestCoreMenuWheelSelectsAndKeepsActionsVisible(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{40, 80, 120} {
			for _, tab := range []int{0, 2} {
				s := State{tab: tab, followSelection: true}
				s.Status = service.CoreStatus{Mode: "managed", Version: "v1", Running: true}
				wheel := messages.PageMouseScrollMsg{X: width / 2, Y: 6, Width: width, Height: 16}
				for _, up := range []bool{false, true} {
					wheel.Up = up
					for range s.selectionCount() + 2 {
						want := min(s.selectionCount()-1, s.selected+1)
						if up {
							want = max(0, s.selected-1)
						}
						next, cmd := s.Update(wheel)
						s = next
						if s.selected != want || !s.followSelection || cmd != nil || s.Busy || s.Editing {
							t.Fatalf("%s width=%d tab=%d: wheel did not select %d without running it", language, width, tab, want)
						}
						height := wheel.Height - footerHeight
						page := s.layout(width, height)
						offset := s.viewportOffset(page, height)
						hit := page.actions[s.selected]
						if hit.y < offset || hit.y >= offset+height {
							t.Fatalf("%s width=%d tab=%d: selected action is outside the viewport", language, width, tab)
						}
					}
				}
			}
		}
	}
	s := State{tab: 2, followSelection: true}
	s = selectTestAction(t, s, "service")
	s.selected--
	s, _ = s.Update(messages.PageMouseScrollMsg{Width: 120, Height: 30})
	s, cmd := s.Update(coreKey("enter"))
	if !s.Editing || s.form != "service" || cmd != nil {
		t.Fatal("Enter did not open the menu item selected by the wheel")
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
		if at := strings.Index(line, "Advanced"); at >= 0 {
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

func TestDailyOverviewShowsCoreDetailsWithoutGuidance(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	s := State{Status: service.CoreStatus{Mode: "managed", Version: "v1.2.3", BinaryPath: "/core/mihomo", ConfigPath: "/runtime/config.yaml", Controller: "127.0.0.1:9090", MixedPort: 7890}}
	view := ansi.Strip(s.overview(70))
	for _, want := range []string{"v1.2.3", "/core/mihomo", "/runtime/config.yaml", "127.0.0.1:9090", "7890"} {
		if !strings.Contains(view, want) {
			t.Errorf("overview missing %q", want)
		}
	}
	if strings.Contains(view, i18n.T("core.hint.start")) {
		t.Fatal("daily overview still contains guidance")
	}
	s.Status.Mode = "external"
	s, _ = s.Update(messages.CoreConnectionMsg{Version: "v9.8.7"})
	view = ansi.Strip(s.overview(70))
	if !strings.Contains(view, "v9.8.7") || strings.Contains(view, "v1.2.3") {
		t.Fatal("external overview did not use the connected core version")
	}
	if !strings.Contains(view, "/core/mihomo") {
		t.Fatal("external executable path is not visible")
	}
	s.Status.BinaryPath = ""
	if !strings.Contains(ansi.Strip(s.overview(70)), i18n.T("core.path_unavailable")) {
		t.Fatal("unavailable executable path is not explained")
	}
}

func TestCoreMihomoPageShowsManagedUnitBelowOverrides(t *testing.T) {
	i18n.Init()
	s := State{tab: 4, Status: service.CoreStatus{
		Mode: "managed", ServiceUnit: "mihosh-example.service", ServiceInstalled: true, ServiceRunning: true, AutoStart: true,
	}}
	page := ansi.Strip(s.layout(100, 40).content)
	firstLine, _, _ := strings.Cut(page, "\n")
	if strings.Contains(firstLine, i18n.T("core.heading")) {
		t.Fatal("Core navigation has a panel title")
	}
	settings := strings.Index(page, i18n.T("settings.panel_title"))
	card := strings.Index(page, i18n.T("settings.sys_status.panel_title"))
	unit := strings.Index(page, "mihosh-example.service")
	if settings < 0 || card <= settings || unit <= card || !strings.Contains(page, i18n.T("core.service.status_running")) ||
		!strings.Contains(page, i18n.T("core.autostart")+" "+i18n.T("core.enabled")) {
		t.Fatal("managed service card does not show its unit, status and startup state")
	}
	wheel := messages.PageMouseScrollMsg{X: 30, Y: 6, Width: 80, Height: 13}
	for range 20 {
		s, _ = s.Update(wheel)
	}
	if !strings.Contains(ansi.Strip(s.Render(80, 13)), "mihosh-example.service") {
		t.Fatal("managed service card is unreachable in a short terminal")
	}
}
