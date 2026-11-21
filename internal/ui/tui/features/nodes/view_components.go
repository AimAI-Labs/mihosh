package nodes

import (
	"regexp"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
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
	MouseTargetGroupTest
	MouseTargetProxy
	MouseTargetMode
	MouseTargetLayout
)

// MouseHit 是 nodes 页面鼠标命中结果
type MouseHit struct {
	Target MouseTarget
	Index  int
}

func modeSwitchLabels() []string {
	return []string{
		i18n.T("nodes.mode_rule"),
		i18n.T("nodes.mode_global"),
		i18n.T("nodes.mode_direct"),
	}
}

func layoutSwitchLabels() []string {
	return []string{
		i18n.T("nodes.layout_cards"),
		i18n.T("nodes.layout_columns"),
	}
}

// RenderModeSwitchComponent 渲染模式切换按钮与最右侧布局切换菜单（带边框，与导航栏风格一致）
func RenderModeSwitchComponent(currentMode string, width int, layout ...NodesLayout) string {
	currentLayout := LayoutCards
	if len(layout) > 0 {
		currentLayout = layout[0]
	}

	modeValues := []string{"rule", "global", "direct"}
	modes := modeSwitchLabels()

	activeStyle := lipgloss.NewStyle().
		Background(theme.Resolve(common.TokyoSelected())).
		Foreground(theme.Resolve(common.TokyoCyan())).
		Bold(true)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(theme.Resolve(common.TokyoBlue()))
	separatorStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoMuted()))

	var leftParts []string
	for i, label := range modes {
		cell := " " + label + " "
		if strings.ToLower(currentMode) == modeValues[i] {
			leftParts = append(leftParts, activeStyle.Render(cell))
		} else {
			leftParts = append(leftParts, inactiveStyle.Render(cell))
		}
		if i < len(modes)-1 {
			leftParts = append(leftParts, separatorStyle.Render("│"))
		}
	}
	leftContent := lipgloss.JoinHorizontal(lipgloss.Left, leftParts...)

	layouts := layoutSwitchLabels()
	var rightParts []string
	for i, label := range layouts {
		cell := " " + label + " "
		if NodesLayout(i) == currentLayout {
			rightParts = append(rightParts, activeStyle.Render(cell))
		} else {
			rightParts = append(rightParts, inactiveStyle.Render(cell))
		}
		if i < len(layouts)-1 {
			rightParts = append(rightParts, separatorStyle.Render("│"))
		}
	}
	rightContent := lipgloss.JoinHorizontal(lipgloss.Left, rightParts...)

	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	leftW := lipgloss.Width(leftContent)
	rightW := lipgloss.Width(rightContent)
	var content string
	if leftW+1+rightW <= innerWidth {
		content = leftContent + strings.Repeat(" ", innerWidth-leftW-rightW) + rightContent
	} else {
		content = common.PadString(common.TruncateDisplay(leftContent, innerWidth), innerWidth)
	}

	borderStyle := lipgloss.NewStyle().Foreground(theme.Resolve(common.TokyoBlue()))
	topLine := borderStyle.Render(theme.Frame().TopLeft + strings.Repeat("─", innerWidth) + theme.Frame().TopRight)
	middleLine := borderStyle.Render("│") + content + borderStyle.Render("│")
	bottomLine := borderStyle.Render(theme.Frame().BottomLeft + strings.Repeat("─", innerWidth) + theme.Frame().BottomRight)

	return topLine + "\n" + middleLine + "\n" + bottomLine
}

// resolveModeBarMouseHit 检测顶部模式/布局切换栏上的鼠标点击。
func resolveModeBarMouseHit(width, pageX, pageY int) MouseHit {
	none := MouseHit{Target: MouseTargetNone, Index: -1}
	if pageY != 1 || pageX < 1 || width < 3 {
		return none
	}
	innerWidth := width - 2

	curX := 1
	modes := modeSwitchLabels()
	for i, label := range modes {
		w := common.DisplayWidth(" " + label + " ")
		if pageX >= curX && pageX < curX+w && curX+w <= width-1 {
			return MouseHit{Target: MouseTargetMode, Index: i}
		}
		curX += w
		if i < len(modes)-1 {
			curX++
		}
	}
	leftW := curX - 1

	layouts := layoutSwitchLabels()
	rightW := 0
	layoutWidths := make([]int, len(layouts))
	for i, label := range layouts {
		layoutWidths[i] = common.DisplayWidth(" " + label + " ")
		rightW += layoutWidths[i]
		if i < len(layouts)-1 {
			rightW++
		}
	}
	if leftW+1+rightW <= innerWidth {
		rightX := 1 + innerWidth - rightW
		for i, w := range layoutWidths {
			if pageX >= rightX && pageX < rightX+w {
				return MouseHit{Target: MouseTargetLayout, Index: i}
			}
			rightX += w
			if i < len(layoutWidths)-1 {
				rightX++
			}
		}
	}
	return none
}

// ============================================================
//  内联帮助提示面板（右下角浮层）
// ============================================================
//
// 通用渲染逻辑（InlineHelpHint / FormatInlineHintRow /
// OverlayHelpAtBottomRight）已抽取至 components/common，
// 各页面共享，避免重复实现。

func (s State) InlineHelpHints() []common.InlineHelpHint {
	if s.ChainModal.Open {
		return []common.InlineHelpHint{
			{Key: "↵", Desc: i18n.T("help.nodes_chain.toggle")},
			{Key: "Tab", Desc: i18n.T("help.nodes_chain.focus")},
			{Key: "[/]", Desc: i18n.T("help.nodes_chain.reorder")},
			{Key: "Ctrl+S", Desc: i18n.T("help.nodes_chain.apply"), Tone: common.InlineHelpTonePrimary},
			{Key: "Esc", Desc: i18n.T("help.nodes_chain.close")},
		}
	}

	if s.NodeFilterMode {
		return []common.InlineHelpHint{
			{Key: "↵", Desc: i18n.T("help.nodes_search.confirm")},
			{Key: "Esc", Desc: i18n.T("help.nodes_search.cancel")},
			{Key: "Ctrl+R/F", Desc: "正则/模糊"},
		}
	}

	if s.ShowTestDetail {
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
		{Key: "v", Desc: i18n.T("help.nodes.hint_layout"), Tone: common.InlineHelpToneInfo},
		{Key: "g", Desc: i18n.T("help.nodes.hint_locate"), Tone: common.InlineHelpToneInfo},
		{Key: "c", Desc: i18n.T("help.nodes.hint_chain"), Tone: common.InlineHelpToneInfo},
		{Key: "s", Desc: i18n.T("help.nodes.hint_sort"), Tone: common.InlineHelpToneInfo},
		{Key: "/", Desc: i18n.T("help.nodes.hint_search")},
		{Key: "f", Desc: i18n.T("help.nodes.hint_result")},
		{Key: "?", Desc: i18n.T("help.nodes.hint_more")},
	}
}

func (s State) InlineHelpFirstWidth(width int) int {
	if s.ChainModal.Open || s.ShowTestDetail || s.NodeFilterMode {
		return width - 1
	}
	if s.NodeFilter != "" {
		return width / 2
	}
	chain, _ := s.ActiveGroupChain()
	if chain.Enabled && len(chain.Nodes) >= 2 {
		badgeW := common.DisplayWidth(formatActiveChainSummary(chain, max(12, width/3)))
		return max(1, width-badgeW-3)
	}
	return width - 1
}
