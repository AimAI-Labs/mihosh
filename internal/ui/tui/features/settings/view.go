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
	settingsLabelWidth  = 24
	settingsMinRowWidth = 40
	settingsDescWidth   = 30
)

var MihoshSettingKeys = []string{"test-url", "timeout", "language", "auto-refresh-interval", "theme"}
var MihomoSettingKeys = []string{"external-controller", "secret", "mixed-port", "allow-lan", "log-level"}

func GetSettingLabel(key string) string {
	switch key {
	case "external-controller":
		return i18n.T("settings.label.external-controller")
	case "secret":
		return i18n.T("settings.label.secret")
	case "test-url":
		return i18n.T("settings.label.test_url")
	case "timeout":
		return i18n.T("settings.label.timeout")
	case "language":
		return i18n.T("settings.label.language")
	case "auto-refresh-interval":
		return i18n.T("settings.label.auto_refresh_interval")
	case "theme":
		return i18n.T("settings.label.theme")
	case "mixed-port":
		return i18n.T("settings.label.mixed-port")
	case "allow-lan":
		return i18n.T("settings.label.allow-lan")
	case "log-level":
		return i18n.T("settings.label.log-level")
	}
	return ""
}

func GetSettingDesc(key string) string {
	return i18n.T("settings.desc." + strings.ReplaceAll(key, "-", "_"))
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

	ActiveTab      int
	MihomoConfig   *model.MihomoConfig
	MihomoLoaded   bool
	MihomoLoadErr  error
	MihomoFromFile bool // API 不可达时从 YAML 降级读取
}

func (p PageState) activeKeys() []string {
	if p.ActiveTab == 1 {
		return MihomoSettingKeys
	}
	return MihoshSettingKeys
}

