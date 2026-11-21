package preferences

import (
	"path/filepath"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
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
