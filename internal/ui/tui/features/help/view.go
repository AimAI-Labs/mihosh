package help

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// keybinding 单条快捷键条目
type keybinding struct {
	key  string
	desc string
}

// section 快捷键分组
type section struct {
	title    string
	bindings []keybinding
}

// helpSections 所有快捷键分组定义
var helpSections = []section{
	{
		title: "全局",
		bindings: []keybinding{
			{"?", "显示/关闭帮助"},
			{"Tab / Shift+Tab", "切换页面"},
			{"1-5", "直接跳转页面"},
			{"r", "刷新当前页面"},
			{"q / Ctrl+C", "退出程序"},
		},
	},
	{
		title: "节点管理 [1]",
		bindings: []keybinding{
			{"↑/↓  k/j", "选择节点"},
			{"←/→  h/l", "切换策略组"},
			{"Enter", "切换到选中节点"},
			{"t", "测速当前节点"},
			{"a", "测速当前组所有节点"},
			{"m", "切换代理模式"},
			{"s", "切换排序方式"},
			{"/", "搜索节点"},
			{"f", "查看测速失败详情"},
		},
	},
	{
		title: "连接监控 [2]",
		bindings: []keybinding{
			{"↑/↓  k/j", "选择连接"},
			{"Enter", "查看连接详情"},
			{"x", "关闭选中连接"},
			{"X", "关闭所有连接"},
			{"/", "搜索过滤"},
			{"h", "切换流量/活跃/历史视图"},
			{"s / S", "测速选中/全部站点"},
			{"Esc", "清除过滤 / 返回"},
		},
	},
	{
		title: "日志 [3]",
		bindings: []keybinding{
			{"↑/↓  k/j", "选择日志"},
			{"Enter", "查看日志详情"},
			{"[ / ]", "降低/提升日志级别"},
			{"/", "搜索过滤"},
			{"c", "清空日志"},
			{"Esc", "清除搜索"},
		},
	},
	{
		title: "规则 [4]",
		bindings: []keybinding{
			{"↑/↓  k/j", "选择规则"},
			{"/", "搜索过滤"},
			{"t", "类型筛选"},
			{"Esc", "清除搜索 / 关闭筛选"},
		},
	},
	{
		title: "设置 [5]",
		bindings: []keybinding{
			{"↑/↓", "选择配置项"},
			{"Enter / 双击", "编辑配置项"},
			{"←/→ / Tab", "切换选项（语言）"},
			{"s / Enter", "保存修改"},
			{"Esc", "取消编辑"},
		},
	},
	{
		title: "延迟颜色",
		bindings: []keybinding{
			{"绿色 ●", "< 100ms"},
			{"黄色 ●", "100 – 300ms"},
			{"红色 ●", "> 300ms"},
		},
	},
}

// 颜色常量 — Tokyo Night
var (
	colorBorder  = lipgloss.Color("#7aa2f7") // blue
	colorTitle   = lipgloss.Color("#c0caf5") // foreground
	colorSection = lipgloss.Color("#7dcfff") // cyan
	colorKey     = lipgloss.Color("#e0af68") // yellow
	colorDesc    = lipgloss.Color("#a9b1d6") // comment
	colorDim     = lipgloss.Color("#565f89") // dark5
	colorGreen   = lipgloss.Color("#9ece6a")
	colorYellow  = lipgloss.Color("#e0af68")
	colorRed     = lipgloss.Color("#f7768e")
)

// OverlayHelpPopup 将帮助弹窗居中叠加在 base 页面之上。
//
// 步骤：
//  1. 对 base 每行整体套 Faint，让背景内容降亮但不消失
//  2. 渲染弹窗本体，并计算居中偏移量
//  3. 逐行用 ansi.Cut 截取底层的左侧和右侧，将弹窗内容嵌入中间
func OverlayHelpPopup(base string, width, height int) string {
	// ── 1. 暗化底层 ──
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	if len(baseLines) > height {
		baseLines = baseLines[:height]
	}

	faint := lipgloss.NewStyle().Faint(true)
	dimmed := make([]string, height)
	for i, l := range baseLines {
		dimmed[i] = faint.Render(l)
	}

	// ── 2. 弹窗居中计算 ──
	popup := renderPopup(width, height)
	popupLines := strings.Split(popup, "\n")
	popupHeight := len(popupLines)
	if popupHeight == 0 {
		return strings.Join(dimmed, "\n")
	}

	popupWidth := lipgloss.Width(popupLines[0])
	leftOffset := (width - popupWidth) / 2
	if leftOffset < 0 {
		leftOffset = 0
	}
	topOffset := (height - popupHeight) / 2
	if topOffset < 0 {
		topOffset = 0
	}

	// ── 3. 弹窗行嵌入暗化底层 ──
	for i, pl := range popupLines {
		y := topOffset + i
		if y >= height {
			break
		}
		
		leftPart := ansi.Cut(dimmed[y], 0, leftOffset)
		leftW := lipgloss.Width(leftPart)
		if leftW < leftOffset {
			leftPart += strings.Repeat(" ", leftOffset-leftW)
		}

		rightPart := ansi.Cut(dimmed[y], leftOffset+popupWidth, width)
		dimmed[y] = leftPart + pl + rightPart
	}

	return strings.Join(dimmed, "\n")
}

