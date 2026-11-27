package preferences

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const (

	// settingsMouseRowsOffset：配置项第一行相对页面内容顶部的 Y 坐标。
	// 布局：marginTop 空行(1) + 标签栏带边框(3 行: 上边框/内容行/下边框) + 配置面板上边框(1) = 5
	settingsMouseRowsOffset      = 5
	settingsContainerLeft        = 0
	settingsRowPaddingLeft       = 1
	settingsTabHorizontalPadding = 2
	settingsTabContentPadding    = 2
	mihomoApplyIndex             = 5
	mihomoApplyRowOffset         = 6 // 5 fields (0..4) + 1 blank line + apply button row
	mihomoApplyButtonRowY        = settingsMouseRowsOffset + mihomoApplyRowOffset
)

var (
	allowLanValues = []string{"", "true", "false"}
	logLevelValues = []string{"", "info", "warning", "error", "debug", "silent"}
)

// State 设置页面完整状态
type State struct {
	selectedSetting int
	editMode        bool
	editValue       string
	editCursor      int

	doubleClickDetector common.DoubleClickDetector[struct{}]

	// Mihomo 内核版本
	mihomoVersion string
	versionLoaded bool

	activeTab         int // 0=Mihosh, 1=Mihomo
	mihomoConfig      *model.MihomoConfig
	savedOverrides    map[string]string
	draftOverrides    map[string]string
	overridesSaving   bool
	overridesReadAt   time.Time
	mihomoLoaded      bool
	mihomoLoadErr     error
	mihomoFromFile    bool // API 不可达时从 YAML 文件降级读取
	IsCoreUpgrading   bool
	IsCoreRestarting  bool
	IsConfigReloading bool
	IsGeoUpdating     bool

	SysStatus SysStatusState
	syncModal SyncModalState

	configSvc *service.ConfigService
	coreSvc   *service.CoreService
	syncSvc   *service.SyncService
	client    *service.Client
	Config    *service.Config
}

// NewState 创建新的设置页面状态
func NewState(configSvc *service.ConfigService, client *service.Client) State {
	return State{
		SysStatus: NewSysStatusState(),
		configSvc: configSvc,
		client:    client,
	}
}

func NewMihomoState(configSvc *service.ConfigService, client *service.Client, coreSvcs ...*service.CoreService) State {
	s := NewState(configSvc, client)
	s.activeTab = 1
	s.savedOverrides = map[string]string{}
	s.draftOverrides = map[string]string{}
	if len(coreSvcs) > 0 {
		s.coreSvc = coreSvcs[0]
	}
	s.reloadConfig()
	return s
}

// SetConfig updates the cached application configuration held by the preferences state.
func (s *State) SetConfig(cfg *service.Config) {
	s.Config = cfg
}

func (s *State) reloadConfig() {
	var (
		cfg *service.Config
		err error
	)
	if s.configSvc != nil {
		cfg, err = s.configSvc.LoadConfig()
	} else {
		cfg, err = service.Load()
	}
	if err == nil && cfg != nil {
		s.Config = cfg
	}
}

func (s State) Refresh() tea.Cmd {
	if s.overridesSaving {
		return FetchMihomoVersion(s.client)
	}
	return tea.Batch(FetchMihomoOverrides(s.configSvc), FetchMihomoVersion(s.client))
}

func (s State) IsSaving() bool {
	return s.overridesSaving
}

func (s State) HasUnsavedOverrides() bool {
	if s.activeTab != 1 {
		return false
	}
	for _, k := range MihomoSettingKeys {
		if s.getSavedOverride(k) != s.getDraftOverride(k) {
			return true
		}
	}
	return false
}

func (s State) CancelEdit() State {
	s.editMode, s.editValue, s.editCursor = false, "", 0
	return s
}

// IsEditing 返回是否处于编辑模式
func (s State) IsEditing() bool {
	return s.editMode
}

// SelectedSettingIndex 返回当前选中的设置项索引
func (s State) SelectedSettingIndex() int {
	return s.selectedSetting
}

