package settings

import (
	"fmt"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	asciiMinPrintable = 32
	asciiMaxPrintable = 127

	// settingsMouseRowsOffset：配置项第一行相对页面内容顶部的 Y 坐标。
	// 布局：marginTop 空行(1) + 标签栏带边框(3 行: 上边框/内容行/下边框) + 配置面板上边框(1) = 5
	settingsMouseRowsOffset      = 5
	settingsDoubleClickThreshold = 350 * time.Millisecond
	settingsContainerLeft        = 2
	settingsRowPaddingLeft       = 1
	settingsTabHorizontalPadding = 2
	settingsTabContentPadding    = 2
)

// State 设置页面完整状态
type State struct {
	selectedSetting int
	editMode        bool
	editValue       string
	editCursor      int

	lastMouseSetting int
	lastMouseAt      time.Time

	// Toast 管理器
	toastManager *common.ToastManager

	// Mihomo 内核版本
	mihomoVersion string
	versionLoaded bool

	activeTab      int // 0=Mihosh, 1=Mihomo
	mihomoConfig   *model.MihomoConfig
	mihomoLoaded   bool
	mihomoLoadErr  error
	mihomoFromFile bool // API 不可达时从 YAML 文件降级读取
}

// IsEditing 返回是否处于编辑模式
func (s State) IsEditing() bool {
	return s.editMode
}

// SelectedSettingIndex 返回当前选中的设置项索引
func (s State) SelectedSettingIndex() int {
	return s.selectedSetting
}

// IsLanguageSelected 返回当前是否选中语言设置项
func (s State) IsLanguageSelected() bool {
	return s.activeTab == 0 && s.selectedSetting == LanguageSettingIndex()
}

// IsThemeSelected 返回当前是否选中主题设置项
func (s State) IsThemeSelected() bool {
	return s.activeTab == 0 && s.selectedSetting == ThemeSettingIndex()
}

// FetchMihomoVersion 返回一个拉取 Mihomo 版本信息的 Cmd
func FetchMihomoVersion(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return messages.MihomoVersionMsg{Version: "unknown"}
		}
		info, err := client.GetVersion()
		if err != nil || info == nil {
			return messages.MihomoVersionMsg{Version: "unknown"}
		}
		return messages.MihomoVersionMsg{Version: info.Version}
	}
}

// ApplyMihomoVersion 应用版本信息
func (s State) ApplyMihomoVersion(version string) State {
	s.mihomoVersion = version
	s.versionLoaded = true
	return s
}

// ToPageState 转换为渲染层所需的 PageState
func (s State) ToPageState(cfg *config.Config) PageState {
	if s.toastManager == nil {
		s.toastManager = common.NewToastManager()
	}
	return PageState{
		Config:          cfg,
		SelectedSetting: s.selectedSetting,
		EditMode:        s.editMode,
		EditValue:       s.editValue,
		EditCursor:      s.editCursor,
		Toast:           s.toastManager,
		MihomoVersion:   s.mihomoVersion,
		ActiveTab:       s.activeTab,
		MihomoConfig:    s.mihomoConfig,
		MihomoLoaded:    s.mihomoLoaded,
		MihomoLoadErr:   s.mihomoLoadErr,
		MihomoFromFile:  s.mihomoFromFile,
	}
}

