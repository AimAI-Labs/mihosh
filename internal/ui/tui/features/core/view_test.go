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
	s := NewState(svc)
	s.Status = service.CoreStatus{Mode: "managed", Version: "v1", Running: true}
	s, cmd := s.Update(coreKey("enter"))
	if !s.Editing || s.form != "profile" || s.Busy || cmd == nil {
		t.Fatal("a running core without nodes should offer the subscription form")
	}
	s, _ = s.Update(coreKey("esc"))
	s, _ = s.Update(coreKey("3"))
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

func TestCoreExistingConnectionOffersConnectionFormWithoutStartingCore(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()
	s := NewState(svc)
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
	s := State{followSelection: true}
	view := ansi.Strip(s.Render(120, 28))
	for _, label := range []string{"1 Daily use", "2 Versions", "3 Advanced"} {
		if strings.Contains(view, label) {
			t.Fatalf("section still has a numeric prefix: %q", label)
		}
	}
	for y, line := range strings.Split(view, "\n") {
		if at := strings.Index(line, "Advanced"); at >= 0 {
			next, cmd := s.Update(messages.PageMouseClickMsg{X: lipgloss.Width(line[:at]), Y: y, Width: 120, Height: 28})
			if next.tab != 2 || next.Busy || next.Editing || cmd != nil {
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
				for _, target := range []struct {
					index  int
					action string
				}{{0, "network"}, {4, "ports"}} {
					s := NewState(svc).selectTab(2)
					s.selected = target.index
					label := i18n.T("core.action." + target.action)
					view := ansi.Strip(s.Render(width, height))
					found := false
					for y, line := range strings.Split(view, "\n") {
						at := strings.Index(line, label)
						if at < 0 {
							continue
						}
						x := lipgloss.Width(line[:at]) + 1
						next, _ := s.Update(messages.PageMouseClickMsg{X: x, Y: y, Width: width, Height: height})
						if !next.Editing || next.form != target.action {
							t.Fatalf("%s %dx%d: click on visible %s did not open its form", language, width, height, target.action)
						}
						found = true
						break
					}
					if !found {
						t.Fatalf("%s %dx%d: selected %s is not visible:\n%s", language, width, height, target.action, view)
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
					s := NewState(svc).selectTab(tab)
					s.Status = service.CoreStatus{Mode: "managed", Version: "v1", Running: true, SystemProxy: true, TUN: true, RecoveryPending: true}
					s.Err = errors.New(strings.Repeat("a long error message ", 12))
					states = append(states, s)
				}
				for _, form := range []string{"network", "profile", "version", "import", "external", "ports", "service"} {
					s, _ := NewState(svc).editForm(form)
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
				t.Fatalf("help does not align with right margin: %q", lines[15])
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