// ActiveTab 返回当前选中的标签页索引
func (s State) ActiveTab() int {
	return s.activeTab
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
func FetchMihomoVersion(client *service.Client) tea.Cmd {
	return messages.CoreRequest(client, func(client *service.Client) tea.Msg {
		if client == nil {
			return messages.MihomoVersionMsg{Version: "unknown"}
		}
		info, err := client.GetVersion()
		if err != nil || info == nil {
			return messages.MihomoVersionMsg{Version: "unknown"}
		}
		return messages.MihomoVersionMsg{Version: info.Version}
	})
}

// ApplyMihomoVersion 应用版本信息
func (s State) ApplyMihomoVersion(version string) State {
	s.mihomoVersion = version
	s.versionLoaded = true
	return s
}

// ToPageState 转换为渲染层所需的 PageState
func (s State) ToPageState(cfg *service.Config, sysLogs []model.LogEntry) PageState {
	return PageState{
		Config:          cfg,
		SelectedSetting: s.selectedSetting,
		EditMode:        s.editMode,
		EditValue:       s.editValue,
		EditCursor:      s.editCursor,
		MihomoVersion:   s.mihomoVersion,
		ActiveTab:       s.activeTab,
		MihomoConfig:    s.mihomoConfig,
		MihomoOverrides: cloneOverridesMap(s.draftOverrides),
		MihomoDirty:     s.HasUnsavedOverrides(),
		MihomoSaving:    s.overridesSaving,
		MihomoLoaded:    s.mihomoLoaded,
		MihomoLoadErr:   s.mihomoLoadErr,
		MihomoFromFile:  s.mihomoFromFile,
		SysStatus:       s.SysStatus,
		SysLogs:         sysLogs,
		SyncModal:       s.syncModal,
	}
}

// Update 处理设置页面按键，返回：(新状态, 更新后的s.Config, cmd)
func (s State) Update(msg tea.Msg) (State, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.PageMouseScrollMsg:
		if s.syncModal.Open {
			return s.handleSyncModalScroll(msg.Up), nil
		}
		// actionsPanelBottomY is roughly the height used by Tab bar (3) + Spacer (1) + Config Panel (8) + Spacer (1) + Actions Panel (4) + Desc/Warning (1)
		newState, cmd := s.HandleMouseScroll(msg.Up, msg.Y, msg.Height, 18)
		return newState, cmd
	case messages.PageMouseClickMsg:
		if s.syncModal.Open {
			return s.handleSyncModalClick(msg.X, msg.Y, msg.Width, msg.Height)
		}
		newState, cmd := s.HandleMouseLeft(msg.X, msg.Y, msg.Width, msg.Height)
		return newState, cmd
	case tea.KeyMsg:
		if s.syncModal.Open {
			return s.handleSyncModalKey(msg)
		}
		if s.activeTab == 1 {
			if s.overridesSaving {
				return s, nil
			}
			if msg.String() == "r" && !s.editMode {
				return s, s.Refresh()
			}
			if !s.mihomoLoaded || s.mihomoLoadErr != nil {
				return s, nil
			}
			if msg.String() == "ctrl+s" {
				next, err := s.commitCurrentEditDraft()
				if err != nil {
					return next, saveErrorNotice(err)
				}
				return next.submitMihomoOverrides()
			}
		}
		if s.editMode {
			return s.handleEditMode(msg)
		}

		switch {
		case key.Matches(msg, common.Keys.Up):
			if s.selectedSetting > 0 {
				s.selectedSetting--
			}
		case key.Matches(msg, common.Keys.Down):
			if s.selectedSetting < s.maxSelectionIndex() {
				s.selectedSetting++
			}
		case msg.String() == "b":
			if s.activeTab == 0 {
				return s.OpenSyncModal(), nil
			}
		case key.Matches(msg, common.Keys.Enter):
			if s.activeTab == 1 && s.selectedSetting == mihomoApplyIndex {
				return s.submitMihomoOverrides()
			}
			if s.activeTab == 0 && s.activeKeys()[s.selectedSetting] == "border-style" {
				return s.toggleBorderStyle()
			}
			s.editMode = true
			s.editValue = s.getEditValue(s.Config, s.activeKeys()[s.selectedSetting])
			s.editCursor = len([]rune(s.editValue))
		}
	}

	return s, nil
}

// HandleMouseScroll 鼠标滚轮处理
func (s State) HandleMouseScroll(up bool, pageY, pageHeight, actionsPanelBottomY int) (State, tea.Cmd) {
	// If we are in the Mihomo tab and the mouse is below the actions panel (where viewport is)
	if s.activeTab == 1 && s.SysStatus.Supported && pageY > 0 {
		if pageY > actionsPanelBottomY {
			mouseMsg := tea.MouseMsg{Action: tea.MouseActionPress}
			if up {
				mouseMsg.Button = tea.MouseButtonWheelUp
			} else {
				mouseMsg.Button = tea.MouseButtonWheelDown
			}

			cmd := s.SysStatus.Update(mouseMsg)
			return s, cmd
		}
	}

	// 编辑模式下禁用滚轮，避免误切当前正在编辑的配置项
	if s.editMode {
		return s, nil
	}
	if up {
		if s.selectedSetting > 0 {
			s.selectedSetting--
		}
	} else {
		if s.selectedSetting < s.maxSelectionIndex() {
			s.selectedSetting++
		}
	}
	return s, nil
}