// renderPopup 渲染帮助弹窗（无背景色，使用终端默认背景）
func renderPopup(termWidth, termHeight int) string {
	// ── 弹窗尺寸 ──
	popupWidth := termWidth * 88 / 100
	if popupWidth > 112 {
		popupWidth = 112
	}
	if popupWidth < 60 {
		popupWidth = 60
	}

	popupHeight := termHeight * 85 / 100
	if popupHeight > 42 {
		popupHeight = 42
	}
	if popupHeight < 20 {
		popupHeight = 20
	}

	// 内容区宽度 = 弹窗宽度 - 边框(2) - 内边距(左右各2=4)
	innerWidth := popupWidth - 6
	if innerWidth < 30 {
		innerWidth = 30
	}

	// 列数
	cols := 3
	if innerWidth < 80 {
		cols = 2
	}
	if innerWidth < 48 {
		cols = 1
	}

	// ── 标题行 ──
	title := lipgloss.NewStyle().Bold(true).Foreground(colorTitle).Render("Mihosh 快捷键帮助")
	closeHint := lipgloss.NewStyle().Foreground(colorDim).Render("? / Esc / q  关闭")
	gap := innerWidth - lipgloss.Width(title) - lipgloss.Width(closeHint)
	if gap < 1 {
		gap = 1
	}
	titleLine := title + strings.Repeat(" ", gap) + closeHint

	divider := lipgloss.NewStyle().Foreground(colorDim).Render(strings.Repeat("─", innerWidth))

	// ── 各分区卡片 ──
	colW := innerWidth/cols - 1
	cards := make([]string, 0, len(helpSections))
	for _, sec := range helpSections {
		cards = append(cards, renderSection(sec, colW))
	}

	var colContents []string
	cardsPerCol := (len(cards) + cols - 1) / cols
	for c := 0; c < cols; c++ {
		start := c * cardsPerCol
		end := start + cardsPerCol
		if end > len(cards) {
			end = len(cards)
		}
		if start >= len(cards) {
			break
		}
		colContents = append(colContents, lipgloss.JoinVertical(lipgloss.Left, cards[start:end]...))
	}

	var body string
	if len(colContents) == 1 {
		body = colContents[0]
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, colContents...)
	}

	// ── 组装并限高 ──
	content := lipgloss.JoinVertical(lipgloss.Left, titleLine, divider, "", body)

	contentLines := strings.Split(content, "\n")
	maxLines := popupHeight - 4 // 边框2 + 内边距上下各1
	if maxLines < 4 {
		maxLines = 4
	}
	if len(contentLines) > maxLines {
		contentLines = contentLines[:maxLines]
		contentLines = append(contentLines,
			lipgloss.NewStyle().Foreground(colorDim).Render("↓ 更多内容..."),
		)
	}
	for len(contentLines) < maxLines {
		contentLines = append(contentLines, "")
	}
	content = strings.Join(contentLines, "\n")

	// ── 弹窗外壳：圆角边框，无背景色 ──
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2).
		Width(popupWidth)

	return popupStyle.Render(content)
}

// renderSection 渲染单个快捷键分区（无背景色）
func renderSection(sec section, width int) string {
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(colorSection)

	keyWidth := 16
	if width < 38 {
		keyWidth = 12
	}
	keyStyle := lipgloss.NewStyle().Foreground(colorKey).Width(keyWidth)
	descStyle := lipgloss.NewStyle().Foreground(colorDesc)

	var lines []string
	lines = append(lines, sectionStyle.Render(sec.title))

	for _, b := range sec.bindings {
		k := b.key
		desc := b.desc

		if sec.title == "延迟颜色" {
			var dot string
			switch {
			case strings.Contains(k, "绿"):
				dot = lipgloss.NewStyle().Foreground(colorGreen).Render("●")
			case strings.Contains(k, "黄"):
				dot = lipgloss.NewStyle().Foreground(colorYellow).Render("●")
			case strings.Contains(k, "红"):
				dot = lipgloss.NewStyle().Foreground(colorRed).Render("●")
			default:
				dot = "●"
			}
			lines = append(lines, "  "+dot+" "+descStyle.Render(desc))
			continue
		}

		lines = append(lines, "  "+keyStyle.Render(k)+descStyle.Render(desc))
	}

	lines = append(lines, "") // 分区间距
	return strings.Join(lines, "\n")
}

// RenderHelpPage 旧接口，保留供可能存在的引用
func RenderHelpPage(width, height int) string {
	return renderPopup(width, height)
}
