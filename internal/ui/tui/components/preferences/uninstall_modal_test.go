package preferences

import (
	"errors"
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
	"github.com/stretchr/testify/require"
)

func setupTUIUninstallState(t *testing.T) (s State, root, home, exe string) {
	t.Helper()
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("auto") })

	home = t.TempDir()
	root = filepath.Join(t.TempDir(), "mihosh-cfg")
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("ZDOTDIR", "")

	cfg := config.DefaultConfig
	cfg.ShellProxy = true
	require.NoError(t, config.Save(&cfg))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# >>> mihosh proxy >>>\neval mihosh\n# <<< mihosh proxy <<<\n"), 0600))

	exe = filepath.Join(t.TempDir(), "mihosh")
	require.NoError(t, os.WriteFile(exe, []byte("bin"), 0755))

	svc := service.NewUninstallService(nil).SetTargets(exe, home)
	s = NewState(service.NewConfigService(), nil).SetUninstallService(svc)
	s.Config = &cfg
	return s, root, home, exe
}

func TestUninstallModalKeyboardFlowKeepAndDelete(t *testing.T) {
	t.Run("keep config via space and tab+enter", func(t *testing.T) {
		s, root, _, exe := setupTUIUninstallState(t)

		var cmd tea.Cmd
		s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
		require.NotNil(t, cmd)
		require.True(t, s.uninstallModal.Open)
		require.True(t, s.uninstallModal.KeepConfig)
		require.True(t, s.InputCapture().Active)

		planMsg := cmd().(messages.UninstallPlanMsg)
		require.NoError(t, planMsg.Err)
		s = s.ApplyUninstallPlan(planMsg)
		require.False(t, s.uninstallModal.Loading)
		require.NotNil(t, s.uninstallModal.Plan)

		for _, lang := range []string{"en-US", "zh-CN"} {
			i18n.SetLanguageOverride(lang)
			wide := ansi.Strip(RenderSettingsPage(s.ToPageState(s.Config, nil), 100, 30))
			narrow := ansi.Strip(RenderSettingsPage(s.ToPageState(s.Config, nil), 44, 16))
			require.Contains(t, wide, i18n.T("settings.uninstall.modal_title"))
			require.Contains(t, narrow, i18n.T("settings.uninstall.modal_title"))
		}
		i18n.SetLanguageOverride("en-US")

		// Enter on Focus 0 toggles KeepConfig without running uninstall.
		s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
		require.Nil(t, cmd)
		require.False(t, s.uninstallModal.KeepConfig)
		// Space toggles back to true.
		s, cmd = s.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
		require.Nil(t, cmd)
		require.True(t, s.uninstallModal.KeepConfig)

		// Tab -> Confirm button -> Enter executes uninstall keeping config.
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
		require.Equal(t, uninstallFocusConfirm, s.uninstallModal.Focus)
		s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
		require.NotNil(t, cmd)
		require.True(t, s.uninstallModal.Busy)

		done := cmd().(messages.UninstallDoneMsg)
		require.NoError(t, done.Err)
		require.True(t, done.Result.ConfigPreserved)
		require.True(t, done.Result.BinaryRemoved)
		require.NoFileExists(t, exe)
		require.FileExists(t, filepath.Join(root, "config.yaml"))
	})

	t.Run("remove config when toggled off", func(t *testing.T) {
		s, root, _, exe := setupTUIUninstallState(t)

		s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
		s = s.ApplyUninstallPlan(cmd().(messages.UninstallPlanMsg))
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
		require.False(t, s.uninstallModal.KeepConfig)
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
		s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
		done := cmd().(messages.UninstallDoneMsg)
		require.NoError(t, done.Err)
		require.True(t, done.Result.ConfigRemoved)
		require.NoFileExists(t, exe)
		require.NoDirExists(t, root)
	})
}

func TestUninstallModalMouseInteractions(t *testing.T) {
	s, root, _, exe := setupTUIUninstallState(t)
	s, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	s = s.ApplyUninstallPlan(cmd().(messages.UninstallPlanMsg))

	click := func(state State, label string) (State, tea.Cmd) {
		t.Helper()
		view := ansi.Strip(RenderSettingsPage(state.ToPageState(state.Config, nil), 100, 30))
		for y, line := range strings.Split(view, "\n") {
			if x := strings.Index(line, label); x >= 0 {
				return state.Update(messages.PageMouseClickMsg{X: lipgloss.Width(line[:x]), Y: y, Width: 100, Height: 30})
			}
		}
		t.Fatalf("missing control %q in\n%s", label, view)
		return state, nil
	}

	// Click toggle row flips KeepConfig to false.
	s, cmd = click(s, i18n.T("settings.uninstall.toggle_keep_config"))
	require.Nil(t, cmd)
	require.False(t, s.uninstallModal.KeepConfig)

	// Click Cancel closes modal.
	sCancel, _ := click(s, i18n.T("settings.uninstall.btn_cancel"))
	require.False(t, sCancel.uninstallModal.Open)

	// Click outside modal closes it.
	sOutside, _ := s.Update(messages.PageMouseClickMsg{X: 0, Y: 0, Width: 100, Height: 30})
	require.False(t, sOutside.uninstallModal.Open)

	// Click Confirm triggers uninstall.
	s, cmd = click(s, i18n.T("settings.uninstall.btn_confirm"))
	require.NotNil(t, cmd)
	done := cmd().(messages.UninstallDoneMsg)
	require.NoError(t, done.Err)
	require.NoFileExists(t, exe)
	require.NoDirExists(t, root)

	// Failure keeps modal open with error text.
	failed := s.CompleteUninstall(messages.UninstallDoneMsg{Err: errors.New("boom")})
	require.True(t, failed.uninstallModal.Open)
	require.False(t, failed.uninstallModal.Busy)
	require.Equal(t, "boom", failed.uninstallModal.ErrText)
}