// HandleMouseLeft 处理 settings 页面左键单击/双击
func (s State) HandleMouseLeft(pageX, pageY, pageWidth, pageHeight int) (State, tea.Cmd) {
	if s.activeTab == 1 {
		return s.handleMihomoMouseLeft(pageX, pageY, pageWidth)
	}
	settingIdx := resolveMouseSettingIndex(s, pageY, pageWidth)
	var feedback tea.Cmd

	if s.editMode {
		if s.activeTab == 0 && s.selectedSetting == LanguageSettingIndex() {
			if lang, ok := resolveLanguageMouseTarget(pageX + settingsLabelWidth - settingLabelWidth(pageWidth)); ok {
				if s.configSvc != nil && s.configSvc.SetConfigValue(s.activeKeys()[s.selectedSetting], lang) == nil {
					s.reloadConfig()
					i18n.SetLanguageOverride(lang)
					s.editMode = false
					s.editValue = ""
					s.editCursor = 0
					return s, noticeCmd(i18n.T("settings.toast.save_success_lang"), messages.NoticeSuccess)
				}
				return s, noticeCmd(i18n.T("settings.toast.save_failed"), messages.NoticeError)
			}
		}
		if s.selectedSetting == ThemeSettingIndex() && settingIdx == s.selectedSetting {
			if t, ok := resolveThemeMouseTarget(pageX, pageY, pageWidth); ok {
				if s.configSvc != nil && s.configSvc.SetConfigValue("theme", t) == nil {
					s.reloadConfig()
					theme.SetTheme(t)
					s.editMode = false
					s.editValue = ""
					s.editCursor = 0
					return s, tea.Batch(func() tea.Msg { return messages.ThemeChangedMsg{} }, noticeCmd(i18n.T("settings.toast.save_success_theme"), messages.NoticeSuccess))
				}
				return s, noticeCmd(i18n.T("settings.toast.save_failed"), messages.NoticeError)
			}
		}

		// 编辑模式下点击空白处退出编辑
		if settingIdx < 0 || settingIdx >= len(s.activeKeys()) {
			s.editMode = false
			s.editValue = ""
			s.editCursor = 0
		}
		return s, nil
	}

	if settingIdx < 0 || settingIdx >= len(s.activeKeys()) {
		return s, nil
	}

	s.selectedSetting = settingIdx
	if s.activeKeys()[settingIdx] == "border-style" {
		if choice, ok := resolveEnumMouseTarget(pageX+settingsLabelWidth-settingLabelWidth(pageWidth), borderStyleOptions()); ok {
			style := "rounded"
			if choice == i18n.T("settings.border.square") {
				style = "square"
			}
			return s.saveBorderStyle(style)
		}
		return s, nil
	}
	if s.activeTab == 0 && settingIdx == LanguageSettingIndex() {
		if lang, ok := resolveLanguageMouseTarget(pageX + settingsLabelWidth - settingLabelWidth(pageWidth)); ok {
			if s.configSvc != nil && s.configSvc.SetConfigValue(s.activeKeys()[settingIdx], lang) == nil {
				s.reloadConfig()
				i18n.SetLanguageOverride(lang)
				return s, noticeCmd(i18n.T("settings.toast.save_success_lang"), messages.NoticeSuccess)
			}
			feedback = noticeCmd(i18n.T("settings.toast.save_failed"), messages.NoticeError)
		}
	}
	if s.activeTab == 0 && settingIdx == ThemeSettingIndex() {
		if t, ok := resolveThemeMouseTarget(pageX, pageY, pageWidth); ok {
			if s.configSvc != nil && s.configSvc.SetConfigValue("theme", t) == nil {
				s.reloadConfig()
				theme.SetTheme(t)
				return s, tea.Batch(func() tea.Msg { return messages.ThemeChangedMsg{} }, noticeCmd(i18n.T("settings.toast.save_success_theme"), messages.NoticeSuccess))
			}
			feedback = noticeCmd(i18n.T("settings.toast.save_failed"), messages.NoticeError)
		}
	}

	now := time.Now()
	if s.doubleClickDetector.IsDoubleClick(struct{}{}, settingIdx, now) {
		s.editMode = true
		s.editValue = s.getEditValue(s.Config, s.activeKeys()[settingIdx])
		s.editCursor = len([]rune(s.editValue))
	}

	return s, feedback
}

