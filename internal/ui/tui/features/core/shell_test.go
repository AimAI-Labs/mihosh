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
	if s.Status.ShellProxy {
		t.Fatal("shell proxy initial status should start disabled")
	}
	for _, language := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{24, 40, 80, 120} {
			view := s.Render(width, 20)
			if lipgloss.Height(view) > 20 {
				t.Fatal("advanced page exceeded terminal height")
			}
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > width || strings.Contains(line, "core.desc.") {
					t.Fatalf("invalid advanced page line: %q", line)
				}
			}
		}
	}
	s, cmd := s.Update(coreKey("w"))
	if s.Editing || !s.Busy || s.task != "shell-proxy" || cmd == nil {
		t.Fatal("shell proxy shortcut should toggle directly without opening a modal")
	}
	batch := cmd().(tea.BatchMsg)
	result := batch[1]().(messages.CoreOperationMsg)
	s, _ = s.Update(result)
	if result.Err != nil || s.Busy || !s.Status.ShellProxy || s.shellPath != filepath.Join(root, ".zshrc") {
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
	if err != nil || !strings.Contains(string(startup), " activate zsh)") {
		t.Fatal("TUI did not install shell startup", err)
	}
	if !strings.Contains(ansi.Strip(s.Render(120, 30)), "source '") {
		t.Fatal("save result does not explain how to apply it to the current shell")
	}
	status, err := svc.Status()
	if err != nil || !status.ShellProxy {
		t.Fatal("service status did not expose updated ShellProxy state")
	}
	s, cmd = s.Update(coreKey("w"))
	if s.Editing || !s.Busy || s.task != "shell-proxy" || cmd == nil {
		t.Fatal("toggling shell proxy back off did not queue operation")
	}
	batch = cmd().(tea.BatchMsg)
	result = batch[1]().(messages.CoreOperationMsg)
	s, _ = s.Update(result)
	cfg, err = config.Load()
	if err != nil || cfg.ShellProxy || s.Status.ShellProxy {
		t.Fatal("TUI did not turn shell proxy back off")
	}
}
