package preferences

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
	"github.com/charmbracelet/x/ansi"
)

const (
	settingsLabelWidth  = 24
	settingsMinRowWidth = 40
	settingsDescWidth   = 30
)

var MihoshSettingKeys = []string{"test-url", "timeout", "language", "auto-refresh-interval", "theme", "border-style"}
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
	case "border-style":
		return i18n.T("settings.label.border-style")
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

	ActiveTab       int
	MihomoConfig    *model.MihomoConfig
	MihomoOverrides map[string]string
	MihomoDirty     bool
	MihomoSaving    bool
	MihomoLoaded    bool
	MihomoLoadErr   error
	MihomoFromFile  bool // API 不可达时从 YAML 文件降级读取
	SysStatus       SysStatusState
	SysLogs         []model.LogEntry
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
		case "border-style":
			if state.Config.BorderStyle == "square" {
				return "square"
			}
			return "rounded"
		}
	} else if state.ActiveTab == 1 {
		if state.MihomoOverrides != nil {
			return state.MihomoOverrides[key]
		}
		if state.MihomoConfig != nil {
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
	}
	return ""
}

// buildSettingsLayoutBase 抽取基础布局的渲染逻辑，供高度计算和最终渲染复用
func buildSettingsLayoutBase(state PageState, width int) (parts []string, descRow string, versionText string) {
	// Tab Bar
	title := "settings.tab.mihosh"
	if state.ActiveTab == 1 {
		title = "settings.tab.mihomo"
	}
	tabBar := renderSettingsTabBar([]string{i18n.T(title)}, 0, width)

	// 渲染设置项列表
	var settingItems []string
	keys := state.activeKeys()
	for i := 0; i < len(keys); i++ {
		item := renderSettingItem(state, i, keys[i], GetSettingLabel(keys[i]), width)
		settingItems = append(settingItems, item)
	}
	if state.ActiveTab == 1 {
		settingItems = append(settingItems, "", renderMihomoApplyRow(state, width))
	}
	listContent := strings.Join(settingItems, "\n")
	settingsPanel := common.RenderTokyoPanel(i18n.T("settings.panel_title"), listContent, width)

	var warningText string
	if state.ActiveTab == 1 {
		if !state.MihomoLoaded {
			listContent = "\n  " + i18n.T("settings.mihomo.loading") + "\n"
			settingsPanel = common.RenderTokyoPanel(i18n.T("settings.panel_title"), listContent, width)
		} else if state.MihomoLoadErr != nil && state.MihomoConfig == nil {
			listContent = "\n  " + fmt.Sprintf(i18n.T("settings.mihomo.load_error"), state.MihomoLoadErr) + "\n"
			settingsPanel = common.RenderTokyoPanel(i18n.T("settings.panel_title"), listContent, width)
		} else if state.MihomoFromFile && state.MihomoLoadErr != nil {
			warningText = "⚠ " + i18n.T("settings.mihomo.offline_warning")
		}

	}

	// 渲染选中项描述（信息行右侧，配置框右下方提示）已按需求取消
	var descPart string

	// 渲染版本信息（左下角，状态栏上方）
	mihomoVer := state.MihomoVersion
	if mihomoVer == "" {
		mihomoVer = "..."
	}
	mihoshLink := utils.CreateHyperlink("https://github.com/AimAI-Labs/mihosh", "Mihosh "+model.Version)
	mihomoLink := utils.CreateHyperlink("https://github.com/MetaCubeX/mihomo", "Mihomo "+mihomoVer)
	versionText = fmt.Sprintf("%s | Built: %s | %s", mihoshLink, model.Date, mihomoLink)

	// 描述行：左侧警告（如果有），右侧描述，与配置框同宽
	rowWidth := width - 2
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

	descRow = lipgloss.NewStyle().
		MarginTop(1).
		Width(rowWidth).
		Render(descRowContent)

	parts = []string{tabBar, "", settingsPanel}

	return parts, descRow, versionText
}

// CalculateViewportHeight 计算系统状态面板的剩余高度
func CalculateViewportHeight(state PageState, width, height int) int {
	parts, descRow, _ := buildSettingsLayoutBase(state, width)
	usedHeight := lipgloss.Height(lipgloss.JoinVertical(lipgloss.Left, append(parts, descRow)...))
	return height - usedHeight - 7
}

// RenderSettingsPage 渲染设置页面
func RenderSettingsPage(state PageState, width, height int) string {
	return renderSettingsPage(state, width, height, true)
}

