package settings

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/spf13/viper"
)

func TestHandleMouseLeft_SingleClickSelectsSetting(t *testing.T) {
	state := State{}
	cfg := &config.Config{
		APIAddress:   "http://127.0.0.1:9090",
		Secret:       "abc",
		TestURL:      "http://example.com",
		Timeout:      5000,
		ProxyAddress: "http://127.0.0.1:7890",
	}
	configSvc := service.NewConfigService()

	next, _, _ := state.HandleMouseLeft(0, 5, cfg, configSvc)
	if next.selectedSetting != 1 {
		t.Fatalf("expected selectedSetting=1, got %d", next.selectedSetting)
	}
	if next.editMode {
		t.Fatalf("expected editMode=false on single click")
	}
}

func TestHandleMouseLeft_DoubleClickEntersEditMode(t *testing.T) {
	state := State{}
	cfg := &config.Config{
		Timeout: 7000,
	}
	configSvc := service.NewConfigService()

	const timeoutRowY = 7 // timeout index=3, offset=4
	next, _, _ := state.HandleMouseLeft(0, timeoutRowY, cfg, configSvc)
	if next.editMode {
		t.Fatalf("expected editMode=false on first click")
	}

	next, _, _ = next.HandleMouseLeft(0, timeoutRowY, cfg, configSvc)
	if !next.editMode {
		t.Fatalf("expected editMode=true after double click")
	}
	if next.editValue != "7000" {
		t.Fatalf("expected editValue=7000, got %q", next.editValue)
	}
	if next.editCursor != 4 {
		t.Fatalf("expected editCursor=4, got %d", next.editCursor)
	}
}

func TestHandleMouseLeft_ClickLanguageTabSavesImmediately(t *testing.T) {
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
	cfg.Language = "auto"
	if err := config.Save(&cfg); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	configSvc := service.NewConfigService()
	state := State{}

	const languageRowY = 9
	const zhCNTabX = settingsContainerLeft + settingsRowPaddingLeft + settingsLabelWidth + len("auto") + 2 + 1
	next, newCfg, _ := state.HandleMouseLeft(zhCNTabX, languageRowY, &cfg, configSvc)

	if next.selectedSetting != 5 {
		t.Fatalf("expected language row selected, got %d", next.selectedSetting)
	}
	if newCfg == nil {
		t.Fatalf("expected config reload after language click")
	}
	if newCfg.Language != "zh-CN" {
		t.Fatalf("expected language zh-CN, got %q", newCfg.Language)
	}
	if next.editMode {
		t.Fatalf("expected language click to save directly without entering edit mode")
	}
}

func TestSettingsIncludesAutoRefreshInterval(t *testing.T) {
	cfg := &config.Config{
		AutoRefreshInterval: 7,
	}

	if len(SettingKeys) != 7 {
		t.Fatalf("expected 7 setting keys, got %d", len(SettingKeys))
	}
	if SettingKeys[6] != "auto-refresh-interval" {
		t.Fatalf("expected auto-refresh-interval setting key, got %q", SettingKeys[6])
	}
	if got := GetSettingValue(cfg, 6); got != "7" {
		t.Fatalf("expected auto refresh interval value 7, got %q", got)
	}
}