// Update 处理设置页面按键，返回：(新状态, 更新后的cfg, cmd)
func (s State) Update(msg tea.KeyMsg, cfg *config.Config, configSvc *service.ConfigService, client *api.Client) (State, *config.Config, tea.Cmd) {
	if s.editMode {
		return s.handleEditMode(msg, cfg, configSvc, client)
	}

	if msg.String() == "h" || msg.String() == "l" || msg.String() == "tab" {
		s.activeTab = 1 - s.activeTab
		s.selectedSetting = 0
		return s, cfg, nil
	}

	if msg.String() == "r" {
		s.mihomoLoaded = false
		return s, cfg, tea.Batch(FetchMihomoVersion(client), configSvc.FetchMihomoConfig(client))
	}

	switch {
	case key.Matches(msg, common.Keys.Up):
		if s.selectedSetting > 0 {
			s.selectedSetting--
		}
	case key.Matches(msg, common.Keys.Down):
		if s.selectedSetting < len(s.activeKeys())-1 {
			s.selectedSetting++
		}
	case key.Matches(msg, common.Keys.Enter):
		s.editMode = true
		s.editValue = s.getEditValue(cfg, s.activeKeys()[s.selectedSetting])
		s.editCursor = len([]rune(s.editValue))
	}

	return s, cfg, nil
}

// HandleMouseScroll 鼠标滚轮处理
func (s State) HandleMouseScroll(up bool) State {
	// 编辑模式下禁用滚轮，避免误切当前正在编辑的配置项
	if s.editMode {
		return s
	}
	if up {
		if s.selectedSetting > 0 {
			s.selectedSetting--
		}
	} else {
		if s.selectedSetting < len(s.activeKeys())-1 {
			s.selectedSetting++
		}
	}
	return s
}

// HandleMouseLeft 处理 settings 页面左键单击/双击
func (s State) HandleMouseLeft(pageX, pageY int, cfg *config.Config, configSvc *service.ConfigService, client *api.Client) (State, *config.Config, tea.Cmd) {
	// 优先处理标签栏点击（带边框的标签栏内容行位于 settingsTabBarContentY）
	if pageY == settingsTabBarContentY {
		if tab, ok := resolveSettingsTabMouseTarget(pageX); ok {
			// 切换标签页时不主动退出编辑模式，避免误触；但需要重置选中项防止越界
			if s.activeTab != tab {
				s.activeTab = tab
				s.selectedSetting = 0
				s.editMode = false
				s.editValue = ""
				s.editCursor = 0
			}
			return s, cfg, nil
		}
	}

	settingIdx := resolveMouseSettingIndex(s, pageY)

	if s.editMode {
		if s.activeTab == 0 && s.selectedSetting == LanguageSettingIndex() {
			if lang, ok := resolveLanguageMouseTarget(pageX); ok {
				if err := configSvc.SetConfigValue(s.activeKeys()[s.selectedSetting], lang); err == nil {
					newCfg, _ := configSvc.LoadConfig()
					s.editMode = false
					s.editValue = ""
					s.editCursor = 0
					s.showToast(i18n.T("settings.toast.save_success_lang"), common.ToastSuccess)
					return s, newCfg, nil
				}
				s.showToast(i18n.T("settings.toast.save_failed"), common.ToastError)
				return s, cfg, nil
			}
		}
		if s.activeTab == 0 && s.selectedSetting == ThemeSettingIndex() {
			if t, ok := resolveThemeMouseTarget(pageX); ok {
				if err := configSvc.SetConfigValue("theme", t); err == nil {
					newCfg, _ := configSvc.LoadConfig()
					theme.SetTheme(t)
					s.editMode = false
					s.editValue = ""
					s.editCursor = 0
					s.showToast(i18n.T("settings.toast.save_success_theme"), common.ToastSuccess)
					return s, newCfg, nil
				}
				s.showToast(i18n.T("settings.toast.save_failed"), common.ToastError)
				return s, cfg, nil
			}
		}

		if s.activeTab == 1 {
			settingKey := s.activeKeys()[s.selectedSetting]
			if settingKey == "allow-lan" {
				val := !s.mihomoConfig.AllowLan
				s.editMode = false
				s.editValue = ""
				s.editCursor = 0
				return s, cfg, configSvc.SaveMihomoConfigField(client, "allow-lan", val)
			}
			if settingKey == "log-level" {
				val := nextLogLevel(s.mihomoConfig.LogLevel)
				s.editMode = false
				s.editValue = ""
				s.editCursor = 0
				return s, cfg, configSvc.SaveMihomoConfigField(client, "log-level", val)
			}
		}

		// 编辑模式下点击空白处退出编辑
		if settingIdx < 0 || settingIdx >= len(s.activeKeys()) {
			s.editMode = false
			s.editValue = ""
			s.editCursor = 0
		}
		return s, cfg, nil
	}

	if settingIdx < 0 || settingIdx >= len(s.activeKeys()) {
		return s, cfg, nil
	}

	s.selectedSetting = settingIdx
	if s.activeTab == 0 && settingIdx == LanguageSettingIndex() {
		if lang, ok := resolveLanguageMouseTarget(pageX); ok {
			if err := configSvc.SetConfigValue(s.activeKeys()[settingIdx], lang); err == nil {
				newCfg, _ := configSvc.LoadConfig()
				s.showToast(i18n.T("settings.toast.save_success_lang"), common.ToastSuccess)
				return s, newCfg, nil
			}
			s.showToast(i18n.T("settings.toast.save_failed"), common.ToastError)
		}
	}
	if s.activeTab == 0 && settingIdx == ThemeSettingIndex() {
		if t, ok := resolveThemeMouseTarget(pageX); ok {
			if err := configSvc.SetConfigValue("theme", t); err == nil {
				newCfg, _ := configSvc.LoadConfig()
				theme.SetTheme(t)
				s.showToast(i18n.T("settings.toast.save_success_theme"), common.ToastSuccess)
				return s, newCfg, nil
			}
			s.showToast(i18n.T("settings.toast.save_failed"), common.ToastError)
		}
	}

	if s.activeTab == 1 {
		settingKey := s.activeKeys()[settingIdx]
		if settingKey == "allow-lan" {
			val := !s.mihomoConfig.AllowLan
			return s, cfg, configSvc.SaveMihomoConfigField(client, "allow-lan", val)
		}
		if settingKey == "log-level" {
			val := nextLogLevel(s.mihomoConfig.LogLevel)
			return s, cfg, configSvc.SaveMihomoConfigField(client, "log-level", val)
		}
	}

	now := time.Now()
	if s.isMouseDoubleClick(settingIdx, now) {
		s.editMode = true
		s.editValue = s.getEditValue(cfg, s.activeKeys()[settingIdx])
		s.editCursor = len([]rune(s.editValue))
	}

	return s, cfg, nil
}