func RenderEmbedded(state PageState, width int) string {
	return renderSettingsPage(state, width, 0, false)
}

func renderSettingsPage(state PageState, width, height int, standalone bool) string {
	// Toast 管理器
	if state.Toast == nil {
		state.Toast = common.NewToastManager()
	}
	state.Toast.CleanExpired()

	parts, descRow, versionText := buildSettingsLayoutBase(state, width)
	if !standalone {
		parts = parts[2:] // Core already supplies the section header.
	}

	if standalone && state.ActiveTab == 1 && state.SysStatus.Supported && state.SysStatus.Viewport.Height > 0 {
		statusView := lipgloss.NewStyle().MarginTop(1).Render(common.RenderTokyoPanel(
			i18n.T("settings.sys_status.panel_title"),
			"\n"+state.SysStatus.Viewport.View()+"\n",
			width,
		))
		parts = append(parts, statusView)
	}

	if standalone || strings.TrimSpace(ansi.Strip(descRow)) != "" {
		parts = append(parts, descRow)
	}
	mainContent := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// 填充至页面高度，使右下角帮助提示浮层能正确定位到底部
	if standalone {
		mainContent = lipgloss.PlaceVertical(height, lipgloss.Top, mainContent)
	}

	// 将版本信息叠加在左下角（状态栏上方）
	if standalone {
		mainContent = overlayVersionAtBottomLeft(mainContent, versionText, width, height)
	}

	// 渲染 Toast（如果有）
	toastStr := state.Toast.Render(width)

	// 组装最终结果
	result := mainContent

	// 如果有 Toast，叠加在右上角
	if toastStr != "" {
		result = overlayToast(result, toastStr, width)
	}

	if standalone {
		return renderSettingsInlineHelp(result, state, width, height)
	}
	return result
}

func (s State) InlineHelpHints(width int) []common.InlineHelpHint {
	hints := buildSettingsInlineHelpHints(s.ToPageState(s.Config, nil))
	return common.FitInlineHints(hints, max(1, width-1))
}

func (s State) InlineHelp(width int) string {
	return ansi.Truncate(common.FormatInlineHintRow(s.InlineHelpHints(width)), max(1, width-1), "")
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
	maxWidth := width - 2
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
				{Key: "↵", Desc: i18n.T("help.settings_edit_lang.confirm")},
				{Key: "Esc", Desc: i18n.T("help.settings_edit_lang.cancel")},
			}
		}
		if state.ActiveTab == 0 && state.SelectedSetting == ThemeSettingIndex() {
			// 主题项：Tab/方向键切换 + 应用 + 取消
			return []common.InlineHelpHint{
				{Key: "←→/Tab", Desc: i18n.T("help.settings_edit_theme.switch")},
				{Key: "↵", Desc: i18n.T("help.settings_edit_theme.confirm")},
				{Key: "Esc", Desc: i18n.T("help.settings_edit_theme.cancel")},
			}
		}
		if state.ActiveTab == 1 && state.SelectedSetting >= 0 && state.SelectedSetting < len(MihomoSettingKeys) {
			k := MihomoSettingKeys[state.SelectedSetting]
			if k == "allow-lan" || k == "log-level" {
				return []common.InlineHelpHint{
					{Key: "←→/Tab", Desc: i18n.T("help.settings_edit_lang.switch")},
					{Key: "↵", Desc: i18n.T("help.settings_edit_config.confirm")},
					{Key: "Esc", Desc: i18n.T("help.settings_edit_config.cancel")},
				}
			}
		}
		// 普通编辑项：移动光标 + 保存 + 取消
		return []common.InlineHelpHint{
			{Key: "←→", Desc: i18n.T("help.settings_edit_config.move")},
			{Key: "↵", Desc: i18n.T("help.settings_edit_config.confirm")},
			{Key: "Esc", Desc: i18n.T("help.settings_edit_config.cancel")},
		}
	}

	if state.ActiveTab == 1 {
		if state.SelectedSetting == mihomoApplyIndex {
			return []common.InlineHelpHint{
				{Key: "↑↓", Desc: i18n.T("help.settings.hint_select")},
				{Key: "↵", Desc: i18n.T("settings.mihomo.apply_button"), Tone: common.InlineHelpToneSuccess},
				{Key: "r", Desc: i18n.T("help.settings.hint_refresh")},
				{Key: "?", Desc: i18n.T("core.inline.help")},
			}
		}
		return []common.InlineHelpHint{
			{Key: "↑↓", Desc: i18n.T("help.settings.hint_select")},
			{Key: "↵", Desc: i18n.T("help.settings.hint_edit")},
			{Key: "Ctrl+S", Desc: i18n.T("settings.mihomo.apply_button")},
			{Key: "r", Desc: i18n.T("help.settings.hint_refresh")},
			{Key: "?", Desc: i18n.T("core.inline.help")},
		}
	}

	// 普通模式：切换标签 + 选择 + 编辑 + 刷新 + 帮助
	return []common.InlineHelpHint{
		{Key: "↑↓", Desc: i18n.T("help.settings.hint_select")},
		{Key: "↵", Desc: i18n.T("help.settings.hint_edit")},
		{Key: "r", Desc: i18n.T("help.settings.hint_refresh")},
		{Key: "?", Desc: i18n.T("core.inline.help")},
	}
}

