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

func TestDownloadModalPreservesLayoutAndCapturesInteraction(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	s := releaseFixture(t).selectRelease(2)
	before := s.layout(120, 29)
	s, _ = s.releaseAction("install")
	defer s.Close()
	s.Progress = coreinfra.Progress{Route: "direct", Downloaded: 1 << 20, Total: 4 << 20}
	after := s.layout(120, 29)
	if len(before.releases) != len(after.releases) || before.releases[0] != after.releases[0] {
		t.Fatal("download progress shifted the original release layout")
	}
	view := ansi.Strip(s.Render(120, 30))
	for _, want := range []string{"Task progress", "1.0 MiB / 4.0 MiB", "25%"} {
		if !strings.Contains(view, want) {
			t.Fatalf("modal missing %q", want)
		}
	}
	lines := strings.Split(view, "\n")
	if !strings.Contains(lines[1], "Versions") {
		t.Fatal("modal replaced the original header")
	}
	for _, key := range []string{"down", "3", "e", "enter"} {
		next, cmd := s.Update(coreKey(key))
		if next.selected != s.selected || next.tab != s.tab || next.Editing || cmd != nil {
			t.Fatalf("modal allowed underlying key %s", key)
		}
	}
	hit := before.tabs[2]
	next, _ := s.Update(messages.PageMouseClickMsg{X: hit.x, Y: hit.y, Width: 120, Height: 30})
	if next.tab != s.tab {
		t.Fatal("modal allowed underlying mouse click")
	}
	cancelled := false
	cancel := s.cancel
	s.cancel = func() { cancelled = true; cancel() }
	s, _ = s.Update(coreKey("esc"))
	if !cancelled || !s.TaskModal() {
		t.Fatal("Esc did not cancel the download while waiting for its result")
	}
	s, _ = s.Update(messages.CoreOperationMsg{ID: s.generation, Action: "install", Err: context.Canceled})
	if !strings.Contains(ansi.Strip(s.Render(120, 30)), "Task cancelled") {
		t.Fatal("cancel result was not displayed in the modal")
	}
	s, _ = s.Update(coreKey("enter"))
	if s.TaskModal() || s.Err != nil || s.Progress != (coreinfra.Progress{}) {
		t.Fatal("closing the modal did not clean up its state")
	}
}

func TestDownloadModalResultsAndTerminalBounds(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{12, 24, 80, 120} {
			for _, height := range []int{4, 8, 24} {
				for _, result := range []error{nil, errors.New(strings.Repeat("download failed ", 50))} {
					s := releaseFixture(t)
					s, _ = s.releaseAction("install")
					s.Close()
					s, _ = s.Update(messages.CoreOperationMsg{ID: s.generation, Action: "install", Err: result})
					view := s.Render(width, height)
					if lipgloss.Height(view) != height {
						t.Fatal("modal exceeds terminal height")
					}
					for _, line := range strings.Split(view, "\n") {
						if lipgloss.Width(line) > width {
							t.Fatalf("modal exceeds terminal width: %q", line)
						}
					}
					if width >= 80 && height >= 24 {
						want := i18n.T("core.task.complete")
						if result != nil {
							want = "download failed"
						}
						if !strings.Contains(ansi.Strip(view), want) {
							t.Fatal("task result is missing from popup")
						}
					}
				}
			}
		}
	}
}