// handleEditMode 处理编辑模式按键
func (s State) handleEditMode(msg tea.KeyMsg, cfg *config.Config, configSvc *service.ConfigService, client *api.Client) (State, *config.Config, tea.Cmd) {
	if s.activeTab == 1 {
		keys := s.activeKeys()
		settingKey := keys[s.selectedSetting]

		// allow-lan tab toggle
		if settingKey == "allow-lan" {
			switch {
			case key.Matches(msg, common.Keys.Escape):
				s.editMode = false
			case key.Matches(msg, common.Keys.Enter):
				val := s.editValue == "true"
				s.editMode = false
				s.editValue = ""
				return s, cfg, configSvc.SaveMihomoConfigField(client, "allow-lan", val)
			case msg.String() == "left", msg.String() == "right", msg.String() == "tab":
				if s.editValue == "true" {
					s.editValue = "false"
				} else {
					s.editValue = "true"
				}
			}
			return s, cfg, nil
		}
		// log-level tab toggle
		if settingKey == "log-level" {
			levels := []string{"info", "warning", "error", "debug", "silent"}
			switch {
			case key.Matches(msg, common.Keys.Escape):
				s.editMode = false
			case key.Matches(msg, common.Keys.Enter):
				val := s.editValue
				s.editMode = false
				s.editValue = ""
				return s, cfg, configSvc.SaveMihomoConfigField(client, "log-level", val)
			case msg.String() == "left":
				matched := false
				for i, l := range levels {
					if l == s.editValue {
						s.editValue = levels[(i+len(levels)-1)%len(levels)]
						matched = true
						break
					}
				}
				if !matched && len(levels) > 0 {
					s.editValue = levels[0]
				}
			case msg.String() == "right", msg.String() == "tab":
				matched := false
				for i, l := range levels {
					if l == s.editValue {
						s.editValue = levels[(i+1)%len(levels)]
						matched = true
						break
					}
				}
				if !matched && len(levels) > 0 {
					s.editValue = levels[0]
				}
			}
			return s, cfg, nil
		}
	}

	if s.activeTab == 0 && s.selectedSetting == LanguageSettingIndex() { // 语言设置采用 tab 切换
		switch {
		case key.Matches(msg, common.Keys.Escape):
			s.editMode = false
			s.editValue = ""
		case key.Matches(msg, common.Keys.Enter):
			settingKey := s.activeKeys()[s.selectedSetting]
			if err := configSvc.SetConfigValue(settingKey, s.editValue); err == nil {
				newCfg, _ := configSvc.LoadConfig()
				s.editMode = false
				s.editValue = ""
				s.showToast(i18n.T("settings.toast.save_success_lang"), common.ToastSuccess)
				return s, newCfg, nil
			}
			s.showToast(i18n.T("settings.toast.save_failed"), common.ToastError)
		case msg.String() == "left":
			s.editValue = prevLanguage(s.editValue)
		case msg.String() == "right", msg.String() == "tab":
			s.editValue = nextLanguage(s.editValue)
		}
		return s, cfg, nil
	}

	if s.activeTab == 0 && s.selectedSetting == ThemeSettingIndex() { // 主题设置采用 tab 切换，切换后热生效
		switch {
		case key.Matches(msg, common.Keys.Escape):
			s.editMode = false
			s.editValue = ""
		case key.Matches(msg, common.Keys.Enter):
			newTheme := s.editValue
			if err := configSvc.SetConfigValue("theme", newTheme); err != nil {
				s.showToast(i18n.T("settings.toast.save_failed"), common.ToastError)
				return s, cfg, nil
			}
			newCfg, _ := configSvc.LoadConfig()
			theme.SetTheme(newTheme)
			s.editMode = false
			s.editValue = ""
			s.showToast(i18n.T("settings.toast.save_success_theme"), common.ToastSuccess)
			return s, newCfg, func() tea.Msg { return messages.ThemeChangedMsg{} }
		case msg.String() == "left":
			s.editValue = prevTheme(s.editValue)
		case msg.String() == "right", msg.String() == "tab":
			s.editValue = nextTheme(s.editValue)
		}
		return s, cfg, nil
	}

	switch {
	case key.Matches(msg, common.Keys.Escape):
		s.editMode = false
		s.editValue = ""
		s.editCursor = 0

	case key.Matches(msg, common.Keys.Enter):
		settingKey := s.activeKeys()[s.selectedSetting]
		if s.activeTab == 1 {
			var val interface{} = s.editValue
			if settingKey == "mixed-port" {
				var port int
				if _, err := fmt.Sscanf(s.editValue, "%d", &port); err == nil {
					val = port
				} else {
					s.showToast(i18n.T("settings.toast.save_failed"), common.ToastError)
					return s, cfg, nil
				}
			}
			s.editMode = false
			s.editValue = ""
			s.editCursor = 0
			return s, cfg, configSvc.SaveMihomoConfigField(client, settingKey, val)
		}

		if err := configSvc.SetConfigValue(settingKey, s.editValue); err != nil {
			// 保存失败：保持编辑模式，显示错误提示
			s.showToast(i18n.Tf("settings.toast.save_failed_with_err", err.Error()), common.ToastError)
			return s, cfg, nil
		}
		newCfg, _ := configSvc.LoadConfig()
		s.editMode = false
		s.editValue = ""
		s.editCursor = 0
		s.showToast(i18n.T("settings.toast.save_success"), common.ToastSuccess)
		return s, newCfg, nil

	case msg.String() == "left":
		if s.editCursor > 0 {
			s.editCursor--
		}

	case msg.String() == "right":
		if s.editCursor < len([]rune(s.editValue)) {
			s.editCursor++
		}

	case key.Matches(msg, common.Keys.Home):
		s.editCursor = 0

	case key.Matches(msg, common.Keys.End):
		s.editCursor = len([]rune(s.editValue))

	case key.Matches(msg, common.Keys.Backspace):
		runes := []rune(s.editValue)
		if s.editCursor > 0 && s.editCursor <= len(runes) {
			s.editValue = string(append(runes[:s.editCursor-1], runes[s.editCursor:]...))
			s.editCursor--
		}

	case key.Matches(msg, common.Keys.Delete):
		runes := []rune(s.editValue)
		if s.editCursor < len(runes) {
			s.editValue = string(append(runes[:s.editCursor], runes[s.editCursor+1:]...))
		}

	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			inputRunes := []rune(msg.String())
			runes := []rune(s.editValue)
			if s.editCursor > len(runes) {
				s.editCursor = len(runes)
			}
			newRunes := append([]rune(nil), runes[:s.editCursor]...)
			newRunes = append(newRunes, inputRunes...)
			newRunes = append(newRunes, runes[s.editCursor:]...)
			s.editValue = string(newRunes)
			s.editCursor += len(inputRunes)
		}
	}

	return s, cfg, nil
}

