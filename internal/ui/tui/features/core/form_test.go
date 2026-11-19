package core

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestDownloadAllowsNetworkPreferencesWithoutReplacingActiveTask(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	if err := config.Save(&config.DefaultConfig); err != nil {
		t.Fatal(err)
	}
	svc := service.NewCoreService()
	defer svc.Close()
	s := releaseFixture(t)
	s.svc = svc
	s, _ = s.begin("install")
	defer s.Close()
	id := s.generation
	s.Progress = coreinfra.Progress{Downloaded: 1, Total: 2}
	s, _ = s.Update(coreKey("3"))
	s = selectTestAction(t, s, "network")
	s, _ = s.Update(coreKey("enter"))
	if s.tab != 2 || !s.Editing || s.form != "network" || !s.Busy || s.task != "install" || s.generation != id {
		t.Fatal("download blocked settings or lost its active task")
	}
	s.fields[1].SetValue("https://mirror.example")
	s, _ = s.Update(coreKey("enter"))
	if s.Editing || !s.Busy || s.task != "install" || s.generation != id || s.Progress.Downloaded != 1 {
		t.Fatal("saving preferences replaced the active download")
	}
	cfg, err := config.Load()
	if err != nil || len(cfg.Core.Accelerators) != 1 || cfg.Core.Accelerators[0].URL != "https://mirror.example" {
		t.Fatal("network preferences were not saved", err)
	}
	s, _ = s.Update(messages.CoreOperationMsg{ID: id, Action: "install"})
	if s.Busy {
		t.Fatal("download completion was lost")
	}
}

func TestDownloadAllowsInspectingAdvancedDialogsWithoutConcurrentOperations(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()
	for _, action := range []string{"service", "configure", "recover"} {
		s := releaseFixture(t)
		s.svc = svc
		s, _ = s.begin("install")
		defer s.Close()
		id := s.generation
		s, _ = s.Update(coreKey("3"))
		s, _ = s.choose(action)
		if !s.Editing || s.form != action || !s.Busy || s.task != "install" {
			t.Fatalf("download blocked viewing %s", action)
		}
		s, cmd := s.Update(coreKey("enter"))
		if cmd != nil || !s.Editing || !s.Busy || s.generation != id || s.args != nil {
			t.Fatal("pending download allowed a concurrent operation")
		}
		page, _ := s.formDialog(120, 30)
		if action == "service" || action == "configure" || action == "recover" {
			var hit actionHit
			if action == "service" {
				hit = page.actions[s.selected]
			} else {
				hit = page.controls[0].actionHit
			}
			s, cmd = s.Update(messages.PageMouseClickMsg{X: hit.x, Y: hit.y, Width: 120, Height: 30})
			if cmd != nil || !s.Editing || !s.Busy || s.generation != id || s.args != nil {
				t.Fatal("mouse confirmation changed the pending download")
			}
		}
		s, _ = s.Update(messages.CoreOperationMsg{ID: id, Action: "install"})
		if s.Busy || !s.Editing {
			t.Fatal("download completion closed the inspected dialog")
		}
		s, _ = s.Update(coreKey("esc"))
		if s.Editing || s.task != "install" {
			t.Fatal("closing settings discarded the download result")
		}
	}
}

func TestAdvancedDialogsCancelWithoutOperationsAndRestoreSelection(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()
	for i, item := range (State{tab: 2}).actions() {
		if item.action == "mihomo" {
			t.Fatal("duplicate mihomo action must not appear on the advanced tab")
		}
		s := NewState(svc, nil, nil).selectTab(2)
		if _, isSwitch := s.switchValue(item.action); isSwitch || item.action == "setup" {
			continue // Two-option settings toggle inline without opening a dialog.
		}
		s.selected, s.scroll = i, 3
		s, _ = s.choose(item.action)
		if !s.Editing || s.form != item.action || s.Busy {
			t.Fatalf("%s did not open a dialog", item.action)
		}
		for _, key := range []string{"3", "q", "y"} {
			s, _ = s.Update(coreKey(key))
			if !s.Editing || s.Busy || s.tab != 2 {
				t.Fatalf("%s dialog allowed underlying shortcut %s", item.action, key)
			}
		}
		s, cmd := s.Update(coreKey("esc"))
		if s.Editing || s.Busy || s.fields != nil || s.args != nil || s.form != "" || s.selected != i || s.scroll != 3 || cmd != nil {
			t.Fatalf("%s dialog did not cancel and restore its parent", item.action)
		}
	}
}

