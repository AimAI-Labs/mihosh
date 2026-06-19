package settings

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

const (
	settingsLabelWidth  = 20
	settingsMinRowWidth = 40
	settingsDescWidth   = 30
)

var SettingKeys = []string{"api-address", "secret", "test-url", "timeout", "proxy-address", "language", "auto-refresh-interval", "theme"}

func GetSettingLabel(index int) string {
	switch index {
	case 0:
		return i18n.T("settings.label.api_address")
	case 1:
		return i18n.T("settings.label.secret")
	case 2:
		return i18n.T("settings.label.test_url")
	case 3:
		return i18n.T("settings.label.timeout")
	case 4:
		return i18n.T("settings.label.proxy_address")
	case 5:
		return i18n.T("settings.label.language")
	case 6:
		return i18n.T("settings.label.auto_refresh_interval")
	case 7:
		return i18n.T("settings.label.theme")
	}
	return ""
}

func GetSettingDesc(index int) string {
	switch index {
	case 0:
		return i18n.T("settings.desc.api_address")
	case 1:
		return i18n.T("settings.desc.secret")
	case 2:
		return i18n.T("settings.desc.test_url")
	case 3:
		return i18n.T("settings.desc.timeout")
	case 4:
		return i18n.T("settings.desc.proxy_address")
	case 5:
		return i18n.T("settings.desc.language")
	case 6:
		return i18n.T("settings.desc.auto_refresh_interval")
	case 7:
		return i18n.T("settings.desc.theme")
	}
	return ""
}

// PageState 设置页面状态
type PageState struct {
	Config          *config.Config
	SelectedSetting int
	EditMode        bool
	EditValue       string
	EditCursor      int
	// Toast 状态
	Toast *common.ToastManager

	MihomoVersion string
}

// GetSettingValue 获取配置值
func GetSettingValue(cfg *config.Config, index int) string {
	if cfg == nil {
		return ""
	}

	switch index {
	case 0:
		return cfg.APIAddress
	case 1:
		return cfg.Secret
	case 2:
		return cfg.TestURL
	case 3:
		return fmt.Sprintf("%d", cfg.Timeout)
	case 4:
		return cfg.ProxyAddress
	case 5:
		if cfg.Language == "" {
			return "auto"
		}
		return cfg.Language
	case 6:
		return fmt.Sprintf("%d", cfg.AutoRefreshInterval)
	case 7:
		if cfg.Theme == "" {
			return "tokyo-night"
		}
		return cfg.Theme
	}
	return ""
}

// RenderSettingsPage 渲染设置页面
func RenderSettingsPage(state PageState, width, height int) string {
	// Toast 管理器
	if state.Toast == nil {
		state.Toast = common.NewToastManager()
	}
	state.Toast.CleanExpired()

	// 容器统一样式
	containerStyle := lipgloss.NewStyle().
		MarginLeft(2).
		MarginTop(1)

	// 渲染设置项列表
	var settingItems []string
	for i := 0; i < len(SettingKeys); i++ {
		item := renderSettingItem(state, i, GetSettingLabel(i), width)
		settingItems = append(settingItems, item)
	}

	// 使用 Tokyo 面板包裹设置列表
	listContent := strings.Join(settingItems, "\n")
	settingsPanel := common.RenderTokyoPanel(i18n.T("settings.panel_title"), listContent, width-4)

	// 渲染选中项描述（信息行左侧）
	var descPart string
	if state.SelectedSetting >= 0 && state.SelectedSetting < len(SettingKeys) {
		descPart = lipgloss.NewStyle().
			Foreground(common.TokyoMuted()).
			Italic(true).
			Render("💡 " + GetSettingDesc(state.SelectedSetting))
	}

	// 渲染版本信息（信息行右侧，方框右下方）
	mihomoVer := state.MihomoVersion
	if mihomoVer == "" {
		mihomoVer = "..."
	}
	mihoshLink := utils.CreateHyperlink("https://github.com/AimAI-Labs/mihosh", "Mihosh "+model.Version)
	mihomoLink := utils.CreateHyperlink("https://github.com/MetaCubeX/mihomo", "Mihomo "+mihomoVer)
	versionText := fmt.Sprintf("%s | Built: %s | %s", mihoshLink, model.Date, mihomoLink)

	// 信息行：左侧描述、右侧版本信息，整体宽度对齐配置框
	rowWidth := width - 4
	versionSlot := rowWidth - lipgloss.Width(descPart)
	if versionSlot < 0 {
		versionSlot = 0
	}
	// 防止版本信息超长折行破坏布局
	if lipgloss.Width(versionText) > versionSlot {
		versionText = common.TruncateDisplay(versionText, versionSlot)
	}
	versionPart := lipgloss.NewStyle().
		Foreground(common.TokyoMuted()).
		Align(lipgloss.Right).
		Width(versionSlot).
		Render(versionText)
	infoRow := lipgloss.NewStyle().
		MarginTop(1).
		Render(lipgloss.JoinHorizontal(lipgloss.Top, descPart, versionPart))

	// 组装主要内容：配置框 → 信息行（描述左 / 版本右）
	mainContent := lipgloss.JoinVertical(
		lipgloss.Left,
		settingsPanel,
		infoRow,
	)

	// 包裹容器边距
	mainContent = containerStyle.Render(mainContent)

	// 填充至页面高度，使右下角帮助提示浮层能正确定位到底部
	mainContent = lipgloss.PlaceVertical(height, lipgloss.Top, mainContent)

	// 渲染 Toast（如果有）
	toastStr := state.Toast.Render(width)

	// 组装最终结果
	result := mainContent

	// 如果有 Toast，叠加在右上角
	if toastStr != "" {
		result = overlayToast(result, toastStr, width)
	}

	return renderSettingsInlineHelp(result, state, width, height)
}

