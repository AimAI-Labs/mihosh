package settings

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/utils"
	"github.com/charmbracelet/lipgloss"
)

const (
	settingsLabelWidth  = 20
	settingsMinRowWidth = 40
	settingsDescWidth   = 30
)

var SettingKeys = []string{"api-address", "secret", "test-url", "timeout", "proxy-address", "language", "auto-refresh-interval"}
var SettingLabels = []string{"API 地址", "密钥", "测速URL", "超时(ms)", "代理地址", "语言", "自动刷新(秒)"}
var SettingDescs = []string{
	"Clash API 服务地址",
	"API 认证密钥",
	"用于测试节点延迟的 URL",
	"连接超时时间（毫秒）",
	"HTTP/SOCKS5 代理地址",
	"界面显示语言",
	"数据自动刷新间隔（秒）",
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
	for i, label := range SettingLabels {
		item := renderSettingItem(state, i, label, width)
		settingItems = append(settingItems, item)
	}

	// 使用 Tokyo 面板包裹设置列表
	listContent := strings.Join(settingItems, "\n")
	settingsPanel := common.RenderTokyoPanel("配置项", listContent, width-4)

	// 渲染当前选中项的描述
	var descSection string
	if state.SelectedSetting >= 0 && state.SelectedSetting < len(SettingDescs) {
		descStyle := lipgloss.NewStyle().
			Foreground(common.TokyoMuted).
			Italic(true).
			MarginTop(1)
		descSection = descStyle.Render("💡 " + SettingDescs[state.SelectedSetting])
	}

	// 操作提示
	var helpText string
	if state.EditMode {
		if state.SelectedSetting == LanguageSettingIndex() {
			helpText = "[←/→/Tab]切换 [Enter]保存 [Esc]取消"
		} else {
			helpText = "[Enter]保存 [Esc]取消 [←/→]移动光标"
		}
	} else {
		helpText = "[↑/↓]选择 [Enter/双击]编辑"
	}

	// 组装主要内容
	mainContent := lipgloss.JoinVertical(
		lipgloss.Left,
		settingsPanel,
		descSection,
	)

	// 包裹容器边距
	mainContent = containerStyle.Render(mainContent)

	// 计算内容高度
	contentLines := strings.Count(mainContent, "\n") + 1

	// 渲染 Toast（如果有）
	toastStr := state.Toast.Render(width)
	if toastStr != "" {
		toastLines := strings.Count(toastStr, "\n") + 1
		contentLines += toastLines
	}

	// 渲染底部提示
	footer := common.RenderFooter(width, height, contentLines, helpText)

	// 组装最终结果
	result := mainContent + footer

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