// renderSettingsInlineHelp 渲染右下角内联帮助面板并叠加到页面上
func renderSettingsInlineHelp(page string, state PageState, width, height int) string {
	hints := common.FitInlineHints(buildSettingsInlineHelpHints(state), max(1, width-1))
	if len(hints) == 0 {
		return page
	}

	body := common.FormatInlineHintRow(hints)
	return common.OverlayHelpAtBottomRight(page, body, width, height)
}

// renderSettingItem 渲染单个设置项
func renderSettingItem(state PageState, index int, key string, label string, width int) string {
	width = max(24, width)
	value := GetSettingValue(state, key)
	labelWidth := settingsLabelWidth
	var themeRows [][]string
	if key == "theme" {
		labelWidth, themeRows = themeButtonLayout(width)
		label = ansi.Truncate(label+":", labelWidth-1, "…")
	} else {
		label += ":"
	}

	// 密钥特殊处理
	if state.ActiveTab == 1 && key == "secret" && value != "" {
		value = utils.MaskSecret(value)
	}

	// 标签样式
	labelStyle := lipgloss.NewStyle().
		Width(labelWidth).
		Align(lipgloss.Right).
		Foreground(common.TokyoMuted()).
		PaddingRight(1)

	selectedLabelStyle := labelStyle.
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
		renderedLabel = selectedLabelStyle.Render(label)
	} else {
		renderedLabel = labelStyle.Render(label)
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
		var rows []string
		for _, names := range themeRows {
			rows = append(rows, renderEnumTabs(valToRender, names, state.EditMode && index == state.SelectedSetting))
		}
		renderedValue = strings.Join(rows, "\n")
	} else if key == "border-style" {
		renderedValue = renderEnumTabs(i18n.T("settings.border."+value), borderStyleOptions(), false)
	} else if key == "allow-lan" {
		valToRender := value
		if state.EditMode && index == state.SelectedSetting {
			valToRender = state.EditValue
		}
		renderedValue = renderAllowLanTabs(valToRender, state.EditMode && index == state.SelectedSetting, width)
	} else if key == "log-level" {
		valToRender := value
		if state.EditMode && index == state.SelectedSetting {
			valToRender = state.EditValue
		}
		renderedValue = renderLogLevelTabs(valToRender, state.EditMode && index == state.SelectedSetting, width)
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
	} else if state.ActiveTab == 1 && value == "" {
		renderedValue = lipgloss.NewStyle().Foreground(common.TokyoMuted()).Render(i18n.T("settings.mihomo.empty"))
	} else {
		maxValWidth := max(1, width-4-settingsRowPaddingLeft-settingsLabelWidth)
		renderedValue = valueStyle.Render(ansi.Truncate(value, maxValWidth, "…"))
	}

	// 拼装每行的内容
	lineInner := lipgloss.JoinHorizontal(lipgloss.Top, renderedLabel, renderedValue)

	// 定义单行块的样式
	rowStyle := lipgloss.NewStyle().Width(width - 4).PaddingLeft(1)
	if key != "theme" {
		rowStyle = rowStyle.MaxHeight(1)
	}
	if index == state.SelectedSetting {
		rowStyle = rowStyle.Inherit(selectedBg)
	}

	return rowStyle.Render(lineInner)
}

func borderStyleOptions() []string {
	return []string{i18n.T("settings.border.rounded"), i18n.T("settings.border.square")}
}

