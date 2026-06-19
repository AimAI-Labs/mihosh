package settings

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

func init() {
	i18n.Init()
}

// setupTestConfig 辅助函数用于初始化一个带有临时文件的配置服务环境
func setupTestConfig(t *testing.T, initialCfg *config.Config) (*service.ConfigService, *config.Config) {
	t.Cleanup(func() {
		viper.Reset()
	})
	viper.Reset()

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	if initialCfg == nil {
		initialCfg = &config.Config{
			APIAddress:          "http://127.0.0.1:9090",
			Secret:              "abc",
			TestURL:             "http://example.com",
			Timeout:             5000,
			ProxyAddress:        "http://127.0.0.1:7890",
			Language:            "auto",
			AutoRefreshInterval: 5,
		}
	}

	if err := config.Save(initialCfg); err != nil {
		t.Fatalf("setupTestConfig Save() returned error: %v", err)
	}

	configSvc := service.NewConfigService()
	cfg, err := configSvc.LoadConfig()
	if err != nil {
		t.Fatalf("setupTestConfig LoadConfig() returned error: %v", err)
	}

	return configSvc, cfg
}

func TestIsEditing(t *testing.T) {
	s := State{editMode: false}
	if s.IsEditing() {
		t.Errorf("expected IsEditing to be false")
	}

	s.editMode = true
	if !s.IsEditing() {
		t.Errorf("expected IsEditing to be true")
	}
}

func TestToPageState(t *testing.T) {
	cfg := &config.Config{APIAddress: "http://127.0.0.1:9090"}
	s := State{
		selectedSetting: 2,
		editMode:        true,
		editValue:       "test",
		editCursor:      4,
	}

	pageState := s.ToPageState(cfg)

	if pageState.Config != cfg {
		t.Errorf("expected Config to match")
	}
	if pageState.SelectedSetting != 2 {
		t.Errorf("expected SelectedSetting to be 2, got %d", pageState.SelectedSetting)
	}
	if pageState.EditMode != true {
		t.Errorf("expected EditMode to be true")
	}
	if pageState.EditValue != "test" {
		t.Errorf("expected EditValue to be 'test', got %q", pageState.EditValue)
	}
	if pageState.EditCursor != 4 {
		t.Errorf("expected EditCursor to be 4, got %d", pageState.EditCursor)
	}
	if pageState.Toast == nil {
		t.Errorf("expected Toast manager to be lazy-initialized")
	}
}

func TestUpdate_NonEditMode_UpDown(t *testing.T) {
	configSvc, cfg := setupTestConfig(t, nil)
	s := State{selectedSetting: 1}

	// 测试 Up 键
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	next, _, _, _ := s.Update(upMsg, cfg, configSvc)
	if next.selectedSetting != 0 {
		t.Errorf("expected Up key to change selectedSetting to 0, got %d", next.selectedSetting)
	}

	// 已经在 0，继续按 Up 不应越界
	next, _, _, _ = next.Update(upMsg, cfg, configSvc)
	if next.selectedSetting != 0 {
		t.Errorf("expected selectedSetting to remain 0, got %d", next.selectedSetting)
	}

	// 测试 Down 键
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	next, _, _, _ = next.Update(downMsg, cfg, configSvc)
	if next.selectedSetting != 1 {
		t.Errorf("expected Down key to change selectedSetting to 1, got %d", next.selectedSetting)
	}

	// 移动到最后一项（索引 6）
	for i := 0; i < 10; i++ {
		next, _, _, _ = next.Update(downMsg, cfg, configSvc)
	}
	if next.selectedSetting != len(SettingKeys)-1 {
		t.Errorf("expected selectedSetting to cap at %d, got %d", len(SettingKeys)-1, next.selectedSetting)
	}
}

func TestUpdate_NonEditMode_Enter(t *testing.T) {
	configSvc, cfg := setupTestConfig(t, nil)
	// 选中 "timeout" (index 3, value is 5000)
	s := State{selectedSetting: 3}

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	next, _, _, _ := s.Update(enterMsg, cfg, configSvc)

	if !next.editMode {
		t.Errorf("expected editMode to be true")
	}
	if next.editValue != "5000" {
		t.Errorf("expected editValue to be '5000', got %q", next.editValue)
	}
	if next.editCursor != 4 {
		t.Errorf("expected editCursor to be 4, got %d", next.editCursor)
	}
}