func (s State) handleMihomoMouseLeft(pageX, pageY, pageWidth int) (State, tea.Cmd) {
	if s.overridesSaving || !s.mihomoLoaded || s.mihomoLoadErr != nil {
		return s, nil
	}
	if pageY == mihomoApplyButtonRowY {
		if s.editMode {
			if resolveMihomoApplyButtonClick(pageX, pageWidth, s.overridesSaving) {
				next, err := s.commitCurrentEditDraft()
				if err != nil {
					return next, saveErrorNotice(err)
				}
				next.selectedSetting = mihomoApplyIndex
				return next.submitMihomoOverrides()
			}
			s = s.CancelEdit()
			s.selectedSetting = mihomoApplyIndex
			return s, nil
		}
		s.selectedSetting = mihomoApplyIndex
		if resolveMihomoApplyButtonClick(pageX, pageWidth, s.overridesSaving) {
			return s.submitMihomoOverrides()
		}
		return s, nil
	}

	settingIdx := resolveMouseSettingIndex(s, pageY, pageWidth)
	if s.editMode {
		if settingIdx == s.selectedSetting && settingIdx >= 0 && settingIdx < len(MihomoSettingKeys) {
			settingKey := MihomoSettingKeys[settingIdx]
			if settingKey == "allow-lan" {
				if val, ok := resolveVisibleEnumMouseTarget(pageX, pageWidth, s.editValue, allowLanValues); ok {
					s.setDraftOverride("allow-lan", val)
					return s.CancelEdit(), nil
				}
			}
			if settingKey == "log-level" {
				if val, ok := resolveVisibleEnumMouseTarget(pageX, pageWidth, s.editValue, logLevelValues); ok {
					s.setDraftOverride("log-level", val)
					return s.CancelEdit(), nil
				}
			}
		}
		if settingIdx < 0 || settingIdx >= len(MihomoSettingKeys) {
			return s.CancelEdit(), nil
		}
		return s, nil
	}

	if settingIdx < 0 || settingIdx >= len(MihomoSettingKeys) {
		return s, nil
	}
	s.selectedSetting = settingIdx
	settingKey := MihomoSettingKeys[settingIdx]
	if settingKey == "allow-lan" {
		if val, ok := resolveVisibleEnumMouseTarget(pageX, pageWidth, s.getDraftOverride("allow-lan"), allowLanValues); ok {
			s.setDraftOverride("allow-lan", val)
			return s, nil
		}
	}
	if settingKey == "log-level" {
		if val, ok := resolveVisibleEnumMouseTarget(pageX, pageWidth, s.getDraftOverride("log-level"), logLevelValues); ok {
			s.setDraftOverride("log-level", val)
			return s, nil
		}
	}
	now := time.Now()
	if s.doubleClickDetector.IsDoubleClick(struct{}{}, settingIdx, now) {
		s.editMode = true
		s.editValue = s.getDraftOverride(settingKey)
		s.editCursor = len([]rune(s.editValue))
	}
	return s, nil
}

func (s State) toggleBorderStyle() (State, tea.Cmd) {
	style := "square"
	if s.Config != nil && s.Config.BorderStyle == style {
		style = "rounded"
	}
	return s.saveBorderStyle(style)
}

func (s State) saveBorderStyle(style string) (State, tea.Cmd) {
	if s.configSvc == nil {
		return s, nil
	}
	if err := s.configSvc.SetConfigValue("border-style", style); err != nil {
		return s, noticeCmd(i18n.T("settings.toast.save_failed"), messages.NoticeError)
	}
	s.reloadConfig()
	theme.SetBorderStyle(style)
	return s, tea.ClearScreen
}

