package tui

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/settings"
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
		currentPage:   layout.PageSettings,
		config:        &cfg,
		configSvc:     service.NewConfigService(),
		settingsState: settings.NewState(service.NewConfigService(), nil),
	}

	for i := 0; i < 2; i++ {
		next, _ := model.dispatchToPage(keyMsg("down"))
		model = next.(Model)
	}
	next, _ := model.dispatchToPage(keyMsg("enter"))
	model = next.(Model)
	next, _ = model.dispatchToPage(keyMsg("right"))
	model = next.(Model)
	next, _ = model.dispatchToPage(keyMsg("enter"))
	model = next.(Model)

	if model.config.Language != "en-US" {
		t.Fatalf("expected model language en-US, got %q", model.config.Language)
	}
	if got := i18n.T("menu.nodes"); got != "Nodes" {
		t.Fatalf("expected i18n to switch immediately, got %q", got)
	}
}

func TestSettingsLanguageMouseClickAppliesI18nImmediately(t *testing.T) {
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
		currentPage:   layout.PageSettings,
		config:        &cfg,
		configSvc:     service.NewConfigService(),
		settingsState: settings.NewState(service.NewConfigService(), nil),
		width:         120,
		height:        30,
	}

	const pageX = 48 // originally 42, shifted by 4 due to label width increase, +2 to be safe
	const pageY = 7
	const rawX = pageX
	const rawY = pageY + layout.TopNavHeight

	msg := messages.PageMouseClickMsg{X: pageX, Y: pageY, Width: model.width, Height: model.height - layout.TopNavHeight}
	next, cmd := model.dispatchToPage(msg)
	if cmd == nil {
		t.Fatalf("expected clear screen cmd after language mouse change")
	}
	model = next.(Model)

	if model.config.Language != "en-US" {
		t.Fatalf("expected model language en-US, got %q", model.config.Language)
	}
	if got := i18n.T("menu.nodes"); got != "Nodes" {
		t.Fatalf("expected i18n to switch immediately, got %q", got)
	}
}

func TestSettingsThemeMouseClickClearsScreen(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
		theme.SetTheme("tokyo-night")
	})
	viper.Reset()

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cfg := config.DefaultConfig
	cfg.Theme = "tokyo-night"
	if err := config.Save(&cfg); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	theme.SetTheme("tokyo-night")

	model := Model{
		currentPage:   layout.PageSettings,
		config:        &cfg,
		configSvc:     service.NewConfigService(),
		settingsState: settings.NewState(service.NewConfigService(), nil),
		width:         120,
		height:        30,
	}

	const pageX = 48 // originally 40, shifted by 4 due to label width increase, +4 to be safe
	const pageY = 9
	const rawX = pageX
	const rawY = pageY + layout.TopNavHeight

	msg := messages.PageMouseClickMsg{X: pageX, Y: pageY, Width: model.width, Height: model.height - layout.TopNavHeight}
	next, cmd := model.dispatchToPage(msg)
	if cmd == nil {
		nextModel := next.(Model)
		gotTheme := "<nil>"
		if nextModel.config != nil {
			gotTheme = nextModel.config.Theme
		}
		t.Fatalf("expected clear screen cmd after theme mouse change, config theme=%q current theme=%q", gotTheme, theme.CurrentName())
	}
	model = next.(Model)

	if model.config.Theme != "catppuccin" {
		t.Fatalf("expected model theme catppuccin, got %q", model.config.Theme)
	}
	if got := theme.CurrentName(); got != "catppuccin" {
		t.Fatalf("expected current theme catppuccin, got %q", got)
	}
}

func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