func TestUpdate_LanguageEditMode(t *testing.T) {
	configSvc, cfg := setupTestConfig(t, nil)
	// 语言对应的 index
	langIdx := LanguageSettingIndex()
	if langIdx == -1 {
		t.Fatalf("LanguageSettingIndex not found")
	}

	s := State{
		selectedSetting: langIdx,
		editMode:        true,
		editValue:       "auto",
	}

	// 1. 测试 right 键循环切换：auto -> zh-CN -> en-US -> auto
	rightMsg := tea.KeyMsg{Type: tea.KeyRight}
	next, _, _, _ := s.Update(rightMsg, cfg, configSvc)
	if next.editValue != "zh-CN" {
		t.Errorf("expected next language zh-CN, got %q", next.editValue)
	}

	next, _, _, _ = next.Update(rightMsg, cfg, configSvc)
	if next.editValue != "en-US" {
		t.Errorf("expected next language en-US, got %q", next.editValue)
	}

	// 测试 tab 键循环切换
	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	next, _, _, _ = next.Update(tabMsg, cfg, configSvc)
	if next.editValue != "auto" {
		t.Errorf("expected next language auto, got %q", next.editValue)
	}

	// 2. 测试 left 键循环切换
	leftMsg := tea.KeyMsg{Type: tea.KeyLeft}
	next, _, _, _ = next.Update(leftMsg, cfg, configSvc)
	if next.editValue != "en-US" {
		t.Errorf("expected prev language en-US, got %q", next.editValue)
	}

	// 3. 测试 Escape 键退出编辑且不保存
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	nextEsc, _, _, _ := next.Update(escMsg, cfg, configSvc)
	if nextEsc.editMode {
		t.Errorf("expected editMode to be false after Esc")
	}
	if nextEsc.editValue != "" {
		t.Errorf("expected editValue to be cleared")
	}

	// 4. 测试 Enter 键保存语言
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	nextEnter, newCfg, proxyAddr, _ := next.Update(enterMsg, cfg, configSvc)
	if nextEnter.editMode {
		t.Errorf("expected editMode to be false after Enter")
	}
	if newCfg.Language != "en-US" {
		t.Errorf("expected saved language to be en-US, got %q", newCfg.Language)
	}
	if proxyAddr != newCfg.ProxyAddress {
		t.Errorf("expected returned proxy address to be %q, got %q", newCfg.ProxyAddress, proxyAddr)
	}

	// 5. 测试 Enter 保存非法语言（但键盘切换的值都是合法的，我们可以手动赋个非法值来测试报错分支）
	sInvalid := State{
		selectedSetting: langIdx,
		editMode:        true,
		editValue:       "invalid-lang",
	}
	nextInvalid, _, _, _ := sInvalid.Update(enterMsg, cfg, configSvc)
	// 虽然语言设置键盘模式通常不会有非法值，但以防万一检查它的处理逻辑，保存失败会调用 s.showToast
	if nextInvalid.toastManager == nil || nextInvalid.toastManager.Render(80) == "" {
		t.Errorf("expected toast error message when saving invalid language")
	}
}

