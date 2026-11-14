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

func TestAdvancedDialogsCancelWithoutOperationsAndRestoreSelection(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()
	for i, item := range (State{tab: 2}).actions() {
		s := NewState(svc).selectTab(2)
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

func TestNetworkSelectorsUseCurrentValueAndRequireExplicitChange(t *testing.T) {
	for _, action := range []string{"system-proxy", "tun"} {
		for _, enabled := range []bool{false, true} {
			s := State{Status: service.CoreStatus{SystemProxy: enabled, TUN: enabled}}
			s, _ = s.choose(action)
			if (s.choice == 1) != enabled {
				t.Fatal("selector does not start at the current value")
			}
			unchanged, cmd := s.Update(coreKey("enter"))
			if unchanged.Editing || unchanged.Busy || cmd != nil {
				t.Fatal("accepting the current value repeated the operation")
			}
			s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
			s, cmd = s.Update(coreKey("enter"))
			defer s.Close()
			if s.Editing || !s.Busy || s.task != action || cmd == nil || s.args != nil {
				t.Fatal("confirmed value was not queued with the dialog cleared")
			}
		}
	}
}

func TestAdvancedDialogsPreserveBackgroundAndMouseMatchesVisibleControls(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, action := range []string{"service", "system-proxy", "tun", "configure", "recover"} {
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
					// Clicking a background tab must not leave or execute an action.
					next, cmd := s.Update(messages.PageMouseClickMsg{X: 3, Y: 1, Width: width, Height: height})
					if next.tab != 2 || !next.Editing || next.Busy || cmd != nil {
						t.Fatal("background tab received the click")
					}
					page, _ := s.formDialog(width, height)
					if action == "service" {
						hit := page.actions[s.selected]
						next, cmd = s.Update(messages.PageMouseClickMsg{X: hit.x, Y: hit.y, Width: width, Height: height})
					} else {
						hit := page.controls[len(page.controls)-1]
						next, cmd = s.Update(messages.PageMouseClickMsg{X: hit.x, Y: hit.y, Width: width, Height: height})
						if action == "system-proxy" || action == "tun" {
							if next.choice != 1 || next.Busy || cmd != nil {
								t.Fatal("option click did not change only the pending value")
							}
							continue
						}
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