// showToast 显示 Toast 提示
func (s *State) showToast(msg string, toastType common.ToastType) {
	if s.toastManager == nil {
		s.toastManager = common.NewToastManager()
	}
	s.toastManager.Add(msg, toastType, 2*time.Second)
}

func resolveMouseSettingIndex(s State, pageY int) int {
	settingIdx := pageY - settingsMouseRowsOffset
	if settingIdx < 0 || settingIdx >= len(s.activeKeys()) {
		return -1
	}
	return settingIdx
}

// resolveSettingsTabMouseTarget 解析标签栏内容行的鼠标点击目标。
// 标签栏带圆角边框：│[tab0]│[tab1]│...，第一个标签从 settingsContainerLeft+1 开始。
// 返回 (tabIndex, true) 表示命中某个标签；否则返回 (0, false)。
func resolveSettingsTabMouseTarget(pageX int) (int, bool) {
	if pageX < 0 {
		return 0, false
	}

	// 标签内容行：左边框在 settingsContainerLeft，内容从 +1 列开始
	cursor := settingsContainerLeft + 1
	labels := []string{i18n.T("settings.tab.mihosh"), i18n.T("settings.tab.mihomo")}
	for i, label := range labels {
		tabWidth := lipgloss.Width(" " + label + " ")
		if pageX >= cursor && pageX < cursor+tabWidth {
			return i, true
		}
		cursor += tabWidth
		if i < len(labels)-1 {
			cursor++ // 分隔符 │ 占 1 列
		}
	}

	return 0, false
}