func TestUpdate_NormalEditMode_CursorAndInput(t *testing.T) {
	configSvc, cfg := setupTestConfig(t, nil)
	s := State{
		selectedSetting: 0, // api-address
		editMode:        true,
		editValue:       "http://127.0.0.1:9090",
		editCursor:      21,
	}

	// 1. 测试 left 键移动光标
	leftMsg := tea.KeyMsg{Type: tea.KeyLeft}
	next, _, _, _ := s.Update(leftMsg, cfg, configSvc)
	if next.editCursor != 20 {
		t.Errorf("expected editCursor=20, got %d", next.editCursor)
	}

	// 2. 测试 Home 键
	homeMsg := tea.KeyMsg{Type: tea.KeyHome}
	next, _, _, _ = next.Update(homeMsg, cfg, configSvc)
	if next.editCursor != 0 {
		t.Errorf("expected editCursor=0 after Home, got %d", next.editCursor)
	}

	// 已经到最左侧，按 left 应依然为 0
	next, _, _, _ = next.Update(leftMsg, cfg, configSvc)
	if next.editCursor != 0 {
		t.Errorf("expected editCursor to stay at 0, got %d", next.editCursor)
	}

	// 3. 测试 right 键
	rightMsg := tea.KeyMsg{Type: tea.KeyRight}
	next, _, _, _ = next.Update(rightMsg, cfg, configSvc)
	if next.editCursor != 1 {
		t.Errorf("expected editCursor=1 after Right, got %d", next.editCursor)
	}

	// 4. 测试 End 键
	endMsg := tea.KeyMsg{Type: tea.KeyEnd}
	next, _, _, _ = next.Update(endMsg, cfg, configSvc)
	if next.editCursor != len(next.editValue) {
		t.Errorf("expected editCursor at end, got %d", next.editCursor)
	}

	// 已经到最右侧，按 right 不应越界
	next, _, _, _ = next.Update(rightMsg, cfg, configSvc)
	if next.editCursor != len(next.editValue) {
		t.Errorf("expected editCursor not to exceed length, got %d", next.editCursor)
	}

	// 5. 测试输入普通字符
	// 往末尾追加字符 "s"
	sChar := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}
	next, _, _, _ = next.Update(sChar, cfg, configSvc)
	if next.editValue != "http://127.0.0.1:9090s" {
		t.Errorf("expected value to be updated, got %q", next.editValue)
	}
	if next.editCursor != 22 {
		t.Errorf("expected editCursor to increment to 22, got %d", next.editCursor)
	}

	// 在中间插入字符（如第 5 位插入 "x"）
	next.editCursor = 5
	xChar := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}
	next, _, _, _ = next.Update(xChar, cfg, configSvc)
	if next.editValue != "http:x//127.0.0.1:9090s" {
		t.Errorf("expected insertion, got %q", next.editValue)
	}
	if next.editCursor != 6 {
		t.Errorf("expected editCursor to increment to 6, got %d", next.editCursor)
	}

	// 6. 测试 Backspace 键
	// 删除刚才插入的 "x" (此时光标在第6位，删除第5位的字符)
	backspaceMsg := tea.KeyMsg{Type: tea.KeyBackspace}
	next, _, _, _ = next.Update(backspaceMsg, cfg, configSvc)
	if next.editValue != "http://127.0.0.1:9090s" {
		t.Errorf("expected character to be deleted, got %q", next.editValue)
	}
	if next.editCursor != 5 {
		t.Errorf("expected editCursor to decrement to 5, got %d", next.editCursor)
	}

	// 光标在 0 时按 Backspace 不应做任何事
	next.editCursor = 0
	next, _, _, _ = next.Update(backspaceMsg, cfg, configSvc)
	if next.editValue != "http://127.0.0.1:9090s" {
		t.Errorf("expected no deletion, got %q", next.editValue)
	}

	// 7. 测试 Delete 键
	// 删除光标处的字符（光标在0，删除第一个 'h'）
	deleteMsg := tea.KeyMsg{Type: tea.KeyDelete}
	next, _, _, _ = next.Update(deleteMsg, cfg, configSvc)
	if next.editValue != "ttp://127.0.0.1:9090s" {
		t.Errorf("expected first char deleted, got %q", next.editValue)
	}
	if next.editCursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", next.editCursor)
	}

	// 光标在末尾时按 Delete 不应做任何事
	next.editCursor = len(next.editValue)
	next, _, _, _ = next.Update(deleteMsg, cfg, configSvc)
	if next.editValue != "ttp://127.0.0.1:9090s" {
		t.Errorf("expected no deletion when cursor at end, got %q", next.editValue)
	}
}

