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
	settingsLabelWidth  = 24
	settingsMinRowWidth = 40
)

var SettingKeys = []string{"api-address", "secret", "test-url", "timeout", "proxy-address", "language"}
var SettingLabels = []string{"API 地址", "密钥", "测速URL", "超时(ms)", "代理地址", "语言"}

// PageState 设置页面状态
type PageState struct {
	Config          *config.Config
	SelectedSetting int
	EditMode        bool
	EditValue       string
	EditCursor      int
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
	}
	return ""
}

// RenderSettingsPage 渲染设置页面
func RenderSettingsPage(state PageState, width, height int) string {
	// 定义基础样式
	headerStyle := common.PageHeaderStyle.
		MarginBottom(1)

	// 容器统一样式，整体偏移
	containerStyle := lipgloss.NewStyle().
		MarginLeft(2).
		MarginTop(1)

	// 标签样式：固定宽度、左侧无填充、文字靠右对齐
	labelStyle := lipgloss.NewStyle().
		Width(settingsLabelWidth).
		Align(lipgloss.Right).
		Foreground(common.CMuted).
		PaddingRight(2)

	// 选中状态下的标签
	selectedLabelStyle := labelStyle.Copy().
		Foreground(common.CActive).
		Bold(true)

	// 普通行的值样式
	valueStyle := lipgloss.NewStyle().
		Foreground(common.CWhite)

	// 选中项的行样式：整行背景色
	selectedRowStyle := lipgloss.NewStyle().
		Background(common.CHighlight)

	// 编辑模式的值样式
	editBoxStyle := lipgloss.NewStyle().
		Foreground(common.CWarning).
		Background(common.CDim).
		Padding(0, 1)

	// 模拟文本输入光标（反色块）
	cursorStyle := lipgloss.NewStyle().
		Background(common.CWhite).
		Foreground(lipgloss.Color("#000000"))

	// 配置项列表
	var lines []string
	for i, label := range SettingLabels {
		value := GetSettingValue(state.Config, i)

		// 密钥特殊处理
		if i == 1 && value != "" {
			value = utils.MaskSecret(value)
		}

		var renderedLabel string
		if i == state.SelectedSetting {
			renderedLabel = selectedLabelStyle.Render(label + ":")
		} else {
			renderedLabel = labelStyle.Render(label + ":")
		}

		var renderedValue string
		if i == 5 {
			// 语言选项使用 Tab 组件渲染
			valToRender := value
			if state.EditMode && i == state.SelectedSetting {
				valToRender = state.EditValue
			}
			renderedValue = renderLanguageTabs(valToRender, state.EditMode && i == state.SelectedSetting)
		} else if state.EditMode && i == state.SelectedSetting {
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

		// 定义单行块的样式，使选中框能拉伸一定宽度
		rowWidth := width - 6
		if rowWidth < settingsMinRowWidth {
			rowWidth = settingsMinRowWidth
		}

		rowStyle := lipgloss.NewStyle().Width(rowWidth).PaddingLeft(1)
		if i == state.SelectedSetting {
			rowStyle = rowStyle.Inherit(selectedRowStyle)
		}

		lines = append(lines, rowStyle.Render(lineInner))
	}

	// 操作提示
	var helpText string
	if state.EditMode {
		if state.SelectedSetting == 5 {
			helpText = common.MutedStyle.Render("[←/→/Tab]切换 [Enter]保存 [Esc]取消")
		} else {
			helpText = common.MutedStyle.Render("[Enter]保存 [Esc]取消")
		}
	} else {
		helpText = common.MutedStyle.Render("[↑/↓]选择 [Enter/双击]编辑")
	}

	mainContent := lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render("系统设置"),
		strings.Join(lines, "\n"),
	)

	// 包裹容器边距
	mainContent = containerStyle.Render(mainContent)

	contentLines := strings.Count(mainContent, "\n") + 1
	footer := common.RenderFooter(width, height, contentLines, helpText)
	return mainContent + footer
}

func renderLanguageTabs(currentLang string, editMode bool) string {
	modes := []string{"auto", "zh-CN", "en-US"}
	var parts []string

	activeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#555555")).
		Foreground(common.CWhite).
		Padding(0, 1)

	if editMode {
		activeStyle = activeStyle.Background(lipgloss.Color("#007BFF")).Bold(true)
	}

	inactiveStyle := lipgloss.NewStyle().
		Foreground(common.CMuted).
		Padding(0, 1)

	borderStyle := lipgloss.NewStyle().Foreground(common.CMuted)

	for i, m := range modes {
		if currentLang == m {
			parts = append(parts, activeStyle.Render(" "+m+" "))
		} else {
			parts = append(parts, inactiveStyle.Render(" "+m+" "))
		}
		if i < len(modes)-1 {
			parts = append(parts, borderStyle.Render("│"))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}