// handleEditMode 处理编辑模式按键
func (s State) handleEditMode(msg tea.KeyMsg) (State, tea.Cmd) {
	if s.activeTab == 1 {
		keys := s.activeKeys()
		settingKey := keys[s.selectedSetting]

		// allow-lan tab toggle (empty / true / false) -> updates draft only
		if settingKey == "allow-lan" {
			switch {
			case key.Matches(msg, common.Keys.Escape):
				return s.CancelEdit(), nil
			case key.Matches(msg, common.Keys.Enter):
				s.setDraftOverride("allow-lan", s.editValue)
				return s.CancelEdit(), nil
			case msg.String() == "left":
				s.editValue = cycleOption(allowLanValues, s.editValue, -1)
			case msg.String() == "right", msg.String() == "tab":
				s.editValue = cycleOption(allowLanValues, s.editValue, 1)
			}
			return s, nil
		}
		// log-level tab toggle (empty / info / warning / error / debug / silent) -> updates draft only
		if settingKey == "log-level" {
			switch {
			case key.Matches(msg, common.Keys.Escape):
				return s.CancelEdit(), nil
			case key.Matches(msg, common.Keys.Enter):
				s.setDraftOverride("log-level", s.editValue)
				return s.CancelEdit(), nil
			case msg.String() == "left":
				s.editValue = cycleOption(logLevelValues, s.editValue, -1)
			case msg.String() == "right", msg.String() == "tab":
				s.editValue = cycleOption(logLevelValues, s.editValue, 1)
			}
			return s, nil
		}
	}

	if s.activeTab == 0 && s.selectedSetting == LanguageSettingIndex() { // 语言设置采用 tab 切换
		switch {
		case key.Matches(msg, common.Keys.Escape):
			s.editMode = false
			s.editValue = ""
		case key.Matches(msg, common.Keys.Enter):
			settingKey := s.activeKeys()[s.selectedSetting]
			if s.configSvc != nil && s.configSvc.SetConfigValue(settingKey, s.editValue) == nil {
				lang := s.editValue
				s.reloadConfig()
				i18n.SetLanguageOverride(lang)
				s.editMode = false
				s.editValue = ""
				return s, noticeCmd(i18n.T("settings.toast.save_success_lang"), messages.NoticeSuccess)
			}
			return s, noticeCmd(i18n.T("settings.toast.save_failed"), messages.NoticeError)
		case msg.String() == "left":
			s.editValue = prevLanguage(s.editValue)
		case msg.String() == "right", msg.String() == "tab":
			s.editValue = nextLanguage(s.editValue)
		}
		return s, nil
	}

	if s.activeTab == 0 && s.selectedSetting == ThemeSettingIndex() { // 主题设置采用 tab 切换，切换后热生效
		switch {
		case key.Matches(msg, common.Keys.Escape):
			s.editMode = false
			s.editValue = ""
		case key.Matches(msg, common.Keys.Enter):
			newTheme := s.editValue
			if s.configSvc == nil || s.configSvc.SetConfigValue("theme", newTheme) != nil {
				return s, noticeCmd(i18n.T("settings.toast.save_failed"), messages.NoticeError)
			}
			s.reloadConfig()
			theme.SetTheme(newTheme)
			s.editMode = false
			s.editValue = ""
			return s, tea.Batch(func() tea.Msg { return messages.ThemeChangedMsg{} }, noticeCmd(i18n.T("settings.toast.save_success_theme"), messages.NoticeSuccess))
		case msg.String() == "left":
			s.editValue = prevTheme(s.editValue)
		case msg.String() == "right", msg.String() == "tab":
			s.editValue = nextTheme(s.editValue)
		}
		return s, nil
	}

	switch {
	case key.Matches(msg, common.Keys.Escape):
		s.editMode = false
		s.editValue = ""
		s.editCursor = 0

	case key.Matches(msg, common.Keys.Enter):
		settingKey := s.activeKeys()[s.selectedSetting]
		if s.activeTab == 1 {
			next, err := s.commitCurrentEditDraft()
			if err != nil {
				return next, saveErrorNotice(err)
			}
			return next, nil
		}

		if s.configSvc == nil {
			return s, noticeCmd(i18n.T("settings.toast.save_failed"), messages.NoticeError)
		}
		if err := s.configSvc.SetConfigValue(settingKey, s.editValue); err != nil {
			// 保存失败：保持编辑模式，显示错误提示
			return s, saveErrorNotice(err)
		}
		s.reloadConfig()
		s.editMode = false
		s.editValue = ""
		s.editCursor = 0
		return s, noticeCmd(i18n.T("settings.toast.save_success"), messages.NoticeSuccess)

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
			var inputRunes []rune
			if msg.Type == tea.KeySpace {
				inputRunes = []rune{' '}
			} else if len(msg.Runes) > 0 {
				inputRunes = msg.Runes
			} else {
				inputRunes = []rune(msg.String())
			}
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

	return s, nil
}

func noticeCmd(text string, level messages.NoticeLevel) tea.Cmd {
	return func() tea.Msg {
		return messages.NoticeMsg{Text: text, Level: level}
	}
}

func saveErrorNotice(err error) tea.Cmd {
	return noticeCmd(i18n.Tf("settings.toast.save_failed_with_err", err.Error()), messages.NoticeError)
}

func resolveMouseSettingIndex(s State, pageY, width int) int {
	settingIdx := pageY - settingsMouseRowsOffset
	if s.activeTab == 0 && settingIdx >= ThemeSettingIndex() {
		_, rows := themeButtonLayout(width)
		if settingIdx < ThemeSettingIndex()+len(rows) {
			return ThemeSettingIndex()
		}
		settingIdx -= len(rows) - 1
	}
	if settingIdx < 0 || settingIdx >= len(s.activeKeys()) {
		return -1
	}
	return settingIdx
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

func (s *State) maxSelectionIndex() int {
	if s.activeTab == 1 {
		return mihomoApplyIndex
	}
	return len(MihoshSettingKeys) - 1
}

func (s State) ApplyMihomoOverrides(msg *messages.MihomoOverridesMsg) State {
	if s.overridesSaving || msg.ReadAt.Before(s.overridesReadAt) {
		return s
	}
	s.overridesReadAt = msg.ReadAt
	s.reloadConfig()
	s.SysStatus.Supported = runtime.GOOS == "linux" && (s.Config == nil || s.Config.Core.Mode != "managed" || s.Config.Core.Runtime == "service")
	s.mihomoLoaded = true
	s.mihomoFromFile = true
	s.mihomoLoadErr = msg.Err
	if msg.Err == nil {
		previousSaved, previousDraft := s.savedOverrides, s.draftOverrides
		s.savedOverrides = msg.Overrides
		s.draftOverrides = cloneOverridesMap(msg.Overrides)
		for _, k := range MihomoSettingKeys {
			if previousDraft[k] != previousSaved[k] {
				s.setDraftOverride(k, previousDraft[k])
			}
		}
	} else {
		s = s.CancelEdit()
	}
	return s
}

func (s State) CompleteMihomoOverridesSave(msg *messages.MihomoOverridesSavedMsg) State {
	s.overridesSaving = false
	if msg.WriteOK {
		s.overridesReadAt = time.Now()
		s.savedOverrides = cloneOverridesMap(msg.Overrides)
		s.draftOverrides = cloneOverridesMap(msg.Overrides)
		s.mihomoLoaded = true
		s.mihomoLoadErr = nil
	}
	return s
}

func (s State) ApplyMihomoConfig(msg *messages.MihomoConfigMsg) State {
	s.reloadConfig()
	s.SysStatus.Supported = runtime.GOOS == "linux" && (s.Config == nil || s.Config.Core.Mode != "managed" || s.Config.Core.Runtime == "service")
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

func resolveThemeMouseTarget(pageX, pageY, width int) (string, bool) {
	labelWidth, rows := themeButtonLayout(width)
	row := pageY - settingsMouseRowsOffset - ThemeSettingIndex()
	if row < 0 || row >= len(rows) {
		return "", false
	}
	return resolveEnumMouseTarget(pageX+settingsLabelWidth-labelWidth, rows[row])
}

func resolveLanguageMouseTarget(pageX int) (string, bool) {
	modes := []string{"auto", "zh-CN", "en-US"}
	return resolveEnumMouseTarget(pageX, modes)
}

func resolveEnumMouseTarget(pageX int, modes []string) (string, bool) {
	if pageX < 0 {
		return "", false
	}

	// 2 是 TokyoPanel 的左侧边框和空格 (`│ `) 的宽度
	valueStartX := settingsContainerLeft + 2 + settingsRowPaddingLeft + settingsLabelWidth
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
	return common.DisplayWidth(label) + settingsTabContentPadding + settingsTabHorizontalPadding
}

func (s State) getEditValue(cfg *service.Config, settingKey string) string {
	if s.activeTab == 1 {
		return s.getDraftOverride(settingKey)
	}
	return GetSettingValue(s.ToPageState(s.Config, nil), settingKey)
}

func (s State) getDraftOverride(key string) string {
	if s.draftOverrides != nil {
		return s.draftOverrides[key]
	}
	return ""
}

func (s State) getSavedOverride(key string) string {
	if s.savedOverrides != nil {
		return s.savedOverrides[key]
	}
	return ""
}

func (s *State) setDraftOverride(key, value string) {
	s.draftOverrides = cloneOverridesMap(s.draftOverrides)
	if key != "secret" {
		value = strings.TrimSpace(value)
	}
	if value == "" {
		delete(s.draftOverrides, key)
		return
	}
	s.draftOverrides[key] = value
}

func (s State) commitCurrentEditDraft() (State, error) {
	if !s.editMode || s.selectedSetting < 0 || s.selectedSetting >= len(MihomoSettingKeys) {
		return s.CancelEdit(), nil
	}
	settingKey := MihomoSettingKeys[s.selectedSetting]
	val := s.editValue
	if settingKey != "secret" {
		val = strings.TrimSpace(val)
	}
	if err := service.ValidateMihomoOverride(settingKey, val); err != nil {
		return s, err
	}
	s.setDraftOverride(settingKey, val)
	return s.CancelEdit(), nil
}

func (s State) submitMihomoOverrides() (State, tea.Cmd) {
	if s.overridesSaving || !s.mihomoLoaded || s.mihomoLoadErr != nil {
		return s, nil
	}
	for _, k := range MihomoSettingKeys {
		if err := service.ValidateMihomoOverride(k, s.getDraftOverride(k)); err != nil {
			return s, saveErrorNotice(err)
		}
	}
	s.overridesSaving = true
	return s, ApplyMihomoOverrides(s.configSvc, s.coreSvc, s.client, s.draftOverrides)
}

func cloneOverridesMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

func cycleOption(options []string, current string, delta int) string {
	for i, opt := range options {
		if opt == current {
			return options[(i+delta+len(options))%len(options)]
		}
	}
	if len(options) > 0 {
		return options[0]
	}
	return ""
}

func overrideOptionLabel(val string) string {
	if val == "" {
		return i18n.T("settings.mihomo.empty")
	}
	return val
}

func overrideOptionLabels(values []string) []string {
	labels := make([]string, len(values))
	for i, v := range values {
		labels[i] = overrideOptionLabel(v)
	}
	return labels
}

func resolveVisibleEnumMouseTarget(pageX, pageWidth int, current string, values []string) (string, bool) {
	labelWidth := settingLabelWidth(pageWidth)
	start := settingsContainerLeft + 2 + settingsRowPaddingLeft + labelWidth
	labels := overrideOptionLabels(values)
	offset, visible := enumTabsWindow(overrideOptionLabel(current), labels, pageWidth)
	if pageX < start || pageX >= start+visible {
		return "", false
	}
	label, ok := resolveEnumMouseTarget(pageX+settingsLabelWidth-labelWidth+offset, labels)
	if !ok {
		return "", false
	}
	for i, candidate := range labels {
		if candidate == label {
			return values[i], true
		}
	}
	return "", false
}

func mihomoApplyButtonLabel(saving bool) string {
	if saving {
		return i18n.T("settings.mihomo.applying")
	}
	return i18n.T("settings.mihomo.apply_button")
}

func resolveMihomoApplyButtonClick(pageX, pageWidth int, saving bool) bool {
	start := settingsContainerLeft + 2 + settingsRowPaddingLeft + settingLabelWidth(pageWidth)
	width := settingsTabDisplayWidth(mihomoApplyButtonLabel(saving))
	return pageX >= start && pageX < start+width
}

func (s State) RunCoreAction(action string) (State, tea.Cmd) {
	if s.client == nil {
		return s, nil
	}
	switch action {
	case "upgrade_auto":
		if !s.IsCoreUpgrading {
			s.IsCoreUpgrading = true
			return s, UpgradeCoreCmd(s.client, "")
		}
	case "upgrade_release":
		if !s.IsCoreUpgrading {
			s.IsCoreUpgrading = true
			return s, UpgradeCoreCmd(s.client, "release")
		}
	case "upgrade_alpha":
		if !s.IsCoreUpgrading {
			s.IsCoreUpgrading = true
			return s, UpgradeCoreCmd(s.client, "alpha")
		}
	case "restart":
		if !s.IsCoreRestarting {
			s.IsCoreRestarting = true
			return s, RestartCoreCmd(s.client)
		}
	case "reload":
		if !s.IsConfigReloading {
			s.IsConfigReloading = true
			return s, ReloadConfigsCmd(s.client)
		}
	case "update_geo":
		if !s.IsGeoUpdating {
			s.IsGeoUpdating = true
			return s, UpdateGeoDataCmd(s.client)
		}
	case "flush_dns":
		return s, FlushDNSCacheCmd(s.client)
	case "flush_fakeip":
		return s, FlushFakeIPCmd(s.client)
	}
	return s, nil
}

// Core Actions
func UpgradeCoreCmd(client *service.Client, channel string) tea.Cmd {
	return coreActionRequest(client, func(client *service.Client) tea.Msg {
		if cfg, err := service.Load(); err == nil && cfg.Core.Mode == "managed" {
			return messages.CoreActionErrorMsg{Action: i18n.T("menu.core"), Err: fmt.Errorf("%s", i18n.T("core.manage_here"))}
		}

		if err := client.UpgradeCore(channel); err != nil {
			return messages.CoreActionErrorMsg{Action: i18n.T("settings.action.upgrade_auto"), Err: err}
		}
		return messages.CoreActionDoneMsg{Action: i18n.T("settings.action.upgrade_auto"), NeedReloadAll: true}
	})
}

func RestartCoreCmd(client *service.Client) tea.Cmd {
	return coreActionRequest(client, func(client *service.Client) tea.Msg {
		if cfg, err := service.Load(); err == nil && cfg.Core.Mode == "managed" {
			return messages.CoreActionErrorMsg{Action: i18n.T("menu.core"), Err: fmt.Errorf("%s", i18n.T("core.manage_here"))}
		}

		if err := client.RestartCore(); err != nil {
			return messages.CoreActionErrorMsg{Action: i18n.T("settings.action.restart"), Err: err}
		}
		return messages.CoreActionDoneMsg{Action: i18n.T("settings.action.restart"), NeedReloadAll: true, DelayMs: 500}
	})
}

func ReloadConfigsCmd(client *service.Client) tea.Cmd {
	return coreActionRequest(client, func(client *service.Client) tea.Msg {
		if err := client.ReloadConfigsAPI(); err != nil {
			return messages.CoreActionErrorMsg{Action: i18n.T("settings.action.reload"), Err: err}
		}
		return messages.CoreActionDoneMsg{Action: i18n.T("settings.action.reload"), NeedReloadAll: true}
	})
}

func UpdateGeoDataCmd(client *service.Client) tea.Cmd {
	return coreActionRequest(client, func(client *service.Client) tea.Msg {
		if err := client.UpdateGeoData(); err != nil {
			return messages.CoreActionErrorMsg{Action: i18n.T("settings.action.update_geo"), Err: err}
		}
		return messages.CoreActionDoneMsg{Action: i18n.T("settings.action.update_geo"), NeedReloadAll: true}
	})
}

func FlushDNSCacheCmd(client *service.Client) tea.Cmd {
	return coreActionRequest(client, func(client *service.Client) tea.Msg {
		if err := client.FlushDNSCache(); err != nil {
			return messages.CoreActionErrorMsg{Action: i18n.T("settings.action.flush_dns"), Err: err}
		}
		return messages.CoreActionDoneMsg{Action: i18n.T("settings.action.flush_dns"), NeedReloadAll: false}
	})
}

func FlushFakeIPCmd(client *service.Client) tea.Cmd {
	return coreActionRequest(client, func(client *service.Client) tea.Msg {
		if err := client.FlushFakeIP(); err != nil {
			return messages.CoreActionErrorMsg{Action: i18n.T("settings.action.flush_fakeip"), Err: err}
		}
		return messages.CoreActionDoneMsg{Action: i18n.T("settings.action.flush_fakeip"), NeedReloadAll: false}
	})
}

func coreActionRequest(client *service.Client, action func(*service.Client) tea.Msg) tea.Cmd {
	if client == nil {
		return nil
	}
	return messages.CoreRequest(client, action)
}

// Clear action states
func (s State) ClearActionStates() State {
	s.IsCoreUpgrading = false
	s.IsCoreRestarting = false
	s.IsConfigReloading = false
	s.IsGeoUpdating = false
	return s
}

func (s State) HandleMsg(msg tea.Msg, width, height int, cfg *service.Config, sysLogs []model.LogEntry) (State, tea.Cmd) {
	cmd := s.SysStatus.Update(msg)
	s = s.SyncSysStatus(width, height, s.Config, sysLogs)
	return s, cmd
}

// SyncSysStatus updates the SysStatus dimensions and content based on current state.
// Should be called on resize, new log, or status change when on the Settings page and Mihomo tab.
func (s State) SyncSysStatus(width, height int, cfg *service.Config, sysLogs []model.LogEntry) State {
	if !s.SysStatus.Supported || s.activeTab != 1 {
		return s
	}

	pageState := s.ToPageState(s.Config, sysLogs)
	remainingHeight := CalculateViewportHeight(pageState, width, height)

	s.SysStatus.SyncDimensions(width, remainingHeight)
	s.SysStatus.UpdateContent(sysLogs)

	return s
}

// InputCapture declares which input belongs to the current page mode.
func (s State) InputCapture() common.InputCapture {
	textInput := s.editMode || (s.syncModal.Open && s.syncModal.Mode != syncModeMenu)
	return common.InputCapture{
		Active:    s.editMode || s.syncModal.Open,
		Help:      textInput,
		Interrupt: textInput && !s.syncModal.Busy,
	}
}