// ============================================================
//  内联帮助提示面板（右下角浮层）
// ============================================================
//
// 渲染逻辑（InlineHelpHint / FormatInlineHintRow / OverlayHelpAtBottomRight）
// 共享自 components/common。

// buildSettingsInlineHelpHints 根据设置页上下文构建内联帮助条目
func buildSettingsInlineHelpHints(state PageState) []common.InlineHelpHint {
	// 编辑模式：根据是否语言项分流
	if state.EditMode {
		if state.SelectedSetting == LanguageSettingIndex() {
			// 语言项：Tab/方向键切换 + 保存 + 取消
			return []common.InlineHelpHint{
				{Key: "←→/Tab", Desc: i18n.T("help.settings_edit_lang.switch")},
				{Key: "Enter", Desc: i18n.T("help.settings_edit_lang.confirm")},
				{Key: "Esc", Desc: i18n.T("help.settings_edit_lang.cancel")},
			}
		}
		if state.SelectedSetting == ThemeSettingIndex() {
			// 主题项：Tab/方向键切换 + 应用 + 取消
			return []common.InlineHelpHint{
				{Key: "←→/Tab", Desc: i18n.T("help.settings_edit_theme.switch")},
				{Key: "Enter", Desc: i18n.T("help.settings_edit_theme.confirm")},
				{Key: "Esc", Desc: i18n.T("help.settings_edit_theme.cancel")},
			}
		}
		// 普通编辑项：移动光标 + 保存 + 取消
		return []common.InlineHelpHint{
			{Key: "←→", Desc: i18n.T("help.settings_edit_config.move")},
			{Key: "Enter", Desc: i18n.T("help.settings_edit_config.confirm")},
			{Key: "Esc", Desc: i18n.T("help.settings_edit_config.cancel")},
		}
	}

	// 普通模式：选择 + 编辑
	return []common.InlineHelpHint{
		{Key: "↑↓", Desc: i18n.T("help.settings.hint_select")},
		{Key: "Enter", Desc: i18n.T("help.settings.hint_edit")},
	}
}

// renderSettingsInlineHelp 渲染右下角内联帮助面板并叠加到页面上
func renderSettingsInlineHelp(page string, state PageState, width, height int) string {
	hints := buildSettingsInlineHelpHints(state)
	if len(hints) == 0 {
		return page
	}

	body := common.FormatInlineHintRow(hints)
	return common.OverlayHelpAtBottomRight(page, body, width, height)
}