func TestAdvancedInlineSwitchesRenderAndApplyDirectTargets(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	for _, item := range (State{tab: 2}).actions() {
		if item.action == "mihomo" {
			t.Fatal("duplicate mihomo entry remained on the advanced tab")
		}
	}

	svc := service.NewCoreService()
	defer svc.Close()

	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{24, 40, 80, 120} {
			for _, height := range []int{12, 24, 30} {
				for _, action := range []string{"system-proxy", "tun", "shell-proxy"} {
					for _, enabled := range []bool{false, true} {
						s := State{svc: svc, tab: 2, followSelection: true, Status: service.CoreStatus{
							Mode:        "managed",
							Version:     "v1",
							SystemProxy: enabled,
							TUN:         enabled,
							ShellProxy:  enabled,
						}}
						s = selectTestAction(t, s, action)
						view := ansi.Strip(s.Render(width, height))
						if lipgloss.Height(view) != height {
							t.Fatalf("%s %dx%d: inline switch view exceeded height", language, width, height)
						}
						for _, line := range strings.Split(view, "\n") {
							if lipgloss.Width(line) > width {
								t.Fatalf("%s %dx%d: inline switch line overflowed width: %q", language, width, height, line)
							}
						}
						if !strings.Contains(view, i18n.T("core.disabled")) || !strings.Contains(view, i18n.T("core.enabled")) {
							t.Fatalf("%s %dx%d: inline switch labels missing on advanced tab", language, width, height)
						}

						// Clicking the currently active pill is a no-op and never opens a dialog.
						contentHeight := max(1, height-footerHeight)
						page := s.layout(width, contentHeight)
						offset := s.viewportOffset(page, contentHeight)
						activeIdx := 0
						if enabled {
							activeIdx = 1
						}
						var activeHit, inactiveHit actionHit
						for _, ctrl := range page.controls {
							if ctrl.action == "switch:"+action {
								if ctrl.index == activeIdx {
									activeHit = ctrl.actionHit
								} else {
									inactiveHit = ctrl.actionHit
								}
							}
						}
						if activeHit.width == 0 || inactiveHit.width == 0 {
							t.Fatalf("%s %dx%d: switch hitboxes missing for %s", language, width, height, action)
						}
						viewLines := strings.Split(view, "\n")
						if activeHit.y-offset >= 0 && activeHit.y-offset < contentHeight {
							activeRow := activeHit.y - offset
							expectedLabel := i18n.T("core.disabled")
							if enabled {
								expectedLabel = i18n.T("core.enabled")
							}
							if gotSlice := ansi.Cut(viewLines[activeRow], activeHit.x, activeHit.x+activeHit.width); !strings.Contains(gotSlice, expectedLabel) {
								t.Fatalf("%s %dx%d: hitbox %+v missed rendered active label %q (got %q)", language, width, height, activeHit, expectedLabel, gotSlice)
							}
							same, sameCmd := s.Update(messages.PageMouseClickMsg{X: activeHit.x, Y: activeRow, Width: width, Height: height})
							if same.Editing || same.Busy || sameCmd != nil {
								t.Fatalf("%s %s: clicking active switch pill triggered an operation", language, action)
							}
						}
						if inactiveHit.y-offset >= 0 && inactiveHit.y-offset < contentHeight {
							inactiveRow := inactiveHit.y - offset
							expectedLabel := i18n.T("core.enabled")
							if enabled {
								expectedLabel = i18n.T("core.disabled")
							}
							if gotSlice := ansi.Cut(viewLines[inactiveRow], inactiveHit.x, inactiveHit.x+inactiveHit.width); !strings.Contains(gotSlice, expectedLabel) {
								t.Fatalf("%s %dx%d: hitbox %+v missed rendered inactive label %q (got %q)", language, width, height, inactiveHit, expectedLabel, gotSlice)
							}
							toggled, toggledCmd := s.Update(messages.PageMouseClickMsg{X: inactiveHit.x, Y: inactiveRow, Width: width, Height: height})
							toggled.Close()
							if toggled.Editing || !toggled.Busy || toggled.task != action || toggledCmd == nil || toggled.args != nil {
								t.Fatalf("%s %s: clicking target switch pill did not queue direct toggle", language, action)
							}
						}

						// Pressing Enter / Space / choose directly toggles without opening a modal.
						direct, directCmd := s.choose(action)
						direct.Close()
						if direct.Editing || !direct.Busy || direct.task != action || directCmd == nil || direct.args != nil {
							t.Fatalf("%s %s: choose did not toggle directly without modal", language, action)
						}
					}
				}
			}
		}
	}
}

func TestAdvancedDialogsPreserveBackgroundAndMouseMatchesVisibleControls(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, action := range []string{"service", "configure", "recover"} {
			for _, width := range []int{24, 80, 120} {
				for _, height := range []int{8, 24, 30} {
					s := State{tab: 2, selected: 2}
					s, _ = s.choose(action)
					if action == "service" {
						s.selected = len(serviceActions) - 1
					}
					view := ansi.Strip(s.Render(width, height))
					if lipgloss.Height(view) != height {
						t.Fatal("dialog exceeds height")
					}
					for _, line := range strings.Split(view, "\n") {
						if lipgloss.Width(line) > width {
							t.Fatalf("%s %s %dx%d: dialog exceeds width: %q", language, action, width, height, line)
						}
					}
					if width == 120 && height == 30 && !strings.Contains(strings.Split(view, "\n")[1], i18n.T("core.tab.advanced")) {
						t.Fatal("dialog replaced the original page")
					}
					// Outside clicks dismiss without activating the page underneath.
					next, cmd := s.Update(messages.PageMouseClickMsg{X: 0, Y: 0, Width: width, Height: height})
					if next.tab != 2 || next.Editing || next.Busy || cmd != nil {
						t.Fatal("outside click did not dismiss safely")
					}
					page, _ := s.formDialog(width, height)
					if action == "service" {
						hit := page.actions[s.selected]
						next, cmd = s.Update(messages.PageMouseClickMsg{X: hit.x, Y: hit.y, Width: width, Height: height})
					} else {
						hit := page.controls[0]
						next, cmd = s.Update(messages.PageMouseClickMsg{X: hit.x, Y: hit.y, Width: width, Height: height})
					}
					defer next.Close()
					if next.Editing || !next.Busy || cmd == nil {
						t.Fatal("visible dialog action did not execute")
					}
				}
			}
		}
	}
}