func TestUpdate_NormalEditMode_Escape(t *testing.T) {
	configSvc, cfg := setupTestConfig(t, nil)
	s := State{
		selectedSetting: 0,
		editMode:        true,
		editValue:       "http://changed.com",
		editCursor:      18,
	}

	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	next, _, _, _ := s.Update(escMsg, cfg, configSvc)

	if next.editMode {
		t.Errorf("expected editMode to be false")
	}
	if next.editValue != "" {
		t.Errorf("expected editValue to be cleared")
	}
	if next.editCursor != 0 {
		t.Errorf("expected editCursor to be reset to 0")
	}
}

func TestUpdate_NormalEditMode_Enter_Success(t *testing.T) {
	configSvc, cfg := setupTestConfig(t, nil)
	s := State{
		selectedSetting: 3, // timeout
		editMode:        true,
		editValue:       "9999",
		editCursor:      4,
	}

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	next, newCfg, proxyAddr, _ := s.Update(enterMsg, cfg, configSvc)

	if next.editMode {
		t.Errorf("expected editMode to be false after save")
	}
	if next.editValue != "" {
		t.Errorf("expected editValue to be cleared")
	}
	if next.editCursor != 0 {
		t.Errorf("expected editCursor to be reset")
	}
	if newCfg.Timeout != 9999 {
		t.Errorf("expected saved timeout to be 9999, got %d", newCfg.Timeout)
	}
	if proxyAddr != newCfg.ProxyAddress {
		t.Errorf("expected proxyAddress to be matching")
	}

	// 验证 Toast 显示成功
	if next.toastManager == nil {
		t.Fatalf("expected toastManager to be initialized")
	}
	toastMsg := next.toastManager.Render(80)
	if len(toastMsg) == 0 {
		t.Errorf("expected toast message to be visible")
	}
}

func TestUpdate_NormalEditMode_Enter_Fail(t *testing.T) {
	configSvc, cfg := setupTestConfig(t, nil)
	s := State{
		selectedSetting: 3, // timeout
		editMode:        true,
		editValue:       "not-a-number",
		editCursor:      12,
	}

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	next, _, _, _ := s.Update(enterMsg, cfg, configSvc)

	// 应仍保留在编辑模式
	if !next.editMode {
		t.Errorf("expected editMode to remain true on save failure")
	}
	if next.editValue != "not-a-number" {
		t.Errorf("expected editValue to be retained")
	}
	if next.editCursor != 12 {
		t.Errorf("expected editCursor to be retained")
	}

	// 验证 Toast 提示失败
	if next.toastManager == nil {
		t.Fatalf("expected toastManager to be initialized")
	}
	toastMsg := next.toastManager.Render(80)
	if len(toastMsg) == 0 {
		t.Errorf("expected toast error message on failure")
	}
}

func TestHandleMouseScroll(t *testing.T) {
	s := State{selectedSetting: 2}

	// 向上滚动
	next := s.HandleMouseScroll(true)
	if next.selectedSetting != 1 {
		t.Errorf("expected selectedSetting to decrease to 1, got %d", next.selectedSetting)
	}

	// 向上滚动至 0，不应再减少
	next = next.HandleMouseScroll(true)
	next = next.HandleMouseScroll(true)
	if next.selectedSetting != 0 {
		t.Errorf("expected selectedSetting to stop at 0, got %d", next.selectedSetting)
	}

	// 向下滚动
	next = next.HandleMouseScroll(false)
	if next.selectedSetting != 1 {
		t.Errorf("expected selectedSetting to increase to 1, got %d", next.selectedSetting)
	}

	// 向下滚动至最后一项，不应越界
	for i := 0; i < 10; i++ {
		next = next.HandleMouseScroll(false)
	}
	if next.selectedSetting != len(SettingKeys)-1 {
		t.Errorf("expected selectedSetting to cap at %d, got %d", len(SettingKeys)-1, next.selectedSetting)
	}
}

