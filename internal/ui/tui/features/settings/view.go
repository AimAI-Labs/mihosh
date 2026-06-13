package settings

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
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

var SettingKeys = []string{"api-address", "secret", "test-url", "timeout", "proxy-address", "language", "auto-refresh-interval"}

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

	// 渲染当前选中项的描述
	var descSection string
	if state.SelectedSetting >= 0 && state.SelectedSetting < len(SettingKeys) {
		descStyle := lipgloss.NewStyle().
			Foreground(common.TokyoMuted).
			Italic(true).
			MarginTop(1)
		descSection = descStyle.Render("💡 " + GetSettingDesc(state.SelectedSetting))
	}

	// 组装主要内容
	mainContent := lipgloss.JoinVertical(
		lipgloss.Left,
		settingsPanel,
		descSection,
	)

	// 包裹容器边距
	mainContent = containerStyle.Render(mainContent)

	// 渲染底部版本信息
	footerStyle := lipgloss.NewStyle().
		Foreground(common.TokyoMuted).
		Align(lipgloss.Right).
		Width(width - 2).
		PaddingRight(2)

	mihomoVer := state.MihomoVersion
	if mihomoVer == "" {
		mihomoVer = "..."
	}

	mihoshLink := utils.CreateHyperlink("https://github.com/AimAI-Labs/mihosh", "Mihosh "+model.Version)
	mihomoLink := utils.CreateHyperlink("https://github.com/MetaCubeX/mihomo", "Mihomo "+mihomoVer)

	footerText := fmt.Sprintf("%s | Built: %s | %s", mihoshLink, model.Date, mihomoLink)
	footerContent := footerStyle.Render(footerText)

	// 使用 lipgloss.Place 将内容和 footer 定位，如果高度不够，直接返回内容
	if height > lipgloss.Height(mainContent)+2 {
		mainContent = lipgloss.PlaceVertical(height-1, lipgloss.Top, mainContent)
		mainContent = lipgloss.JoinVertical(lipgloss.Left, mainContent, footerContent)
	} else {
		// 如果高度不足，直接追加在后面
		mainContent = lipgloss.JoinVertical(lipgloss.Left, mainContent, footerContent)
	}

	// 渲染 Toast（如果有）
	toastStr := state.Toast.Render(width)

	// 组装最终结果
	result := mainContent

	// 如果有 Toast，叠加在右上角
	if toastStr != "" {
		result = overlayToast(result, toastStr, width)
	}

	return result
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
		Foreground(common.TokyoMuted).
		PaddingRight(1)

	selectedLabelStyle := labelStyle.Copy().
		Foreground(common.TokyoCyan).
		Bold(true)

	// 值样式
	valueStyle := lipgloss.NewStyle().
		Foreground(common.TokyoForeground)

	// 选中状态样式
	selectedBg := lipgloss.NewStyle().
		Background(common.TokyoSelected)

	// 编辑模式样式
	editBoxStyle := lipgloss.NewStyle().
		Foreground(common.TokyoYellow).
		Background(lipgloss.Color("#1A1B26")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(common.TokyoBlue).
		Padding(0, 1)

	// 光标样式
	cursorStyle := lipgloss.NewStyle().
		Background(common.TokyoForeground).
		Foreground(lipgloss.Color("#1A1B26"))

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
		Background(common.TokyoBlue).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 1)

	if editMode {
		activeStyle = activeStyle.Background(common.TokyoGreen)
	}

	inactiveStyle := lipgloss.NewStyle().
		Foreground(common.TokyoMuted).
		Background(lipgloss.Color("#1A1B26")).
		Padding(0, 1)

	separatorStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted)

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

// overlayToast 在页面右上角叠加 Toast
func overlayToast(page, toast string, width int) string {
	pageLines := strings.Split(page, "\n")
	toastLines := strings.Split(toast, "\n")

	// 计算 Toast 应该放置的位置（右上角）
	toastHeight := len(toastLines)
	if toastHeight > len(pageLines) {
		toastHeight = len(pageLines)
	}

	// 从顶部开始叠加
	for i := 0; i < toastHeight; i++ {
		if i < len(pageLines) {
			pageLines[i] = toastLines[i]
		}
	}

	return strings.Join(pageLines, "\n")
}