func themeButtonLayout(width int) (labelWidth int, rows [][]string) {
	names := theme.Names()
	longest := 0
	for _, name := range names {
		longest = max(longest, settingsTabDisplayWidth(name))
	}
	room := max(24, width) - 4 - settingsRowPaddingLeft
	labelWidth = min(settingsLabelWidth, max(1, room-longest))
	available := room - labelWidth
	start, used := 0, 0
	for i, name := range names {
		// The separator takes one column between adjacent buttons.
		buttonWidth := settingsTabDisplayWidth(name)
		if i > start && used+1+buttonWidth > available {
			rows = append(rows, names[start:i])
			start, used = i, 0
		}
		if i > start {
			used++
		}
		used += buttonWidth
	}
	return labelWidth, append(rows, names[start:])
}

func enumTabsWindow(current string, labels []string, width int) (offset, visible int) {
	visible = max(0, width-4-settingsRowPaddingLeft-settingsLabelWidth)
	end := 0
	for _, name := range labels {
		end += settingsTabDisplayWidth(name)
		if name == current {
			return max(0, end-visible), visible
		}
		end++
	}
	return 0, visible
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

func renderAllowLanTabs(current string, editMode bool, width int) string {
	labels := overrideOptionLabels(allowLanValues)
	currentLabel := overrideOptionLabel(current)
	offset, visible := enumTabsWindow(currentLabel, labels, width)
	return ansi.Cut(renderEnumTabs(currentLabel, labels, editMode), offset, offset+visible)
}

func renderLogLevelTabs(current string, editMode bool, width int) string {
	labels := overrideOptionLabels(logLevelValues)
	currentLabel := overrideOptionLabel(current)
	offset, visible := enumTabsWindow(currentLabel, labels, width)
	return ansi.Cut(renderEnumTabs(currentLabel, labels, editMode), offset, offset+visible)
}

func renderMihomoApplyRow(state PageState, width int) string {
	labelSpacer := lipgloss.NewStyle().
		Width(settingsLabelWidth).
		PaddingRight(1).
		Render("")

	btnLabel := mihomoApplyButtonLabel(state.MihomoSaving)
	btnStyle := lipgloss.NewStyle().
		Background(common.TokyoSelected()).
		Foreground(common.TokyoCyan()).
		Bold(true).
		Padding(0, 1)
	if state.SelectedSetting == mihomoApplyIndex {
		btnStyle = btnStyle.Background(common.TokyoBlue()).Foreground(common.Bright())
		if state.MihomoDirty {
			btnStyle = btnStyle.Background(common.TokyoGreen())
		}
	} else if state.MihomoDirty {
		btnStyle = btnStyle.Background(common.TokyoBlue()).Foreground(common.Bright())
	}
	renderedBtn := btnStyle.Render(" " + btnLabel + " ")

	rowWidth := width - 4
	if rowWidth < settingsMinRowWidth {
		rowWidth = settingsMinRowWidth
	}
	availHint := max(0, rowWidth-settingsRowPaddingLeft-settingsLabelWidth-lipgloss.Width(renderedBtn))
	var renderedHint string
	if !state.MihomoSaving && availHint > 2 {
		hintText := i18n.T("settings.mihomo.apply_hint")
		hintStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted())
		if state.MihomoDirty {
			hintText = i18n.T("settings.mihomo.dirty_hint")
			hintStyle = lipgloss.NewStyle().Foreground(common.TokyoYellow())
		}
		renderedHint = hintStyle.Render(ansi.Truncate("  "+hintText, availHint, "…"))
	}

	lineInner := lipgloss.JoinHorizontal(lipgloss.Top, labelSpacer, renderedBtn, renderedHint)
	rowStyle := lipgloss.NewStyle().Width(rowWidth).PaddingLeft(1)
	if state.SelectedSetting == mihomoApplyIndex {
		rowStyle = rowStyle.Inherit(lipgloss.NewStyle().Background(common.TokyoSelected()))
	}
	return rowStyle.Render(lineInner)
}

// settingsTabBarContentY 标签栏内容行（可点击行）相对页面内容顶部的 Y 坐标。
// 布局：上边框(0) → 内容行位于 pageY=1。
const settingsTabBarContentY = 1

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
	topLine := borderStyle.Render(theme.Frame().TopLeft + strings.Repeat("─", innerWidth) + theme.Frame().TopRight)
	middleLine := borderStyle.Render("│") + content + borderStyle.Render("│")
	bottomLine := borderStyle.Render(theme.Frame().BottomLeft + strings.Repeat("─", innerWidth) + theme.Frame().BottomRight)

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
