package settings

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/spf13/viper"
)

func TestHandleMouseLeft_SingleClickSelectsSetting(t *testing.T) {
	state := State{}
	cfg := &config.Config{
		TestURL: "http://example.com",
		Timeout: 5000,
	}
	configSvc := service.NewConfigService()

	state.Config = cfg; state.configSvc = configSvc; next, _ := state.HandleMouseLeft(0, 6, 100, 30)
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

	const timeoutRowY = 6 // timeout index=1, offset=5
	state.Config = cfg; state.configSvc = configSvc; next, _ := state.HandleMouseLeft(0, timeoutRowY, 100, 30)
	if next.editMode {
		t.Fatalf("expected editMode=false on first click")
	}

	next.Config = cfg; next.configSvc = configSvc; next, _ = next.HandleMouseLeft(0, timeoutRowY, 100, 30)
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

	const languageRowY = 7 // language index=2, offset=5
	zhCNTabX := settingsContainerLeft + 2 + settingsRowPaddingLeft + settingsLabelWidth + settingsTabDisplayWidth("auto") + 1
	state.Config = &cfg; state.configSvc = configSvc; next, _ := state.HandleMouseLeft(zhCNTabX, languageRowY, 100, 30)

	if next.selectedSetting != 2 {
		t.Fatalf("expected language row selected, got %d", next.selectedSetting)
	}
	if next.Config == nil {
		t.Fatalf("expected config reload after language click")
	}
	if next.Config.Language != "zh-CN" {
		t.Fatalf("expected language zh-CN, got %q", next.Config.Language)
	}
	if next.editMode {
		t.Fatalf("expected language click to save directly without entering edit mode")
	}
}

func TestResolveThemeMouseTargetIncludesRenderedTabPadding(t *testing.T) {
	valueStartX := settingsContainerLeft + 2 + settingsRowPaddingLeft + settingsLabelWidth

	themeName, ok := resolveThemeMouseTarget(valueStartX + len("tokyo-night") + 3)
	if !ok {
		t.Fatalf("expected click in rendered theme tab padding to resolve")
	}
	if themeName != "tokyo-night" {
		t.Fatalf("expected tokyo-night, got %q", themeName)
	}
}

func TestMihoshSettingKeysAndValues(t *testing.T) {
	cfg := &config.Config{
		AutoRefreshInterval: 7,
	}

	// 验证全局数组没被改错
	if len(MihoshSettingKeys) != 5 {
		t.Fatalf("expected 5 setting keys, got %d", len(MihoshSettingKeys))
	}
	if MihoshSettingKeys[3] != "auto-refresh-interval" {
		t.Fatalf("expected auto-refresh-interval setting key, got %q", MihoshSettingKeys[3])
	}
	if MihoshSettingKeys[4] != "theme" {
		t.Fatalf("expected theme setting key, got %q", MihoshSettingKeys[4])
	}
	state := State{}
	if got := GetSettingValue(state.ToPageState(cfg, nil), "auto-refresh-interval"); got != "7" {
		t.Fatalf("expected auto refresh interval value 7, got %q", got)
	}
}

func TestHandleMouseLeft_ClickOutsideClosesEdit(t *testing.T) {
	state := State{
		selectedSetting: 0,
		editMode:        true,
		editValue:       "http://example.com",
		editCursor:      18,
	}
	cfg := &config.Config{}
	configSvc := service.NewConfigService()

	// 点击无效行 (pageY = 0, yOffset=2，所以 idx = -2)
	state.Config = cfg; state.configSvc = configSvc; next, _ := state.HandleMouseLeft(0, 0, 100, 30)
	if next.editMode {
		t.Fatalf("expected editMode to be false after clicking outside")
	}
	if next.editValue != "" {
		t.Fatalf("expected editValue to be cleared")
	}
}

