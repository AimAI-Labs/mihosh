package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestShellProxyDialogInstallsStartupAndShowsRefreshCommand(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("ZDOTDIR", root)
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	svc := service.NewCoreService()
	defer svc.Close()
	s := NewState(svc, nil, nil).selectTab(2)
	s, _ = s.Update(coreKey("w"))
	if !s.Editing || s.choice != 0 || len(s.fields) != 0 {
		t.Fatal("shell proxy dialog did not open with the saved default")
	}
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{24, 40, 80, 120} {
			view := s.Render(width, 16)
			if lipgloss.Height(view) > 16 {
				t.Fatal("shell proxy dialog exceeded terminal height")
			}
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > width || strings.Contains(line, "core.desc.") {
					t.Fatalf("invalid shell proxy dialog line: %q", line)
				}
			}
		}
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
	s, cmd := s.Update(coreKey("enter"))
	if s.Editing || !s.Busy || cmd == nil {
		t.Fatal("confirm did not queue the shell setting save")
	}
	batch := cmd().(tea.BatchMsg)
	result := batch[1]().(messages.CoreOperationMsg)
	s, _ = s.Update(result)
	if result.Err != nil || s.Busy || s.shellPath != filepath.Join(root, ".zshrc") {
		t.Fatal("shell setting save failed", result.Err)
	}
	cfg, err := config.Load()
	if err != nil || !cfg.ShellProxy {
		t.Fatal("TUI did not persist the selected setting", err)
	}
	if _, err := os.Stat(s.shellPath); err != nil {
		t.Fatal(err)
	}
	startup, err := os.ReadFile(s.shellPath)
	if err != nil || !strings.Contains(string(startup), "proxy.sh") {
		t.Fatal("TUI did not install shell startup", err)
	}
	if !strings.Contains(ansi.Strip(s.Render(120, 30)), "source '") {
		t.Fatal("save result does not explain how to apply it to the current shell")
	}
	s, _ = s.Update(coreKey("w"))
	if s.choice != 1 {
		t.Fatal("reopening the dialog lost the saved preference")
	}
}
