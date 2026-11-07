package nodes

import (
	"regexp"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

var hexColorRegex = regexp.MustCompile(`#([0-9a-fA-F]{6})`)

func parseHexColor(s string) (lipgloss.Color, bool) {
	match := hexColorRegex.FindString(s)
	if match != "" {
		return lipgloss.Color(match), true
	}
	return lipgloss.Color(""), false
}

// MouseTarget 表示 nodes 页面鼠标命中的目标
type MouseTarget int

const (
	MouseTargetNone MouseTarget = iota
	MouseTargetGroup
	MouseTargetProxy
	MouseTargetMode
)

// MouseHit 是 nodes 页面鼠标命中结果
type MouseHit struct {
	Target MouseTarget
	Index  int
}

// RenderModeSwitchComponent 渲染模式切换按钮（带边框，与导航栏风格一致）
func RenderModeSwitchComponent(currentMode string, width int) string {
	modes := []struct {
		Label string
		Value string
	}{
		{i18n.T("nodes.mode_rule"), "rule"},
		{i18n.T("nodes.mode_global"), "global"},
		{i18n.T("nodes.mode_direct"), "direct"},
	}

	activeStyle := lipgloss.NewStyle().
		Background(common.TokyoSelected()).
		Foreground(common.TokyoCyan()).
		Bold(true)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(common.TokyoBlue())
	separatorStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted())

	var parts []string
	for i, m := range modes {
		label := " " + m.Label + " "
		if strings.ToLower(currentMode) == m.Value {
			parts = append(parts, activeStyle.Render(label))
		} else {
			parts = append(parts, inactiveStyle.Render(label))
		}
		if i < len(modes)-1 {
			parts = append(parts, separatorStyle.Render("│"))
		}
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

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

// ============================================================
//  内联帮助提示面板（右下角浮层）
// ============================================================
//
// 通用渲染逻辑（InlineHelpHint / FormatInlineHintRow /
// OverlayHelpAtBottomRight）已抽取至 components/common，
// 各页面共享，避免重复实现。

// buildNodesInlineHelpHints 根据节点页上下文构建内联帮助条目
func buildNodesInlineHelpHints(state PageState) []common.InlineHelpHint {
	if state.FilterMode {
		return []common.InlineHelpHint{
			{Key: "↵", Desc: i18n.T("help.nodes_search.confirm")},
			{Key: "Esc", Desc: i18n.T("help.nodes_search.cancel")},
			{Key: "Ctrl+R/F", Desc: "正则/模糊"},
		}
	}

	if state.ShowTestDetail {
		return []common.InlineHelpHint{
			{Key: "↑/↓", Desc: i18n.T("help.nodes_test_detail.scroll")},
			{Key: "Home/End", Desc: i18n.T("help.nodes_test_detail.jump")},
			{Key: "f/Esc", Desc: i18n.T("help.nodes_test_detail.close")},
		}
	}

	// 普通模式：单行精简提示（核心操作 + ? 打开完整帮助）
	return []common.InlineHelpHint{
		{Key: "↑↓", Desc: i18n.T("help.nodes.hint_select")},
		{Key: "←→", Desc: i18n.T("help.nodes.hint_group")},
		{Key: "↵", Desc: i18n.T("help.nodes.hint_switch")},
		{Key: "t", Desc: i18n.T("help.nodes.hint_test")},
		{Key: "a", Desc: i18n.T("help.nodes.hint_test_all")},
		{Key: "m", Desc: i18n.T("help.nodes.hint_mode")},
		{Key: "s", Desc: i18n.T("help.nodes.hint_sort")},
		{Key: "/", Desc: i18n.T("help.nodes.hint_search")},
		{Key: "f", Desc: i18n.T("help.nodes.hint_result")},
		{Key: "?", Desc: i18n.T("help.nodes.hint_more")},
	}
}

// renderNodesInlineHelp 渲染右下角内联帮助面板并叠加到页面上
func renderNodesInlineHelp(page string, state PageState) string {
	hints := buildNodesInlineHelpHints(state)
	if len(hints) == 0 {
		return page
	}

	// 单行精简展示（无边框，紧贴底栏上边）
	body := common.FormatInlineHintRow(hints)

	return common.OverlayHelpAtBottomRight(page, body, state.Width, state.Height)
}