func (s *State) isMouseDoubleClick(settingIdx int, now time.Time) bool {
	isDoubleClick := s.lastMouseSetting == settingIdx &&
		!s.lastMouseAt.IsZero() &&
		now.Sub(s.lastMouseAt) <= settingsDoubleClickThreshold

	s.lastMouseSetting = settingIdx
	s.lastMouseAt = now

	return isDoubleClick
}

func nextLanguage(lang string) string {
	langs := []string{"auto", "zh-CN", "en-US"}
	for i, l := range langs {
		if l == lang {
			return langs[(i+1)%len(langs)]
		}
	}
	return "auto"
}

func nextLogLevel(level string) string {
	levels := []string{"info", "warning", "error", "debug", "silent"}
	for i, l := range levels {
		if l == level {
			return levels[(i+1)%len(levels)]
		}
	}
	return levels[0]
}

func prevLanguage(lang string) string {
	langs := []string{"auto", "zh-CN", "en-US"}
	for i, l := range langs {
		if l == lang {
			return langs[(i+len(langs)-1)%len(langs)]
		}
	}
	return "auto"
}

func LanguageSettingIndex() int {
	for i, key := range MihoshSettingKeys {
		if key == "language" {
			return i
		}
	}
	return -1
}

// ThemeSettingIndex 返回主题设置项索引
func ThemeSettingIndex() int {
	for i, key := range MihoshSettingKeys {
		if key == "theme" {
			return i
		}
	}
	return -1
}

