package settings

import (
	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

const (
	asciiMinPrintable = 32
	asciiMaxPrintable = 127

	settingsMouseRowsOffset      = 4
	settingsDoubleClickThreshold = 350 * time.Millisecond
	settingsContainerLeft        = 2
	settingsRowPaddingLeft       = 1
)

// State 设置页面完整状态
type State struct {
	selectedSetting int
	editMode        bool
	editValue       string
	editCursor      int

	lastMouseSetting int
	lastMouseAt      time.Time
}

// ToPageState 转换为渲染层所需的 PageState
func (s State) ToPageState(cfg *config.Config) PageState {
	return PageState{
		Config:          cfg,
		SelectedSetting: s.selectedSetting,
		EditMode:        s.editMode,
		EditValue:       s.editValue,
		EditCursor:      s.editCursor,
	}
}

// Update 处理设置页面按键，返回：(新状态, 更新后的cfg, 更新后的proxyAddr, cmd)
// proxyAddr 为空字符串时表示无变化
func (s State) Update(msg tea.KeyMsg, cfg *config.Config, configSvc *service.ConfigService) (State, *config.Config, string, tea.Cmd) {
	if s.editMode {
		return s.handleEditMode(msg, cfg, configSvc)
	}

	switch {
	case key.Matches(msg, common.Keys.Up):
		if s.selectedSetting > 0 {
			s.selectedSetting--
		}
	case key.Matches(msg, common.Keys.Down):
		if s.selectedSetting < len(SettingKeys)-1 {
			s.selectedSetting++
		}
	case key.Matches(msg, common.Keys.Enter):
		s.editMode = true
		s.editValue = GetSettingValue(cfg, s.selectedSetting)
		s.editCursor = len(s.editValue)
	}

	return s, cfg, "", nil
}

// HandleMouseScroll 鼠标滚轮处理
func (s State) HandleMouseScroll(up bool) State {
	if up {
		if s.selectedSetting > 0 {
			s.selectedSetting--
		}
	} else {
		if s.selectedSetting < len(SettingKeys)-1 {
			s.selectedSetting++
		}
	}
	return s
}

// HandleMouseLeft 处理 settings 页面左键单击/双击
func (s State) HandleMouseLeft(pageX, pageY int, cfg *config.Config, configSvc *service.ConfigService) (State, *config.Config, string) {
	settingIdx := resolveMouseSettingIndex(pageY)

	if s.editMode {
		if s.selectedSetting == 5 {
			if lang, ok := resolveLanguageMouseTarget(pageX); ok {
				if err := configSvc.SetConfigValue(SettingKeys[s.selectedSetting], lang); err == nil {
					newCfg, _ := configSvc.LoadConfig()
					s.editMode = false
					s.editValue = ""
					s.editCursor = 0
					return s, newCfg, newCfg.ProxyAddress
				}
				return s, cfg, ""
			}
		}

		// 编辑模式下点击空白处退出编辑
		if settingIdx < 0 || settingIdx >= len(SettingKeys) {
			s.editMode = false
			s.editValue = ""
			s.editCursor = 0
		}
		return s, cfg, ""
	}

	if settingIdx < 0 || settingIdx >= len(SettingKeys) {
		return s, cfg, ""
	}

	s.selectedSetting = settingIdx
	if settingIdx == 5 {
		if lang, ok := resolveLanguageMouseTarget(pageX); ok {
			if err := configSvc.SetConfigValue(SettingKeys[settingIdx], lang); err == nil {
				newCfg, _ := configSvc.LoadConfig()
				return s, newCfg, newCfg.ProxyAddress
			}
		}
	}

	now := time.Now()
	if s.isMouseDoubleClick(settingIdx, now) {
		s.editMode = true
		s.editValue = GetSettingValue(cfg, settingIdx)
		s.editCursor = len(s.editValue)
	}

	return s, cfg, ""
}

// handleEditMode 处理编辑模式按键，返回更新后的 cfg 和 proxyAddr（空表示无变化）
func (s State) handleEditMode(msg tea.KeyMsg, cfg *config.Config, configSvc *service.ConfigService) (State, *config.Config, string, tea.Cmd) {
	if s.selectedSetting == 5 { // 语言设置采用 tab 切换
		switch {
		case key.Matches(msg, common.Keys.Escape):
			s.editMode = false
			s.editValue = ""
		case key.Matches(msg, common.Keys.Enter):
			settingKey := SettingKeys[s.selectedSetting]
			if err := configSvc.SetConfigValue(settingKey, s.editValue); err == nil {
				newCfg, _ := configSvc.LoadConfig()
				s.editMode = false
				s.editValue = ""
				return s, newCfg, newCfg.ProxyAddress, nil
			}
		case msg.String() == "left":
			s.editValue = prevLanguage(s.editValue)
		case msg.String() == "right", msg.String() == "tab":
			s.editValue = nextLanguage(s.editValue)
		}
		return s, cfg, "", nil
	}

	switch {
	case key.Matches(msg, common.Keys.Escape):
		s.editMode = false
		s.editValue = ""
		s.editCursor = 0

	case key.Matches(msg, common.Keys.Enter):
		settingKey := SettingKeys[s.selectedSetting]
		if err := configSvc.SetConfigValue(settingKey, s.editValue); err != nil {
			// 保存失败：保持编辑模式，但不更新 cfg
			return s, cfg, "", nil
		}
		newCfg, _ := configSvc.LoadConfig()
		s.editMode = false
		s.editValue = ""
		s.editCursor = 0
		return s, newCfg, newCfg.ProxyAddress, nil

	case msg.String() == "left":
		if s.editCursor > 0 {
			s.editCursor--
		}

	case msg.String() == "right":
		if s.editCursor < len(s.editValue) {
			s.editCursor++
		}

	case key.Matches(msg, common.Keys.Home):
		s.editCursor = 0

	case key.Matches(msg, common.Keys.End):
		s.editCursor = len(s.editValue)

	case key.Matches(msg, common.Keys.Backspace):
		if s.editCursor > 0 {
			s.editValue = s.editValue[:s.editCursor-1] + s.editValue[s.editCursor:]
			s.editCursor--
		}

	case key.Matches(msg, common.Keys.Delete):
		if s.editCursor < len(s.editValue) {
			s.editValue = s.editValue[:s.editCursor] + s.editValue[s.editCursor+1:]
		}

	default:
		input := msg.String()
		if len(input) > 0 && (len(input) > 1 || (input[0] >= asciiMinPrintable && input[0] < asciiMaxPrintable)) {
			s.editValue = s.editValue[:s.editCursor] + input + s.editValue[s.editCursor:]
			s.editCursor += len(input)
		}
	}

	return s, cfg, "", nil
}

func resolveMouseSettingIndex(pageY int) int {
	settingIdx := pageY - settingsMouseRowsOffset
	if settingIdx < 0 || settingIdx >= len(SettingKeys) {
		return -1
	}
	return settingIdx
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

func prevLanguage(lang string) string {
	langs := []string{"auto", "zh-CN", "en-US"}
	for i, l := range langs {
		if l == lang {
			return langs[(i+len(langs)-1)%len(langs)]
		}
	}
	return "auto"
}

func resolveLanguageMouseTarget(pageX int) (string, bool) {
	if pageX < 0 {
		return "", false
	}

	valueStartX := settingsContainerLeft + settingsRowPaddingLeft + settingsLabelWidth
	modes := []string{"auto", "zh-CN", "en-US"}
	cursor := valueStartX

	for i, mode := range modes {
		tabWidth := len(mode) + 2
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
