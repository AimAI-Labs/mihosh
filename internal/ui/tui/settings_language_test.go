package tui

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

func TestSettingsLanguageSaveAppliesI18nImmediately(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
	})
	viper.Reset()

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cfg := config.DefaultConfig
	cfg.Language = "zh-CN"
	if err := config.Save(&cfg); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	i18n.Init()
	i18n.SetLanguageOverride("zh-CN")

	model := Model{
		currentPage: layout.PageSettings,
		config:      &cfg,
		configSvc:   service.NewConfigService(),
	}

	for i := 0; i < 5; i++ {
		next, _ := model.dispatchKeyToPage(keyMsg("down"))
		model = next.(Model)
	}
	next, _ := model.dispatchKeyToPage(keyMsg("enter"))
	model = next.(Model)
	next, _ = model.dispatchKeyToPage(keyMsg("right"))
	model = next.(Model)
	next, _ = model.dispatchKeyToPage(keyMsg("enter"))
	model = next.(Model)

	if model.config.Language != "en-US" {
		t.Fatalf("expected model language en-US, got %q", model.config.Language)
	}
	if got := i18n.T("menu.nodes"); got != "Nodes" {
		t.Fatalf("expected i18n to switch immediately, got %q", got)
	}
}

func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
