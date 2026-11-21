package common

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ============================================================
//  内联帮助提示面板（右下角交互菜单）—— 通用渲染与命中辅助
// ============================================================
//
// 各页面共享此实现：页面只需构造 []InlineHelpHint（按键 + 描述），
// 调用 OverlayHelpAtBottomRight 将其叠加到页面右下角，并通过
// ResolveInlineHelpHit 将点击位置映射回对应的快捷键动作。

// InlineHelpTone 定义右下角快捷菜单项的语义色彩类别。
type InlineHelpTone uint8

const (
	InlineHelpToneAuto    InlineHelpTone = iota
	InlineHelpToneNav                    // 纯导航提示（不可点击，弱化展示）
	InlineHelpTonePrimary                // 主操作（确认/切换/运行/刷新）
	InlineHelpToneSuccess                // 正向操作（新增/测速/应用/更新）
	InlineHelpToneInfo                   // 浏览与筛选（搜索/模式/排序/视图/标签）
	InlineHelpToneWarning                // 取消/返回/重启/退格
	InlineHelpToneDanger                 // 危险操作（删除/关闭/清空）
	InlineHelpToneHelp                   // 帮助入口（?）
)

// InlineHelp 样式函数（支持主题热切换）

// InlineHelpKeyStyle 按键文本样式（醒目黄）
func InlineHelpKeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(resolve(TokyoYellow()))
}

// InlineHelpDimStyle 分隔符等次要文本样式（灰）
func InlineHelpDimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(resolve(TokyoMuted()))
}

// InlineHelpDescStyle 描述文本样式（柔和蓝）
func InlineHelpDescStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(resolve(Dim())) }

// InlineHelpHint 内联帮助条目（按键 + 描述 + 可选动作与语义色）
type InlineHelpHint struct {
	Key    string
	Desc   string
	Action string
	Tone   InlineHelpTone
}

// Clickable 返回该条目是否支持点击触发对应操作。
func (h InlineHelpHint) Clickable() bool {
	_, ok := h.KeyMsg()
	return ok
}

// KeyMsg 返回点击该条目时应派发的按键消息。
func (h InlineHelpHint) KeyMsg() (tea.KeyMsg, bool) {
	action := h.Action
	if action == "" {
		action = defaultHintAction(h.Key)
	}
	if action == "" {
		return tea.KeyMsg{}, false
	}
	switch action {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}, true
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}, true
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}, true
	case "space", " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}, true
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}, true
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}, true
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}, true
	case "ctrl+f":
		return tea.KeyMsg{Type: tea.KeyCtrlF}, true
	default:
		runes := []rune(action)
		if len(runes) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: runes}, true
		}
		return tea.KeyMsg{}, false
	}
}

// ResolveTone 返回条目的最终语义色彩类别。
func (h InlineHelpHint) ResolveTone() InlineHelpTone {
	if h.Tone != InlineHelpToneAuto {
		return h.Tone
	}
	if !h.Clickable() {
		return InlineHelpToneNav
	}
	switch h.Key {
	case "?":
		return InlineHelpToneHelp
	case "d", "x/X":
		return InlineHelpToneDanger
	case "Esc", "Esc/q", "Esc/n", "f/Esc", "Enter/Esc", "z", "⌫":
		return InlineHelpToneWarning
	case "n", "t", "a", "s/S", "Ctrl+S", "Enter/y":
		return InlineHelpToneSuccess
	case "↵", "Enter", "r", "u", "Space":
		return InlineHelpTonePrimary
	default:
		return InlineHelpToneInfo
	}
}

func defaultHintAction(key string) string {
	switch key {
	case "", "↑↓", "↑/↓", "←→", "←/→", "Home/End", "…":
		return ""
	case "↵", "Enter", "Enter/y":
		return "enter"
	case "Esc", "Esc/q", "Esc/n", "f/Esc", "Enter/Esc":
		return "esc"
	case "Tab", "←→/Tab":
		return "tab"
	case "Space":
		return "space"
	case "⌫":
		return "backspace"
	case "Ctrl+S":
		return "ctrl+s"
	case "Ctrl+R/F":
		return "ctrl+r"
	case "[/]":
		return "]"
	}
	if len([]rune(key)) == 1 {
		return key
	}
	if first, _, ok := strings.Cut(key, "/"); ok && len([]rune(first)) == 1 {
		return first
	}
	return ""
}

func inlineHelpToneColor(tone InlineHelpTone) lipgloss.Color {
	switch tone {
	case InlineHelpTonePrimary:
		return Primary()
	case InlineHelpToneSuccess:
		return Success()
	case InlineHelpToneInfo:
		return Info()
	case InlineHelpToneWarning:
		return Warning()
	case InlineHelpToneDanger:
		return Danger()
	case InlineHelpToneHelp:
		return Secondary()
	default:
		return Muted()
	}
}

func formatInlineHintItem(h InlineHelpHint) string {
	tone := h.ResolveTone()
	if tone == InlineHelpToneNav || !h.Clickable() {
		return InlineHelpDimStyle().Render(h.Key) + " " + InlineHelpDescStyle().Render(h.Desc)
	}
	bg := resolve(Selected())
	keyStyle := lipgloss.NewStyle().Foreground(resolve(inlineHelpToneColor(tone))).Background(bg).Bold(true)
	spaceStyle := lipgloss.NewStyle().Background(bg)
	descStyle := lipgloss.NewStyle().Foreground(resolve(TokyoForeground())).Background(bg)
	return keyStyle.Render(h.Key) + spaceStyle.Render(" ") + descStyle.Render(h.Desc)
}