func (s *State) activeKeys() []string {
	if s.activeTab == 1 {
		return MihomoSettingKeys
	}
	return MihoshSettingKeys
}

func (s State) ApplyMihomoConfig(msg *messages.MihomoConfigMsg) State {
	s.mihomoLoaded = true
	s.mihomoFromFile = msg.FromFile
	if msg.Config != nil {
		// 无论来自 API 还是 YAML 降级，只要有 Config 就展示
		s.mihomoConfig = msg.Config
		if msg.FromFile {
			s.mihomoLoadErr = msg.Err // 保留原始错误用于警告提示
		} else {
			s.mihomoLoadErr = nil
		}
	} else {
		// Config 也为 nil 才是真正的加载失败
		s.mihomoLoadErr = msg.Err
	}
	return s
}

func nextTheme(t string) string {
	themes := theme.Names()
	for i, n := range themes {
		if n == t {
			return themes[(i+1)%len(themes)]
		}
	}
	return themes[0]
}

func prevTheme(t string) string {
	themes := theme.Names()
	for i, n := range themes {
		if n == t {
			return themes[(i+len(themes)-1)%len(themes)]
		}
	}
	return themes[0]
}

// resolveThemeMouseTarget 解析主题 Tab 区域的鼠标点击目标
func resolveThemeMouseTarget(pageX int) (string, bool) {
	if pageX < 0 {
		return "", false
	}

	valueStartX := settingsContainerLeft + settingsRowPaddingLeft + settingsLabelWidth
	themes := theme.Names()
	cursor := valueStartX

	for i, m := range themes {
		tabWidth := settingsTabDisplayWidth(m)
		if pageX >= cursor && pageX < cursor+tabWidth {
			return m, true
		}
		cursor += tabWidth
		if i < len(themes)-1 {
			cursor++
		}
	}

	return "", false
}

func resolveLanguageMouseTarget(pageX int) (string, bool) {
	if pageX < 0 {
		return "", false
	}

	valueStartX := settingsContainerLeft + settingsRowPaddingLeft + settingsLabelWidth
	modes := []string{"auto", "zh-CN", "en-US"}
	cursor := valueStartX

	for i, mode := range modes {
		tabWidth := settingsTabDisplayWidth(mode)
		if pageX >= cursor && pageX < cursor+tabWidth {
			return mode, true
		}
		cursor += tabWidth
		if i < len(modes)-1 {
			cursor++
		}
	}

	return "", false
}

func settingsTabDisplayWidth(label string) int {
	return len(label) + settingsTabContentPadding + settingsTabHorizontalPadding
}

func (s State) getEditValue(cfg *config.Config, settingKey string) string {
	if s.activeTab == 1 && s.mihomoConfig != nil {
		switch settingKey {
		case "external-controller":
			return s.mihomoConfig.ExternalController
		case "secret":
			return s.mihomoConfig.Secret
		case "mixed-port":
			return fmt.Sprintf("%d", s.mihomoConfig.MixedPort)
		case "allow-lan":
			if s.mihomoConfig.AllowLan {
				return "true"
			}
			return "false"
		case "log-level":
			return s.mihomoConfig.LogLevel
		}
		return ""
	}
	return GetSettingValue(s.ToPageState(cfg), settingKey)
}

