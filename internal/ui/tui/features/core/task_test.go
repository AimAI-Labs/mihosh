package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestCoreTaskFooterPreservesViewport(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{80, 120} {
			for _, height := range []int{12, 30} {
				for tab := range tabs {
					for _, action := range []string{"import", "install", "releases"} {
						s := releaseFixture(t).selectTab(tab)
						s.scroll, s.catalog.scroll, s.followSelection = 4, 2, false
						before := strings.Split(ansi.Strip(s.Render(width, height)), "\n")
						s, _ = s.begin(action)
						s.cancel()
						s.Progress = coreinfra.Progress{Route: "direct", Downloaded: 1 << 20, Total: 4 << 20}
						for _, busy := range []bool{true, false} {
							if !busy {
								s, _ = s.Update(messages.CoreOperationMsg{ID: s.generation, Action: action, Err: errors.New("task failed")})
							}
							after := strings.Split(ansi.Strip(s.Render(width, height)), "\n")
							if strings.Join(before[:height-4], "\n") != strings.Join(after[:height-4], "\n") {
								t.Fatalf("%s %dx%d tab=%d action=%s busy=%v: task moved or covered the viewport", language, width, height, tab, action, busy)
							}
							footer := strings.Join(after[height-4:], "\n")
							if busy && action == "releases" {
								if strings.Contains(footer, i18n.T("core.task")) || strings.Contains(footer, i18n.T("core.action.releases")) {
									t.Fatal("background catalogue load displayed a task notice")
								}
								continue
							}
							want := i18n.T("core.task")
							if !busy {
								want = "task failed"
							}
							if !strings.Contains(footer, want) {
								t.Fatalf("task status missing from footer: %q", footer)
							}
						}
						s.Close()
					}
				}
			}
		}
	}
}

func TestTaskFooterAllowsBrowsingAndCancelsWithoutStartingAnotherTask(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	s := releaseFixture(t).selectRelease(2)
	before := s.layout(120, 26)
	s, _ = s.releaseAction("install")
	defer s.Close()
	s.Progress = coreinfra.Progress{Route: "direct", Downloaded: 1 << 20, Total: 4 << 20}
	after := s.layout(120, 26)
	if len(before.releases) != len(after.releases) || before.releases[0] != after.releases[0] {
		t.Fatal("download progress shifted the original release layout")
	}
	view := ansi.Strip(s.Render(120, 30))
	for _, want := range []string{"Task progress", "1.0 MiB / 4.0 MiB", "25%"} {
		if !strings.Contains(view, want) {
			t.Fatalf("footer missing %q", want)
		}
	}
	lines := strings.Split(view, "\n")
	if !strings.Contains(lines[1], "Versions") {
		t.Fatal("task replaced the original header")
	}
	s, _ = s.Update(coreKey("down"))
	if s.selected != 3 {
		t.Fatal("running task prevented keyboard browsing")
	}
	hit := before.releases[1]
	s, _ = s.Update(messages.PageMouseClickMsg{X: hit.x, Y: hit.y, Width: 120, Height: 30})
	if s.selected != hit.index {
		t.Fatal("running task prevented selecting a version by mouse")
	}
	s, _ = s.Update(messages.PageMouseScrollMsg{Width: 80, Height: 12})
	if s.catalog.scroll == 0 || s.followSelection {
		t.Fatal("running task prevented scrolling the version list")
	}
	for _, key := range []string{"i", "a", "g", "enter"} {
		next, cmd := s.Update(coreKey(key))
		if next.generation != s.generation || next.Editing || cmd != nil {
			t.Fatalf("running task allowed another action via %s", key)
		}
	}
	s, _ = s.Update(coreKey("3"))
	if s.tab != 2 || !s.Busy {
		t.Fatal("running task prevented changing sections by keyboard")
	}
	hit = before.tabs[0]
	s, _ = s.Update(messages.PageMouseClickMsg{X: hit.x, Y: hit.y, Width: 120, Height: 30})
	if s.tab != 0 || !s.Busy {
		t.Fatal("running task prevented changing sections by mouse")
	}
	cancelled := false
	cancel := s.cancel
	s.cancel = func() { cancelled = true; cancel() }
	s, _ = s.Update(coreKey("esc"))
	if !cancelled || !s.Busy || s.task == "" {
		t.Fatal("Esc did not cancel the download while waiting for its result")
	}
	s, _ = s.Update(messages.CoreOperationMsg{ID: s.generation, Action: "install", Err: context.Canceled})
	if !strings.Contains(ansi.Strip(s.Render(120, 30)), "Task cancelled") {
		t.Fatal("cancel result was not displayed in the footer")
	}
	s, cmd := s.Update(coreKey("enter"))
	if s.task != "" || s.Err != nil || s.Progress != (coreinfra.Progress{}) || cmd != nil {
		t.Fatal("dismissing the result did not clean up its state or started another task")
	}
}

func TestTaskFooterResultsAndTerminalBounds(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{1, 12, 24, 80, 120} {
			for _, height := range []int{1, 2, 4, 8, 24} {
				for _, result := range []error{nil, context.Canceled, errors.New(strings.Repeat("download failed ", 50))} {
					s := releaseFixture(t)
					s, _ = s.releaseAction("install")
					s.Close()
					s, _ = s.Update(messages.CoreOperationMsg{ID: s.generation, Action: "install", Err: result})
					view := s.Render(width, height)
					if lipgloss.Height(view) != height {
						t.Fatal("task footer exceeds terminal height")
					}
					for _, line := range strings.Split(view, "\n") {
						if lipgloss.Width(line) > width {
							t.Fatalf("task footer exceeds terminal width: %q", line)
						}
					}
					if width >= 80 && height >= 24 {
						want := i18n.T("core.task.complete")
						if errors.Is(result, context.Canceled) {
							want = i18n.T("core.task.cancelled")
						} else if result != nil {
							want = "download failed"
						}
						if !strings.Contains(ansi.Strip(view), want) {
							t.Fatal("task result is missing from footer")
						}
						if result == nil || errors.Is(result, context.Canceled) {
							lines := strings.Split(ansi.Strip(view), "\n")
							if !strings.HasPrefix(lines[height-1], i18n.T("core.action.install")+" · "+want) || !strings.HasSuffix(lines[height-1], "Enter/Esc "+i18n.T("core.inline.dismiss")) {
								t.Fatal("task result and controls do not share the bottom row")
							}
						}
					}
					for _, key := range []string{"enter", "esc"} {
						dismissed, cmd := s.Update(coreKey(key))
						if dismissed.task != "" || dismissed.Err != nil || cmd != nil {
							t.Fatalf("%s did not dismiss the result", key)
						}
					}
				}
			}
		}
	}
}
