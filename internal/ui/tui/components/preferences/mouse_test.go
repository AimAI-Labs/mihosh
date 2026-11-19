package preferences

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestHandleMouseLeft_SingleClickSelectsSetting(t *testing.T) {
	state := State{}
	cfg := &config.Config{
		TestURL: "http://example.com",
		Timeout: 5000,
	}
	configSvc := service.NewConfigService()

	state.Config = cfg
	state.configSvc = configSvc
	next, _ := state.HandleMouseLeft(0, 6, 100, 30)
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
	state.Config = cfg
	state.configSvc = configSvc
	next, _ := state.HandleMouseLeft(0, timeoutRowY, 100, 30)
	if next.editMode {
		t.Fatalf("expected editMode=false on first click")
	}

	next.Config = cfg
	next.configSvc = configSvc
	next, _ = next.HandleMouseLeft(0, timeoutRowY, 100, 30)
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
	state.Config = &cfg
	state.configSvc = configSvc
	next, _ := state.HandleMouseLeft(zhCNTabX, languageRowY, 100, 30)

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

	themeName, ok := resolveThemeMouseTarget(valueStartX+len("tokyo-night")+3, settingsMouseRowsOffset+ThemeSettingIndex(), 160)
	if !ok {
		t.Fatalf("expected click in rendered theme tab padding to resolve")
	}
	if themeName != "tokyo-night" {
		t.Fatalf("expected tokyo-night, got %q", themeName)
	}
}

func TestThemeButtonsWrapAndMatchRenderedClickTargets(t *testing.T) {
	svc, cfg := setupTestConfig(t, nil)
	t.Cleanup(func() {
		theme.SetTheme("tokyo-night")
		theme.SetBorderStyle("rounded")
	})
	for _, width := range []int{38, 80, 160} {
		state := State{Config: cfg, configSvc: svc}
		parts, _, _ := buildSettingsLayoutBase(state.ToPageState(cfg, nil), width)
		lines := strings.Split(ansi.Strip(strings.Join(parts, "\n")), "\n")
		var lastRow int
		for _, name := range theme.Names() {
			found := false
			for y, line := range lines {
				index := strings.Index(line, name)
				if index == -1 {
					continue
				}
				x := common.DisplayWidth(line[:index])
				actual, ok := resolveThemeMouseTarget(x, y, width)
				require.True(t, ok, "%s at width %d", name, width)
				require.Equal(t, name, actual, "width %d at %d,%d in %q", width, x, y, lines)
				lastRow = y
				found = true
				if name == "solarized-light" {
					next, cmd := state.HandleMouseLeft(x, y, width, 30)
					require.NotNil(t, cmd)
					require.Equal(t, name, next.Config.Theme)
					require.Equal(t, ThemeSettingIndex(), next.selectedSetting)
				}
				break
			}
			require.True(t, found, "%s should be fully visible at width %d", name, width)
		}
		require.Equal(t, ThemeSettingIndex()+1, resolveMouseSettingIndex(state, lastRow+1, width))
	}
}

func TestMihoshSettingKeysAndValues(t *testing.T) {
	cfg := &config.Config{
		AutoRefreshInterval: 7,
	}

	// 验证全局数组没被改错
	if len(MihoshSettingKeys) != 6 || MihoshSettingKeys[5] != "border-style" {
		t.Fatalf("expected six setting keys ending in border-style, got %v", MihoshSettingKeys)
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
	state.Config = cfg
	state.configSvc = configSvc
	next, _ := state.HandleMouseLeft(0, 0, 100, 30)
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
	state.Config = cfg
	state.configSvc = configSvc
	next, _ := state.HandleMouseLeft(0, 0, 100, 30)
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
	state.Config = cfg
	state.configSvc = configSvc
	next, _ := state.HandleMouseLeft(0, 7, 100, 30)
	if !next.editMode {
		t.Fatalf("expected editMode to remain true when clicking language row but missing tabs")
	}
	if next.Config != cfg {
		t.Fatalf("expected config to be unchanged")
	}
}

func TestHandleMouseLeft_ClickAllowLanRowToggles(t *testing.T) {
	state := State{activeTab: 1, mihomoLoaded: true}
	const tabStart = settingsContainerLeft + 2 + settingsRowPaddingLeft + settingsLabelWidth
	emptyWidth := settingsTabDisplayWidth(overrideOptionLabel(""))
	for _, tab := range []struct {
		x     int
		value string
	}{
		{tabStart + emptyWidth + 1, "true"},
		{tabStart + emptyWidth + 2 + settingsTabDisplayWidth("true"), "false"},
		{tabStart, ""},
	} {
		next, cmd := state.HandleMouseLeft(tab.x, 8, 100, 30)
		require.Nil(t, cmd)
		require.Equal(t, 3, next.selectedSetting)
		require.False(t, next.editMode)
		require.Equal(t, tab.value, next.getDraftOverride("allow-lan"))
		state = next
	}
	next, cmd := state.HandleMouseLeft(tabStart, mihomoApplyButtonRowY, 100, 30)
	require.NotNil(t, cmd)
	require.True(t, next.IsSaving())
}

func TestHandleMouseLeft_ClickLogLevelRowSets(t *testing.T) {
	state := State{
		activeTab:    1,
		mihomoLoaded: true,
	}
	x := settingsContainerLeft + 2 + settingsRowPaddingLeft + settingsLabelWidth
	for _, value := range logLevelValues {
		next, cmd := state.HandleMouseLeft(x, 9, 120, 30)
		require.Nil(t, cmd)
		require.Equal(t, 4, next.selectedSetting)
		require.False(t, next.editMode)
		require.Equal(t, value, next.getDraftOverride("log-level"))
		state = next
		x += settingsTabDisplayWidth(overrideOptionLabel(value)) + 1
	}
}

func TestHandleMouseLeft_EditModeAllowLanClickToggles(t *testing.T) {
	state := State{
		activeTab:       1,
		selectedSetting: 3,
		editMode:        true,
		editValue:       "false",
		mihomoLoaded:    true,
	}
	trueTabX := settingsContainerLeft + 2 + settingsRowPaddingLeft + settingsLabelWidth + settingsTabDisplayWidth(overrideOptionLabel("")) + 1
	next, cmd := state.HandleMouseLeft(trueTabX, 8, 100, 30)
	require.False(t, next.editMode)
	require.Nil(t, cmd)
	require.Equal(t, "true", next.getDraftOverride("allow-lan"))
}