// renderSettingItem 渲染单个设置项
func renderSettingItem(state PageState, index int, label string, width int) string {
	value := GetSettingValue(state.Config, index)

	// 密钥特殊处理
	if index == 1 && value != "" {
		value = utils.MaskSecret(value)
	}

	// 标签样式
	labelStyle := lipgloss.NewStyle().
		Width(settingsLabelWidth).
		Align(lipgloss.Right).
		Foreground(common.TokyoMuted()).
		PaddingRight(1)

	selectedLabelStyle := labelStyle.Copy().
		Foreground(common.TokyoCyan()).
		Bold(true)

	// 值样式
	valueStyle := lipgloss.NewStyle().
		Foreground(common.TokyoForeground())

	// 选中状态样式
	selectedBg := lipgloss.NewStyle().
		Background(common.TokyoSelected())

	// 编辑模式样式
	editBoxStyle := lipgloss.NewStyle().
		Foreground(common.TokyoYellow()).
		Background(common.Background()).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(common.TokyoBlue()).
		Padding(0, 1)

	// 光标样式
	cursorStyle := lipgloss.NewStyle().
		Background(common.TokyoForeground()).
		Foreground(common.Background())

	// 渲染标签
	var renderedLabel string
	if index == state.SelectedSetting {
		renderedLabel = selectedLabelStyle.Render(label + ":")
	} else {
		renderedLabel = labelStyle.Render(label + ":")
	}

	// 渲染值
	var renderedValue string
	if index == LanguageSettingIndex() {
		// 语言选项使用 Tab 组件渲染
		valToRender := value
		if state.EditMode && index == state.SelectedSetting {
			valToRender = state.EditValue
		}
		renderedValue = renderLanguageTabs(valToRender, state.EditMode && index == state.SelectedSetting)
	} else if index == ThemeSettingIndex() {
		// 主题选项使用 Tab 组件渲染
		valToRender := value
		if state.EditMode && index == state.SelectedSetting {
			valToRender = state.EditValue
		}
		renderedValue = renderThemeTabs(valToRender, state.EditMode && index == state.SelectedSetting)
	} else if state.EditMode && index == state.SelectedSetting {
		// 在光标位置渲染真实光标指示符
		cursorPos := state.EditCursor
		if cursorPos < 0 {
			cursorPos = 0
		}
		runes := []rune(state.EditValue)
		if cursorPos > len(runes) {
			cursorPos = len(runes)
		}

		leftPart := string(runes[:cursorPos])
		var cursorChar string
		var rightPart string

		if cursorPos < len(runes) {
			cursorChar = string(runes[cursorPos])
			rightPart = string(runes[cursorPos+1:])
		} else {
			cursorChar = " "
		}

		displayValue := leftPart + cursorStyle.Render(cursorChar) + rightPart
		renderedValue = editBoxStyle.Render(displayValue)
	} else {
		renderedValue = valueStyle.Render(value)
	}

	// 拼装每行的内容
	lineInner := lipgloss.JoinHorizontal(lipgloss.Top, renderedLabel, renderedValue)

	// 定义单行块的样式
	rowWidth := width - 8
	if rowWidth < settingsMinRowWidth {
		rowWidth = settingsMinRowWidth
	}

	rowStyle := lipgloss.NewStyle().Width(rowWidth).PaddingLeft(1)
	if index == state.SelectedSetting {
		rowStyle = rowStyle.Inherit(selectedBg)
	}

	return rowStyle.Render(lineInner)
}

func renderLanguageTabs(currentLang string, editMode bool) string {
	modes := []string{"auto", "zh-CN", "en-US"}
	var parts []string

	activeStyle := lipgloss.NewStyle().
		Background(common.TokyoBlue()).
		Foreground(common.Bright()).
		Bold(true).
		Padding(0, 1)

	if editMode {
		activeStyle = activeStyle.Background(common.TokyoGreen())
	}

	inactiveStyle := lipgloss.NewStyle().
		Foreground(common.TokyoMuted()).
		Background(common.Background()).
		Padding(0, 1)

	separatorStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted())

	for i, m := range modes {
		if currentLang == m {
			parts = append(parts, activeStyle.Render(" "+m+" "))
		} else {
			parts = append(parts, inactiveStyle.Render(" "+m+" "))
		}
		if i < len(modes)-1 {
			parts = append(parts, separatorStyle.Render("│"))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

// renderThemeTabs 渲染主题切换 Tab（仿 renderLanguageTabs）
func renderThemeTabs(currentTheme string, editMode bool) string {
	themes := theme.Names()
	var parts []string

	activeStyle := lipgloss.NewStyle().
		Background(common.TokyoBlue()).
		Foreground(common.Bright()).
		Bold(true).
		Padding(0, 1)

	if editMode {
		activeStyle = activeStyle.Background(common.TokyoGreen())
	}

	inactiveStyle := lipgloss.NewStyle().
		Foreground(common.TokyoMuted()).
		Background(common.Background()).
		Padding(0, 1)

	separatorStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted())

	for i, m := range themes {
		if currentTheme == m {
			parts = append(parts, activeStyle.Render(" "+m+" "))
		} else {
			parts = append(parts, inactiveStyle.Render(" "+m+" "))
		}
		if i < len(themes)-1 {
			parts = append(parts, separatorStyle.Render("│"))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

// overlayToast 在页面右上角叠加 Toast。
func overlayToast(page, toast string, width int) string {
	if toast == "" {
		return page
	}

	pageLines := strings.Split(page, "\n")
	toastLines := strings.Split(toast, "\n")

	toastHeight := len(toastLines)
	if toastHeight > len(pageLines) {
		toastHeight = len(pageLines)
	}

	for i := 0; i < toastHeight; i++ {
		pageLines[i] = toastLines[i]
	}

	return strings.Join(pageLines, "\n")
}