func TestLanguageHelpers(t *testing.T) {
	// 1. nextLanguage
	if nextLanguage("auto") != "zh-CN" {
		t.Errorf("expected next auto -> zh-CN")
	}
	if nextLanguage("zh-CN") != "en-US" {
		t.Errorf("expected next zh-CN -> en-US")
	}
	if nextLanguage("en-US") != "auto" {
		t.Errorf("expected next en-US -> auto")
	}
	if nextLanguage("invalid") != "auto" {
		t.Errorf("expected next invalid -> auto")
	}

	// 2. prevLanguage
	if prevLanguage("auto") != "en-US" {
		t.Errorf("expected prev auto -> en-US")
	}
	if prevLanguage("en-US") != "zh-CN" {
		t.Errorf("expected prev en-US -> zh-CN")
	}
	if prevLanguage("zh-CN") != "auto" {
		t.Errorf("expected prev zh-CN -> auto")
	}
	if prevLanguage("invalid") != "auto" {
		t.Errorf("expected prev invalid -> auto")
	}

	// 3. LanguageSettingIndex
	idx := LanguageSettingIndex()
	if idx != 5 {
		t.Errorf("expected LanguageSettingIndex to be 5, got %d", idx)
	}

	// 4. resolveLanguageMouseTarget
	// 根据 state.go 中实现：
	// valueStartX = settingsContainerLeft (2) + settingsRowPaddingLeft (1) + settingsLabelWidth (20) = 23
	// modes: "auto", "zh-CN", "en-US"
	// "auto" (len 4) -> x 范围: [23, 23+6) = [23, 29)
	// 分隔符 (len 1) -> x: 29
	// "zh-CN" (len 5) -> x 范围: [30, 30+7) = [30, 37)
	// 分隔符 (len 1) -> x: 37
	// "en-US" (len 5) -> x 范围: [38, 38+7) = [38, 45)

	lang, ok := resolveLanguageMouseTarget(25)
	if !ok || lang != "auto" {
		t.Errorf("expected auto, got %q (ok=%v)", lang, ok)
	}

	lang, ok = resolveLanguageMouseTarget(32)
	if !ok || lang != "zh-CN" {
		t.Errorf("expected zh-CN, got %q (ok=%v)", lang, ok)
	}

	lang, ok = resolveLanguageMouseTarget(40)
	if !ok || lang != "en-US" {
		t.Errorf("expected en-US, got %q (ok=%v)", lang, ok)
	}

	// 越界情况
	_, ok = resolveLanguageMouseTarget(-1)
	if ok {
		t.Errorf("expected ok=false for negative coordinate")
	}

	_, ok = resolveLanguageMouseTarget(100)
	if ok {
		t.Errorf("expected ok=false for too large coordinate")
	}
}

func TestThemeSettingIndex(t *testing.T) {
	if ThemeSettingIndex() != 7 {
		t.Fatalf("expected 7, got %d", ThemeSettingIndex())
	}
}

func TestThemeTabSwitchNext(t *testing.T) {
	if nextTheme("tokyo-night") != "catppuccin" {
		t.Errorf("expected catppuccin, got %q", nextTheme("tokyo-night"))
	}
	if nextTheme("dracula") != "tokyo-night" {
		t.Errorf("expected tokyo-night (wrap), got %q", nextTheme("dracula"))
	}
}

func TestThemeTabSwitchPrev(t *testing.T) {
	if prevTheme("catppuccin") != "tokyo-night" {
		t.Errorf("expected tokyo-night, got %q", prevTheme("catppuccin"))
	}
	if prevTheme("tokyo-night") != "dracula" {
		t.Errorf("expected dracula (wrap), got %q", prevTheme("tokyo-night"))
	}
}

func TestThemeSwitchHotSwap(t *testing.T) {
	defer theme.SetTheme("tokyo-night")
	configSvc, cfg := setupTestConfig(t, nil)

	s := State{
		editMode:        true,
		selectedSetting: ThemeSettingIndex(),
		editValue:       "catppuccin",
	}

	// Enter 保存并热切换
	next, newCfg, _, _ := s.Update(
		tea.KeyMsg{Type: tea.KeyEnter},
		cfg,
		configSvc,
	)

	if next.editMode {
		t.Error("expected editMode=false after Enter")
	}
	if theme.CurrentName() != "catppuccin" {
		t.Fatalf("expected theme catppuccin, got %q", theme.CurrentName())
	}
	if newCfg.Theme != "catppuccin" {
		t.Fatalf("expected cfg.Theme=catppuccin, got %q", newCfg.Theme)
	}
}
