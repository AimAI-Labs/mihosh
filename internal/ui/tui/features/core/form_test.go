package core

import (
	"runtime"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/preferences"
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
	if next, _ := s.Update(coreKey("e")); next.Editing {
		t.Fatal("download settings remained in Advanced")
	}
	s, _ = s.Update(coreKey("2"))
	page := s.layout(120, 30-footerHeight)
	hit := page.controls[0].actionHit
	s, _ = s.Update(messages.PageMouseClickMsg{X: hit.x, Y: hit.y, Width: 120, Height: 30})
	if s.tab != 1 || !s.Editing || s.form != "network" || !s.Busy || s.task != "install" || s.generation != id {
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
	for _, action := range []string{"configure", "recover"} {
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
		hit := page.controls[0].actionHit
		s, cmd = s.Update(messages.PageMouseClickMsg{X: hit.x, Y: hit.y, Width: 120, Height: 30})
		if cmd != nil || !s.Editing || !s.Busy || s.generation != id || s.args != nil {
			t.Fatal("mouse confirmation changed the pending download")
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
		if _, isSwitch := s.switchValue(item.action); isSwitch || item.action == "setup" || item.action == "service" {
			continue // Two-option settings toggle inline; service jumps to Mihomo service status card.
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

	actions := []string{"shell-proxy"}
	if runtime.GOOS == "linux" {
		actions = append(actions, "system-proxy", "tun")
	}
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{24, 40, 80, 120} {
			for _, height := range []int{12, 24, 30} {
				for _, action := range actions {
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
		for _, action := range []string{"configure", "recover"} {
			for _, width := range []int{24, 80, 120} {
				for _, height := range []int{8, 24, 30} {
					s := State{tab: 2, selected: 2}
					s, _ = s.choose(action)
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
					hit := page.controls[0]
					next, cmd = s.Update(messages.PageMouseClickMsg{X: hit.x, Y: hit.y, Width: width, Height: height})
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
			s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
			after, _ = s.formDialog(width, 10)
			if after.actions[s.focus].height == 0 {
				t.Fatal("keyboard focus remained outside the scrolled dialog")
			}
			s, cmd := s.Update(messages.PageMouseClickMsg{X: 0, Y: 0, Width: width, Height: 10})
			if s.Editing || s.selected != 1 || s.scroll != 3 || s.fields != nil || s.dialogScroll != 0 || cmd != nil {
				t.Fatal("outside close did not restore and clear dialog state")
			}
		}
	}
}

func TestServiceStatusCardKeyboardNavigationAndJumpShortcut(t *testing.T) {
	i18n.Init()
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()

	s := NewState(svc, nil, nil).selectTab(2)
	s.viewWidth, s.viewHeight = 100, 26
	s.Status = service.CoreStatus{
		Mode:             "managed",
		Version:          "v1.2.3",
		Running:          true,
		ServiceUnit:      "mihosh-example.service",
		ServiceInstalled: true,
		ServiceRunning:   true,
	}
	s, _ = s.Update(messages.CoreServiceLogMsg{
		Unit:   s.Status.ServiceUnit,
		Output: "● mihosh-example.service\nline1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10",
	})

	// Pressing 'h' on Runtime & Proxy jumps to Mihomo Config tab and focuses the service status card.
	s, _ = s.Update(coreKey("h"))
	if s.tab != 4 || s.Editing || !s.serviceFocus || s.serviceActionIdx != 0 {
		t.Fatalf("expected 'h' to jump to Mihomo service status card, got tab=%d editing=%t focus=%t idx=%d", s.tab, s.Editing, s.serviceFocus, s.serviceActionIdx)
	}

	// Arrow keys move across all available quick actions without leaving the card.
	actions := s.serviceQuickActions()
	for i := 1; i < len(actions); i++ {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
		if s.tab != 4 || !s.serviceFocus || s.serviceActionIdx != i {
			t.Fatalf("right arrow did not move service focus to %d (got %d)", i, s.serviceActionIdx)
		}
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if s.serviceActionIdx != len(actions)-2 {
		t.Fatalf("left arrow did not move service focus back, got %d", s.serviceActionIdx)
	}

	// Down/Up scroll service logs; Up at top returns focus to Mihomo config editor.
	s, _ = s.Update(coreKey("down"))
	if s.serviceScroll != 1 || !s.serviceFocus {
		t.Fatalf("down arrow did not scroll service logs, scroll=%d focus=%t", s.serviceScroll, s.serviceFocus)
	}
	s, _ = s.Update(coreKey("up"))
	if s.serviceScroll != 0 || !s.serviceFocus {
		t.Fatalf("up arrow did not scroll service logs back to top, scroll=%d focus=%t", s.serviceScroll, s.serviceFocus)
	}
	s, _ = s.Update(coreKey("up"))
	if s.serviceFocus {
		t.Fatal("up arrow at top of service logs should return focus to Mihomo config editor")
	}

	// Tab focuses the service card again and Enter triggers the focused service action.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !s.serviceFocus {
		t.Fatal("Tab did not focus the service status card")
	}
	s.serviceActionIdx = 1 // restart when installed & running
	next, cmd := s.Update(coreKey("enter"))
	defer next.Close()
	if next.Editing || !next.Busy || next.task != "service" || cmd == nil {
		t.Fatal("Enter on focused service quick action did not queue service task")
	}

	// During a busy task, Enter on the service card does not start a concurrent operation.
	busy := s
	busy.Busy, busy.task = true, "install"
	busyNext, busyCmd := busy.Update(coreKey("enter"))
	if busyCmd != nil || busyNext.task != "install" {
		t.Fatal("service card queued a concurrent action while busy")
	}

	// Escape returns focus to the Mihomo config panel.
	s, _ = s.Update(coreKey("esc"))
	if s.serviceFocus {
		t.Fatal("Esc did not return focus from service card")
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

func TestMaintenanceAPIActionsRequireConfirmation(t *testing.T) {
	i18n.Init()
	client := api.NewClient(config.DefaultMihomoEndpoint, 1000)
	for _, mode := range []string{"managed", "external"} {
		initial := State{tab: 2, connected: true, Status: service.CoreStatus{Mode: mode}, editor: preferences.NewState(nil, client)}
		for index, item := range initial.actions() {
			id, ok := strings.CutPrefix(item.action, "api:")
			if !ok {
				continue
			}
			t.Run(mode+"/"+id, func(t *testing.T) {
				s := initial
				s.selected = index
				s, cmd := s.Update(coreKey("enter"))
				if !s.Editing || s.form != item.action || cmd != nil {
					t.Fatal("maintenance action did not wait for confirmation")
				}
				desc := i18n.T("core.desc." + id)
				if strings.HasPrefix(desc, "core.desc.") || !strings.Contains(ansi.Strip(s.choiceLayout(120).content), desc) {
					t.Fatalf("missing localized confirmation for %s", id)
				}
				s, cmd = s.Update(coreKey("esc"))
				if s.Editing || cmd != nil || s.selected != index {
					t.Fatal("cancelling did not return to the selected action")
				}
				s, _ = s.Update(coreKey("enter"))
				s, cmd = s.Update(coreKey("enter"))
				if s.Editing || cmd == nil {
					t.Fatal("confirming did not queue the API action")
				}
			})
		}
	}
}

func TestInvalidatingCoreTargetClearsPendingMaintenanceActions(t *testing.T) {
	client := api.NewClient(config.DefaultMihomoEndpoint, 1000)
	s := State{editor: preferences.NewState(nil, client)}
	s.editor, _ = s.editor.RunCoreAction("restart")
	if !s.editor.IsCoreRestarting {
		t.Fatal("restart was not pending")
	}
	s, _ = s.Update(messages.CoreTargetChangedMsg{Invalidate: true})
	if s.editor.IsCoreRestarting {
		t.Fatal("restart remained pending on the new target generation")
	}
}

func TestCoreDialogsRenderHintsAtBottomRightNotInsideBox(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	if err := config.Save(&config.DefaultConfig); err != nil {
		t.Fatal(err)
	}
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	svc := service.NewCoreService()
	defer svc.Close()

	forms := []string{"stop", "restart", "recover", "configure", "basic-config", "api:reload", "geo-update", "external", "network", "version", "import", "details"}
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		cancelHint := "Esc " + i18n.T("core.inline.cancel")
		runHint := "Enter " + i18n.T("core.inline.run")
		saveHint := "Enter " + i18n.T("core.inline.save")
		tabHint := "Tab " + i18n.T("core.inline.field")
		scrollHint := "↑↓ " + i18n.T("core.inline.scroll")

		for _, form := range forms {
			for _, width := range []int{24, 40, 80, 120} {
				for _, height := range []int{8, 16, 30} {
					s := NewState(svc, nil, nil).selectTab(2)
					s.connected = true
					s.Status = service.CoreStatus{
						Mode:       "managed",
						Version:    "v1.2.3",
						Running:    true,
						BinaryPath: "/core/mihomo",
						ConfigPath: "/runtime/config.yaml",
						Controller: "127.0.0.1:9090",
						MixedPort:  7890,
					}
					s.geoFiles = []service.GeoDataFile{{Key: "mmdb", Name: "Country.mmdb", Updatable: true}}
					s, _ = s.editForm(form)

					page, _ := s.formDialog(width, height)
					strippedBox := ansi.Strip(page.content)
					for _, hintText := range []string{cancelHint, runHint, saveHint, tabHint, scrollHint} {
						if strings.Contains(strippedBox, hintText) {
							t.Fatalf("%s %s %dx%d: dialog box should not contain bottom hint %q:\n%s", language, form, width, height, hintText, strippedBox)
						}
					}

					hints := s.InlineHelpHints()
					if len(hints) == 0 || hints[len(hints)-1].Key != "Esc" {
						t.Fatalf("%s %s: expected Esc hint at end of InlineHelpHints, got %+v", language, form, hints)
					}

					_, _, _, bottom := formDialogPlacement(page.content, width, height).Bounds()
					if bottom > max(1, height-1) {
						t.Fatalf("%s %s %dx%d: dialog bottom %d overlaps bottom hint row %d", language, form, width, height, bottom, height-1)
					}

					rendered := ansi.Strip(s.Render(width, height))
					lines := strings.Split(rendered, "\n")
					if len(lines) != height {
						t.Fatalf("%s %s %dx%d: got %d lines", language, form, width, height, len(lines))
					}
					for _, line := range lines {
						if lipgloss.Width(line) > width {
							t.Fatalf("%s %s %dx%d: line overflow %q", language, form, width, height, line)
						}
					}
					if !strings.HasSuffix(lines[height-1], cancelHint) || lipgloss.Width(lines[height-1]) > width-1 {
						t.Fatalf("%s %s %dx%d: bottom-right row missing %q: %q", language, form, width, height, cancelHint, lines[height-1])
					}

					if isChoiceForm(form) && width >= 40 && height >= 16 {
						if len(page.controls) < 2 {
							t.Fatalf("%s %s: missing choice controls: %+v", language, form, page.controls)
						}
						confirmHit := page.controls[0].actionHit
						cancelHit := page.controls[1].actionHit
						gotConfirm := strings.TrimSpace(ansi.Cut(lines[confirmHit.y], confirmHit.x, confirmHit.x+confirmHit.width))
						gotCancel := strings.TrimSpace(ansi.Cut(lines[cancelHit.y], cancelHit.x, cancelHit.x+cancelHit.width))
						if gotConfirm != i18n.T("core.dialog.confirm") || gotCancel != i18n.T("core.dialog.cancel") {
							t.Fatalf("%s %s %dx%d: choice hitboxes mismatch rendered buttons: got (%q, %q)", language, form, width, height, gotConfirm, gotCancel)
						}
					}
				}
			}
		}

		// Verify clicking either the field label or input row focuses that form field.
		s, _ := NewState(svc, nil, nil).editForm("external")
		page, _ := s.formDialog(80, 24)
		if len(page.actions) < 2 || page.actions[1].height != 2 {
			t.Fatalf("%s: expected external field hitbox height 2, got %+v", language, page.actions)
		}
		for _, dy := range []int{0, 1} {
			testState := s
			hit := page.actions[1]
			testState, _ = testState.Update(messages.PageMouseClickMsg{X: hit.x + 1, Y: hit.y + dy, Width: 80, Height: 24})
			if testState.focus != 1 {
				t.Fatalf("%s dy=%d: clicking field row did not focus field 1 (got %d)", language, dy, testState.focus)
			}
		}
	}
}