// GetSettingValue 获取配置值
func GetSettingValue(state PageState, key string) string {
	if state.ActiveTab == 0 && state.Config != nil {
		switch key {
		case "test-url":
			return state.Config.TestURL
		case "timeout":
			return fmt.Sprintf("%d", state.Config.Timeout)
		case "language":
			if state.Config.Language == "" {
				return "auto"
			}
			return state.Config.Language
		case "auto-refresh-interval":
			return fmt.Sprintf("%d", state.Config.AutoRefreshInterval)
		case "theme":
			if state.Config.Theme == "" {
				return "tokyo-night"
			}
			return state.Config.Theme
		}
	} else if state.ActiveTab == 1 && state.MihomoConfig != nil {
		switch key {
		case "external-controller":
			return state.MihomoConfig.ExternalController
		case "secret":
			return state.MihomoConfig.Secret
		case "mixed-port":
			return fmt.Sprintf("%d", state.MihomoConfig.MixedPort)
		case "allow-lan":
			return fmt.Sprintf("%t", state.MihomoConfig.AllowLan)
		case "log-level":
			return state.MihomoConfig.LogLevel
		}
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

	// Tab Bar：复用 connections 的带圆角边框样式（3 行高度：上边框 / 内容行 / 下边框）
	tabs := []string{i18n.T("settings.tab.mihosh"), i18n.T("settings.tab.mihomo")}
	tabBar := renderSettingsTabBar(tabs, state.ActiveTab, width-4)

	// 渲染设置项列表
	var settingItems []string
	keys := state.activeKeys()
	for i := 0; i < len(keys); i++ {
		item := renderSettingItem(state, i, keys[i], GetSettingLabel(keys[i]), width)
		settingItems = append(settingItems, item)
	}

	// 使用 Tokyo 面板包裹设置列表
	listContent := strings.Join(settingItems, "\n")
	settingsPanel := common.RenderTokyoPanel(i18n.T("settings.panel_title"), listContent, width-4)

	// 处理 Mihomo 状态
	var warningText string
	if state.ActiveTab == 1 {
		if !state.MihomoLoaded {
			listContent = "\n  " + i18n.T("settings.mihomo.loading") + "\n"
			settingsPanel = common.RenderTokyoPanel(i18n.T("settings.panel_title"), listContent, width-4)
		} else if state.MihomoLoadErr != nil && state.MihomoConfig == nil {
			// YAML 也读不到，才展示全屏错误
			listContent = "\n  " + fmt.Sprintf(i18n.T("settings.mihomo.load_error"), state.MihomoLoadErr) + "\n"
			settingsPanel = common.RenderTokyoPanel(i18n.T("settings.panel_title"), listContent, width-4)
		} else if state.MihomoFromFile && state.MihomoLoadErr != nil {
			// API 不可达但 YAML 可读：记录警告，稍后显示在配置框左下方
			warningText = "⚠ " + i18n.T("settings.mihomo.offline_warning")
		}
	}

	// 渲染选中项描述（信息行右侧，配置框右下方提示）
	var descPart string
	keys = state.activeKeys()
	if state.SelectedSetting >= 0 && state.SelectedSetting < len(keys) {
		descPart = lipgloss.NewStyle().
			Foreground(common.TokyoMuted()).
			Italic(true).
			Render("💡 " + GetSettingDesc(keys[state.SelectedSetting]))
	}

	// 渲染版本信息（左下角，状态栏上方）
	mihomoVer := state.MihomoVersion
	if mihomoVer == "" {
		mihomoVer = "..."
	}
	mihoshLink := utils.CreateHyperlink("https://github.com/AimAI-Labs/mihosh", "Mihosh "+model.Version)
	mihomoLink := utils.CreateHyperlink("https://github.com/MetaCubeX/mihomo", "Mihomo "+mihomoVer)
	versionText := fmt.Sprintf("%s | Built: %s | %s", mihoshLink, model.Date, mihomoLink)

	// 描述行：左侧警告（如果有），右侧描述，与配置框同宽
	rowWidth := width - 4
	var warningPart string
	if warningText != "" {
		warningPart = lipgloss.NewStyle().
			Foreground(common.TokyoYellow()).
			Render(warningText)
	}

	gap := rowWidth - lipgloss.Width(warningPart) - lipgloss.Width(descPart)
	if gap < 0 {
		gap = 0
	}
	descRowContent := warningPart + strings.Repeat(" ", gap) + descPart

	descRow := lipgloss.NewStyle().
		MarginTop(1).
		Width(rowWidth).
		Render(descRowContent)

	// 组装主要内容：Tab Bar → 配置框 → 描述行
	mainContent := lipgloss.JoinVertical(
		lipgloss.Left,
		tabBar,
		settingsPanel,
		descRow,
	)

	// 包裹容器边距
	mainContent = containerStyle.Render(mainContent)

	// 填充至页面高度，使右下角帮助提示浮层能正确定位到底部
	mainContent = lipgloss.PlaceVertical(height, lipgloss.Top, mainContent)

	// 将版本信息叠加在左下角（状态栏上方）
	mainContent = overlayVersionAtBottomLeft(mainContent, versionText, width, height)

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

// overlayVersionAtBottomLeft 将版本信息叠加在页面左下角（状态栏上方一行）。
func overlayVersionAtBottomLeft(page, versionText string, width, height int) string {
	if versionText == "" {
		return page
	}

	lines := strings.Split(page, "\n")
	targetLine := height - 1 // 状态栏占最后一行，版本信息位于其上方一行
	if targetLine < 0 || targetLine >= len(lines) {
		return page
	}

	versionLine := lipgloss.NewStyle().
		Foreground(common.TokyoMuted()).
		Render(versionText)
	// 宽度限制：保留原始行其余内容（叠加后可能被版本文字覆盖前缀，这里采用左对齐替换整行更安全）
	maxWidth := width - 4
	if maxWidth < 1 {
		maxWidth = 1
	}
	versionLine = common.TruncateDisplay(versionLine, maxWidth)

	// 用版本行替换目标行（左对齐，右侧保留空白），保证不影响右下角帮助浮层位置
	lines[targetLine] = versionLine + strings.Repeat(" ", max(0, width-lipgloss.Width(versionLine)-1))

	return strings.Join(lines, "\n")
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
		if state.ActiveTab == 0 && state.SelectedSetting == LanguageSettingIndex() {
			// 语言项：Tab/方向键切换 + 保存 + 取消
			return []common.InlineHelpHint{
				{Key: "←→/Tab", Desc: i18n.T("help.settings_edit_lang.switch")},
				{Key: "Enter", Desc: i18n.T("help.settings_edit_lang.confirm")},
				{Key: "Esc", Desc: i18n.T("help.settings_edit_lang.cancel")},
			}
		}
		if state.ActiveTab == 0 && state.SelectedSetting == ThemeSettingIndex() {
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

	// 普通模式：切换标签 + 选择 + 编辑
	return []common.InlineHelpHint{
		{Key: "h/l", Desc: i18n.T("help.settings.hint_tab")},
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
func renderSettingItem(state PageState, index int, key string, label string, width int) string {
	value := GetSettingValue(state, key)

	// 密钥特殊处理
	if state.ActiveTab == 1 && key == "secret" && value != "" {
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
	if key == "language" {
		// 语言选项使用 Tab 组件渲染
		valToRender := value
		if state.EditMode && index == state.SelectedSetting {
			valToRender = state.EditValue
		}
		renderedValue = renderLanguageTabs(valToRender, state.EditMode && index == state.SelectedSetting)
	} else if key == "theme" {
		// 主题选项使用 Tab 组件渲染
		valToRender := value
		if state.EditMode && index == state.SelectedSetting {
			valToRender = state.EditValue
		}
		renderedValue = renderThemeTabs(valToRender, state.EditMode && index == state.SelectedSetting)
	} else if key == "allow-lan" {
		valToRender := value
		if state.EditMode && index == state.SelectedSetting {
			valToRender = state.EditValue
		}
		renderedValue = renderAllowLanTabs(valToRender, state.EditMode && index == state.SelectedSetting)
	} else if key == "log-level" {
		valToRender := value
		if state.EditMode && index == state.SelectedSetting {
			valToRender = state.EditValue
		}
		renderedValue = renderLogLevelTabs(valToRender, state.EditMode && index == state.SelectedSetting)
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

func renderAllowLanTabs(current string, editMode bool) string {
	modes := []string{"true", "false"}
	return renderEnumTabs(current, modes, editMode)
}

func renderLogLevelTabs(current string, editMode bool) string {
	modes := []string{"info", "warning", "error", "debug", "silent"}
	return renderEnumTabs(current, modes, editMode)
}

// settingsTabBarHeight 标签栏（带圆角边框）的渲染高度：上边框 / 内容行 / 下边框。
const settingsTabBarHeight = 3

// settingsTabBarContentY 标签栏内容行（可点击行）相对页面内容顶部的 Y 坐标。
// 布局：marginTop 空行(0) + 上边框(1) → 内容行位于 pageY=2。
const settingsTabBarContentY = 2

// renderSettingsTabBar 渲染带圆角边框的标签栏，样式与 connections 模式切换栏一致。
func renderSettingsTabBar(labels []string, active int, width int) string {
	if width < 24 {
		width = 24
	}
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	activeStyle := lipgloss.NewStyle().
		Background(common.TokyoSelected()).
		Foreground(common.TokyoCyan()).
		Bold(true)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(common.TokyoBlue())
	separatorStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted())

	var parts []string
	for i, label := range labels {
		rendered := " " + label + " "
		if i == active {
			parts = append(parts, activeStyle.Render(rendered))
		} else {
			parts = append(parts, inactiveStyle.Render(rendered))
		}
		if i < len(labels)-1 {
			parts = append(parts, separatorStyle.Render("│"))
		}
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	// 内容已自带标签内 padding（" Mihosh "），直接填充至内边框宽度
	contentWidth := lipgloss.Width(content)
	if contentWidth < innerWidth {
		content += strings.Repeat(" ", innerWidth-contentWidth)
	}

	borderStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue())
	topLine := borderStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")
	middleLine := borderStyle.Render("│") + content + borderStyle.Render("│")
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")

	return topLine + "\n" + middleLine + "\n" + bottomLine
}

// SettingsTabBarDisplayWidth 返回单个标签（含 padding）的显示宽度，供鼠标命中检测使用。
func SettingsTabBarDisplayWidth(label string) int {
	return len(label) + 2 // 两侧各一空格
}

func renderEnumTabs(current string, modes []string, editMode bool) string {
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
		if current == m {
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