// FormatInlineHintRow 将一组帮助条目格式化为 "key desc · key desc" 单行交互菜单
func FormatInlineHintRow(hints []InlineHelpHint) string {
	sep := InlineHelpDimStyle().Render(" · ")
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, formatInlineHintItem(h))
	}
	return strings.Join(parts, sep)
}

// WrapInlineHints groups complete items by display width while keeping their order.
func WrapInlineHints(hints []InlineHelpHint, width int) [][]InlineHelpHint {
	var rows [][]InlineHelpHint
	start, used := 0, 0
	for i, hint := range hints {
		itemWidth := lipgloss.Width(hint.Key + " " + hint.Desc)
		if i > start {
			if used+3+itemWidth > width {
				rows = append(rows, hints[start:i])
				start, used = i, 0
			} else {
				used += 3
			}
		}
		used += itemWidth
	}
	if start < len(hints) {
		rows = append(rows, hints[start:])
	}
	return rows
}

// InlineHintAt finds the clickable item at x in a left-aligned hint row.
func InlineHintAt(hints []InlineHelpHint, x, width int) (InlineHelpHint, bool) {
	if x < 0 || x >= width {
		return InlineHelpHint{}, false
	}
	start := 0
	for _, hint := range hints {
		end := start + lipgloss.Width(hint.Key+" "+hint.Desc)
		if x >= start && x < end && hint.Clickable() {
			return hint, true
		}
		start = end + 3
	}
	return InlineHelpHint{}, false
}

// FitInlineHints 将条目裁切到 maxWidth 内，优先保留可点击菜单项和末尾的帮助/取消项。
func FitInlineHints(hints []InlineHelpHint, maxWidth int) []InlineHelpHint {
	if len(hints) == 0 || maxWidth <= 0 || lipgloss.Width(FormatInlineHintRow(hints)) <= maxWidth {
		return hints
	}
	fitted := make([]InlineHelpHint, 0, len(hints))
	for _, h := range hints {
		if h.Clickable() {
			fitted = append(fitted, h)
		}
	}
	if len(fitted) == 0 {
		fitted = append(fitted, hints...)
	}
	if lipgloss.Width(FormatInlineHintRow(fitted)) <= maxWidth || len(fitted) == 1 {
		return fitted
	}
	last := fitted[len(fitted)-1]
	prefix := append([]InlineHelpHint(nil), fitted[:len(fitted)-1]...)
	for len(prefix) > 0 && lipgloss.Width(FormatInlineHintRow(append(append([]InlineHelpHint(nil), prefix...), last))) > maxWidth {
		prefix = prefix[:len(prefix)-1]
	}
	return append(prefix, last)
}

// ResolveInlineHelpHit 检测 (x, y) 是否命中右下角内联菜单中的某个可点击条目。
func ResolveInlineHelpHit(hints []InlineHelpHint, x, y, width, height int) (InlineHelpHint, bool) {
	if len(hints) == 0 || width <= 0 || height <= 0 || y != height-1 || x < 0 || x >= width {
		return InlineHelpHint{}, false
	}
	panelW := lipgloss.Width(FormatInlineHintRow(hints))
	startCol := width - panelW - 1
	if startCol < 0 {
		startCol = 0
	}
	return InlineHintAt(hints, x-startCol, max(1, width-1)-startCol)
}

// OverlayHelpAtBottomRight 将帮助面板叠加在页面右下角（紧贴底栏上方，距右 1 字符）。
// panel 为已渲染好的单行（或多行）帮助文本；width/height 为页面内容区的可用尺寸。
func OverlayHelpAtBottomRight(base, panel string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	panelLines := strings.Split(panel, "\n")
	panelH := len(panelLines)
	if panelH == 0 {
		return base
	}
	panelW := 0
	for _, l := range panelLines {
		if w := lipgloss.Width(l); w > panelW {
			panelW = w
		}
	}

	// 定位：紧贴底栏上边（pageContent 最后一行），距右 1 字符
	startRow := len(baseLines) - panelH
	if startRow < 0 {
		startRow = 0
	}
	startCol := width - panelW - 1
	if startCol < 0 {
		startCol = 0
	}

	for i, pl := range panelLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}
		bl := baseLines[row]
		blW := lipgloss.Width(bl)
		plW := lipgloss.Width(pl)

		// 截取底层左侧
		leftPart := ""
		if startCol > 0 {
			leftPart = ansi.Cut(bl, 0, startCol)
			if lw := lipgloss.Width(leftPart); lw < startCol {
				leftPart += strings.Repeat(" ", startCol-lw)
			}
		}

		// 截取底层右侧
		rightPart := ""
		if blW > startCol+plW {
			rightPart = ansi.Cut(bl, startCol+plW, blW)
		}

		baseLines[row] = leftPart + pl + rightPart
	}

	return strings.Join(baseLines, "\n")
}