func TestCoreDialogsScrollIndependentlyAndRestoreParentOnOutsideClick(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	svc := service.NewCoreService()
	defer svc.Close()
	for _, action := range []string{"network", "external"} {
		for _, width := range []int{24, 80, 120} {
			s := NewState(svc, nil, nil).selectTab(2)
			s.selected, s.scroll = 1, 3
			s, _ = s.editForm(action)
			parentScroll := s.scroll
			before, _ := s.formDialog(width, 10)
			wheel := messages.PageMouseScrollMsg{X: width / 2, Y: 5, Width: width, Height: 10}
			s, _ = s.Update(wheel)
			after, _ := s.formDialog(width, 10)
			if before.maxScroll <= 0 || after.scrollOffset != before.scrollOffset+1 || s.scroll != parentScroll || s.Busy {
				t.Fatalf("%s %d: dialog did not scroll independently", action, width)
			}
			for range 100 {
				s, _ = s.Update(wheel)
			}
			after, _ = s.formDialog(width, 10)
			if after.scrollOffset != after.maxScroll {
				t.Fatal("dialog did not clamp at its end")
			}
			if action == "service" {
				s, _ = s.Update(coreKey("down"))
			} else {
				s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
			}
			after, _ = s.formDialog(width, 10)
			focus := s.focus
			if action == "service" {
				focus = s.selected
			}
			if after.actions[focus].height == 0 {
				t.Fatal("keyboard focus remained outside the scrolled dialog")
			}
			s, cmd := s.Update(messages.PageMouseClickMsg{X: 0, Y: 0, Width: width, Height: 10})
			if s.Editing || s.selected != 1 || s.scroll != 3 || s.fields != nil || s.dialogScroll != 0 || cmd != nil {
				t.Fatal("outside close did not restore and clear dialog state")
			}
		}
	}
}

func TestServiceDialogWheelSelectsVisibleActions(t *testing.T) {
	i18n.Init()
	for _, width := range []int{24, 80, 120} {
		for _, height := range []int{10, 30} {
			s := State{tab: 2, selected: 1, scroll: 3}
			s, _ = s.editForm("service")
			wheel := messages.PageMouseScrollMsg{X: width / 2, Y: height / 2, Width: width, Height: height}
			for i := range len(serviceActions) + 2 {
				var cmd tea.Cmd
				s, cmd = s.Update(wheel)
				page, _ := s.formDialog(width, height)
				if s.selected != min(i+1, len(serviceActions)-1) || page.actions[s.selected].height == 0 || s.Busy || cmd != nil {
					t.Fatalf("%dx%d: wheel did not select a visible action", width, height)
				}
			}
			wheel.Up = true
			for range len(serviceActions) + 2 {
				s, _ = s.Update(wheel)
			}
			if s.selected != 0 {
				t.Fatal("wheel did not clamp at the first action")
			}
			wheel.X, wheel.Y, wheel.Up = 0, 0, false
			unchanged, _ := s.Update(wheel)
			if unchanged.selected != s.selected {
				t.Fatal("wheel outside dialog changed selection")
			}
			s, cmd := s.Update(messages.PageMouseClickMsg{X: 0, Y: 0, Width: width, Height: height})
			if s.Editing || s.selected != 1 || s.scroll != 3 || s.dialogScroll != 0 || cmd != nil {
				t.Fatal("closing service dialog did not restore parent")
			}
		}
	}
}
func TestStopAndRestartShortcutsRequireConfirmation(t *testing.T) {
	for shortcut, action := range map[string]string{"x": "stop", "z": "restart"} {
		t.Run(action, func(t *testing.T) {
			svc := service.NewCoreService()
			t.Cleanup(svc.Close)
			s := State{svc: svc, tab: 2, Status: service.CoreStatus{Mode: "managed", Version: "v1.2.3", Running: true}}
			s, cmd := s.Update(coreKey(shortcut))
			if !s.Editing || s.form != action || s.Busy || cmd != nil {
				t.Fatal("shortcut started the action before confirmation")
			}
			s, cmd = s.Update(coreKey("esc"))
			if s.Editing || s.Busy || cmd != nil {
				t.Fatal("cancel left the action pending")
			}
			s, _ = s.Update(coreKey(shortcut))
			s, cmd = s.Update(coreKey("enter"))
			defer s.Close()
			if s.Editing || !s.Busy || s.task != action || cmd == nil {
				t.Fatal("confirmation did not queue the requested action")
			}
		})
	}
}
