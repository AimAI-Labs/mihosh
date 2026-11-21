package preferences

import (
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

func TestSettingsSyncModalInteractionExportAndPGConfig(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	cfgDir := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", cfgDir)
	cfg := config.DefaultConfig
	cfg.Theme = "tokyo-night"
	require.NoError(t, config.Save(&cfg))

	s := NewState(service.NewConfigService(), nil)
	s.Config = &cfg

	// Press 'b' opens Backup & Cloud Sync modal and activates input capture.
	var cmd tea.Cmd
	s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	require.Nil(t, cmd)
	require.True(t, s.syncModal.Open)
	require.True(t, s.InputCapture().Active)

	// Render modal cleanly on wide and narrow viewports.
	rendered := RenderSettingsPage(s.ToPageState(s.Config, nil), 100, 30)
	require.Contains(t, rendered, "Backup & Cloud Sync")
	require.Contains(t, rendered, "Export Local Archive")

	// Select item 0 (Export Local Archive) -> enters export path input with default path.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, syncModeExportPath, s.syncModal.Mode)
	require.NotEmpty(t, s.syncModal.InputValue)

	// Override input path and trigger export command.
	outPath := filepath.Join(t.TempDir(), "tui-backup.tar.gz")
	s.syncModal.InputValue = outPath
	s, cmd = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	require.True(t, s.syncModal.Busy)

	msg := cmd().(messages.SyncDoneMsg)
	require.NoError(t, msg.Err)
	require.Equal(t, "export", msg.Action)
	s = s.CompleteSync(msg)
	require.False(t, s.syncModal.Busy)
	require.Equal(t, syncModeMenu, s.syncModal.Mode)
	require.Contains(t, s.syncModal.StatusText, outPath)

	// Navigate to PG Config (index 5), open form, save DSN + profile key, and verify masked display.
	s.syncModal.Cursor = 5
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, syncModePGConfig, s.syncModal.Mode)
	s.syncModal.DSNValue = "postgres://alice:secretpwd@db.example.com:5432/mihosh"
	s.syncModal.KeyValue = "my-laptop"
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, syncModeMenu, s.syncModal.Mode)

	reloaded, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "my-laptop", reloaded.Sync.ProfileKey)
	require.Equal(t, "postgres://alice:secretpwd@db.example.com:5432/mihosh", reloaded.Sync.DSN)

	// Rendered summary must mask PG password!
	renderedAfter := RenderSettingsPage(s.ToPageState(s.Config, nil), 100, 30)
	require.NotContains(t, renderedAfter, "secretpwd")
	require.Contains(t, renderedAfter, "my-laptop")

	// Esc closes modal.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.False(t, s.syncModal.Open)
	require.False(t, s.InputCapture().Active)
}

func TestSettingsSyncModalMouseControls(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	cfg := config.DefaultConfig
	require.NoError(t, config.Save(&cfg))
	initial := State{Config: &cfg}
	click := func(s State, width, height int, label string) (State, tea.Cmd) {
		t.Helper()
		view := ansi.Strip(RenderSettingsPage(s.ToPageState(s.Config, nil), width, height))
		for y, line := range strings.Split(view, "\n") {
			if x := strings.Index(line, label); x >= 0 {
				return s.Update(messages.PageMouseClickMsg{X: lipgloss.Width(line[:x]), Y: y, Width: width, Height: height})
			}
		}
		t.Fatalf("missing control %q in %s", label, view)
		return s, nil
	}
	s, _ := click(initial.OpenSyncModal(), 100, 24, "Export Local Archive")
	require.Equal(t, syncModeExportPath, s.syncModal.Mode)
	s.syncModal.InputValue = filepath.Join(t.TempDir(), "mouse-backup.zip")
	s, cmd := click(s, 100, 24, "Enter Confirm")
	require.True(t, s.syncModal.Busy)
	require.NoError(t, cmd().(messages.SyncDoneMsg).Err)

	s, _ = click(initial.OpenSyncModal(), 100, 24, "Import Local Archive")
	require.Equal(t, syncModeImportPath, s.syncModal.Mode)
	s, _ = click(s, 100, 24, "Esc Back")
	require.Equal(t, syncModeMenu, s.syncModal.Mode)
	for _, label := range []string{"Push to PostgreSQL", "Pull & Restore", "Check PostgreSQL Sync"} {
		s, cmd = click(initial.OpenSyncModal(), 100, 24, label)
		require.True(t, s.syncModal.Busy)
		require.NotNil(t, cmd)
	}

	s = initial.OpenSyncModal()
	for range syncMenuItemCount - 1 {
		s, _ = s.Update(messages.PageMouseScrollMsg{Width: 44, Height: 8})
	}
	s, _ = click(s, 44, 8, "Configure PostgreSQL")
	require.Equal(t, syncModePGConfig, s.syncModal.Mode)
	s, _ = click(s, 44, 8, "Sync Profile Key")
	require.Equal(t, 1, s.syncModal.PGField)
	s.syncModal.KeyValue = "mouse-device"
	s, _ = click(s, 44, 8, "Enter Save")
	require.Equal(t, syncModeMenu, s.syncModal.Mode)
	require.Equal(t, "mouse-device", s.Config.Sync.ProfileKey)
	s, _ = click(s, 44, 8, "Esc Close")
	require.False(t, s.syncModal.Open)
}