func TestHandleMouseLeft_ClickInvalidRowDoesNothing(t *testing.T) {
	state := State{
		selectedSetting: 2,
		editMode:        false,
	}
	cfg := &config.Config{}
	configSvc := service.NewConfigService()

	// 键盘模式或非编辑模式下，点击无效的行应该直接返回原状态，不改变选中状态
	state.Config = cfg; state.configSvc = configSvc; next, _ := state.HandleMouseLeft(0, 0, 100, 30)
	if next.selectedSetting != 2 {
		t.Fatalf("expected selectedSetting to remain 2, got %d", next.selectedSetting)
	}
}

func TestHandleMouseLeft_LanguageClickSaveFailure(t *testing.T) {
	state := State{
		selectedSetting: LanguageSettingIndex(),
		editMode:        true,
		editValue:       "auto",
	}
	cfg := &config.Config{Language: "auto"}
	configSvc := service.NewConfigService()

	// 点击非 Tab 区域的 X 坐标 (如 X = 0)，pageY=7 对应语言行 (index=2, offset=5)
	state.Config = cfg; state.configSvc = configSvc; next, _ := state.HandleMouseLeft(0, 7, 100, 30)
	if !next.editMode {
		t.Fatalf("expected editMode to remain true when clicking language row but missing tabs")
	}
	if next.Config != cfg {
		t.Fatalf("expected config to be unchanged")
	}
}

func TestHandleMouseLeft_ClickAllowLanRowToggles(t *testing.T) {
	// allow-lan index=3 in MihomoSettingKeys, offset=5 => rowY=8
	const allowLanRowY = 8
	state := State{
		activeTab:    1,
		mihomoConfig: &model.MihomoConfig{AllowLan: false},
	}
	configSvc := service.NewConfigService()

	trueTabX := settingsContainerLeft + 2 + settingsRowPaddingLeft + settingsLabelWidth + 1
	state.Config = &config.Config{}; state.configSvc = configSvc; next, cmd := state.HandleMouseLeft(trueTabX, allowLanRowY, 100, 30)
	if next.selectedSetting != 3 {
		t.Fatalf("expected allow-lan row selected, got %d", next.selectedSetting)
	}
	if next.editMode {
		t.Fatalf("expected click allow-lan row does not enter edit mode")
	}
	if cmd == nil {
		t.Fatalf("expected save command after clicking allow-lan row")
	}
}

func TestHandleMouseLeft_ClickLogLevelRowSets(t *testing.T) {
	// log-level index=4 in MihomoSettingKeys, offset=5 => rowY=9
	const logLevelRowY = 9
	state := State{
		activeTab:    1,
		mihomoConfig: &model.MihomoConfig{LogLevel: "info"},
	}
	configSvc := service.NewConfigService()

	// "warning" is the second tab.
	// info tab width = 4 + 4 = 8. + 1 (separator) = 9
	warningTabX := settingsContainerLeft + 2 + settingsRowPaddingLeft + settingsLabelWidth + 9 + 1
	state.Config = &config.Config{}; state.configSvc = configSvc; next, cmd := state.HandleMouseLeft(warningTabX, logLevelRowY, 100, 30)
	if next.selectedSetting != 4 {
		t.Fatalf("expected log-level row selected, got %d", next.selectedSetting)
	}
	if next.editMode {
		t.Fatalf("expected click log-level row does not enter edit mode")
	}
	if cmd == nil {
		t.Fatalf("expected save command after clicking log-level row")
	}
}

func TestHandleMouseLeft_EditModeAllowLanClickToggles(t *testing.T) {
	const allowLanRowY = 8
	state := State{
		activeTab:       1,
		selectedSetting: 3,
		editMode:        true,
		editValue:       "false",
		mihomoConfig:    &model.MihomoConfig{AllowLan: false},
	}
	configSvc := service.NewConfigService()

	trueTabX := settingsContainerLeft + 2 + settingsRowPaddingLeft + settingsLabelWidth + 1
	state.Config = &config.Config{}; state.configSvc = configSvc; next, cmd := state.HandleMouseLeft(trueTabX, allowLanRowY, 100, 30)
	if next.editMode {
		t.Fatalf("expected editMode to exit after clicking allow-lan row")
	}
	if cmd == nil {
		t.Fatalf("expected save command after edit-mode allow-lan click")
	}
}
