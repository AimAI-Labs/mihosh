package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestBusyCoreDoesNotStartAnotherTaskAndIgnoresStaleResult(t *testing.T) {
	s := NewState(service.NewCoreService(), nil, nil)
	defer s.svc.Close()
	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	if !s.Busy || cmd == nil {
		t.Fatal("install not started")
	}
	defer s.Close()
	old := s.generation
	s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if cmd != nil || s.generation != old {
		t.Fatal("second task started")
	}
	s, _ = s.Update(messages.CoreOperationMsg{ID: old + 1, Err: errors.New("stale")})
	if !s.Busy || s.Err != nil {
		t.Fatal("stale result corrupted task")
	}
	s, cmd = s.Update(messages.CoreOperationMsg{ID: old})
	if s.Busy || cmd == nil {
		t.Fatal("completion not applied")
	}
}
func TestCoreViewFitsNarrowAndWideTerminals(t *testing.T) {
	i18n.Init()
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{24, 80, 120} {
			s := NewState(service.NewCoreService(), nil, nil)
			view := s.Render(width, 30)
			s.svc.Close()
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > width {
					t.Fatalf("language=%s width=%d line=%q", language, width, line)
				}
			}
		}
	}
}

func TestCoreFormLeaveClearsSensitiveInputs(t *testing.T) {
	svc := service.NewCoreService()
	defer svc.Close()
	s := NewState(svc, nil, nil)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if !s.Editing || len(s.fields) != 3 {
		t.Fatal("external connection form missing")
	}
	s = s.Leave()
	if s.Editing || s.fields != nil || s.args != nil {
		t.Fatal("form state survived leaving page")
	}
}

func TestServiceFormAndNetworkActionsRemainInCoreState(t *testing.T) {
	svc := service.NewCoreService()
	defer svc.Close()
	s := NewState(svc, nil, nil)
	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if !s.Editing || s.form != "service" || cmd != nil || len(s.fields) != 0 {
		t.Fatal("service menu should open without text inputs or a background command")
	}
	s = s.Leave()
	s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if s.Editing || !s.Busy || s.task != "system-proxy" || cmd == nil {
		t.Fatal("system proxy shortcut must toggle inline without opening a dialog")
	}
	s.Close()
}
