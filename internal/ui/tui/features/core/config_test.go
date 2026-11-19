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
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func mihomoPageFixture(t *testing.T) State {
	t.Helper()
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	// Never invoke a host service manager while testing a file-only save.
	t.Setenv("PATH", t.TempDir())
	cfg := config.DefaultConfig
	cfg.Core.Version = "v1.2.3"
	require.NoError(t, config.Save(&cfg))
	svc := service.NewCoreService()
	t.Cleanup(svc.Close)
	require.NoError(t, svc.Configure())
	configSvc := service.NewConfigService()
	s := NewState(svc, configSvc, nil)
	s, _ = s.Update(coreKey("p"))
	s, _ = s.Update(messages.MihomoConfigMsg{Config: configSvc.ReadMihomoConfig(), FromFile: true})
	return s
}

func TestCoreMihomoConfigRetainsEnumSaveAndSingleManagedActions(t *testing.T) {
	i18n.Init()
	s := mihomoPageFixture(t)
	require.True(t, s.IsMihomoPage())
	for range 4 {
		s, _ = s.Update(coreKey("down"))
	}
	s, _ = s.Update(coreKey("enter"))
	require.True(t, s.InputCapture().Active)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
	s, cmd := s.Update(coreKey("enter"))
	require.NotNil(t, cmd)
	result := cmd().(tea.BatchMsg)[0]().(messages.MihomoConfigSavedMsg)
	require.True(t, result.WriteOK)
	require.NoError(t, result.Err)
	path, err := config.GetMihomoConfigPath()
	require.NoError(t, err)
	data, err := config.ReadMihomoYAML(path)
	require.NoError(t, err)
	require.Equal(t, "warning", data["log-level"])
	require.False(t, s.InputCapture().Active)
	cfg, err := config.Load()
	require.NoError(t, err)
	advanced := s.selectTab(2)
	advanced.Status.Mode = cfg.Core.Mode
	advanced.connected = true
	var actions []string
	for _, item := range advanced.actions() {
		if id, ok := strings.CutPrefix(item.action, "api:"); ok {
			actions = append(actions, id)
		}
	}
	require.Equal(t, []string{"reload", "flush_dns", "flush_fakeip"}, actions)
	for _, action := range (State{tab: 2}).actions() {
		require.NotEqual(t, "ports", action.action, "ports must use the shared configuration page")
	}
	for _, language := range []string{"en-US", "zh-CN"} {
		i18n.SetLanguageOverride(language)
		for _, width := range []int{80, 120} {
			view := ansi.Strip(s.Render(width, 24))
			require.Contains(t, view, i18n.T("settings.label.log-level"))
			require.NotContains(t, view, i18n.T("settings.action.flush_fakeip"))
			require.NotContains(t, view, "core.desc.")
		}
	}
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
}

func TestCoreMihomoConfigMouseCoordinatesAndLeaveCleanup(t *testing.T) {
	s := mihomoPageFixture(t)
	page := s.layout(120, 26)
	s, _ = s.Update(messages.PageMouseClickMsg{X: 5, Y: page.editorY + 1 + 2, Width: 120, Height: 30})
	s, _ = s.Update(coreKey("enter"))
	require.True(t, s.InputCapture().Active)
	s = s.Leave()
	require.False(t, s.InputCapture().Active)
	s, _ = s.Update(coreKey("enter"))
	state := s.editor.ToPageState(s.editor.Config, nil)
	require.Equal(t, 2, state.SelectedSetting)
	require.Equal(t, "7890", state.EditValue)
}

func TestCoreMihomoConfigCannotMutateDuringDownload(t *testing.T) {
	s := mihomoPageFixture(t)
	s.Busy, s.task = true, "install"
	s, cmd := s.Update(coreKey("enter"))
	require.Nil(t, cmd)
	require.False(t, s.editor.IsEditing())
	s, cmd = s.Update(messages.PageMouseClickMsg{X: 40, Y: 12, Width: 120, Height: 30})
	require.Nil(t, cmd)
	require.True(t, s.Busy)
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(root, "runtime", "config.yaml"))
	require.NoError(t, err)
	require.False(t, strings.Contains(string(data), "allow-lan: true"))
}
