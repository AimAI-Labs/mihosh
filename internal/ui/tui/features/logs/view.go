package logs

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

const (
	logsHeaderLines      = logsModeSwitchHeight + 5
	logsFixedLines       = logsHeaderLines + 2
	logsMinHeight        = 5
	logsDefaultPadding   = 20
	logsLevelWidth       = 8
	logsModeSwitchHeight = 3 // 级别栏边框高度
)

// 日志级别列表
var logLevels = []string{"debug", "info", "warning", "error", "silent"}

// logLevelColors 返回日志级别颜色映射（动态构建，支持主题热切换）
func logLevelColors() map[string]lipgloss.Color {
	return map[string]lipgloss.Color{
		"debug":   common.TokyoMuted(),
		"info":    common.TokyoBlue(),
		"warning": common.TokyoYellow(),
		"error":   common.TokyoRed(),
		"silent":  common.TokyoPurple(),
	}
}

type logLevelTabStyle struct {
	Background lipgloss.Color
	Foreground lipgloss.Color
	Indicator  lipgloss.Color
	Bold       bool
}

// PageState 日志页面状态
type PageState struct {
	Logs               []model.LogEntry // 日志列表
	FilteredLogIndices []int            // 过滤后的日志索引
	LogLevel           int              // 当前级别索引
	FilterText         string           // 搜索关键词
	FilterMode         bool             // 是否处于过滤输入模式
	SelectedLog        int              // 选中的日志索引
	ScrollTop          int              // 滚动偏移
	HScrollOffset      int              // 水平滚动偏移
	Width              int              // 页面宽度
	Height             int              // 页面高度

	// 详情弹窗
	DetailMode          bool
	DetailLog           *model.LogEntry
	DetailParsed        *ParsedLog
	DetailResolved      *model.ResolvedIP
	DetailSourcePrivate bool
	DetailScroll        int
	Paused              bool
	PausedNewCount      int
	DroppedLogs         uint64
}

func RenderLogsPage(state PageState) string {
	var sections []string

	// 渲染日志级别标签栏
	levelBar := renderLevelBar(state.LogLevel, state.Width, state.Paused, state.PausedNewCount)
	sections = append(sections, levelBar)
	sections = append(sections, "")

	// 渲染搜索框
	searchBox := renderLogSearchBox(state.FilterText, state.FilterMode)
	sections = append(sections, searchBox)
	sections = append(sections, "")

	// 过滤日志 (使用缓存的索引)
	var filteredLogs []model.LogEntry
	for _, idx := range state.FilteredLogIndices {
		if idx >= 0 && idx < len(state.Logs) {
			filteredLogs = append(filteredLogs, state.Logs[idx])
		}
	}

	// 渲染统计信息
	stats := common.MutedStyle().Render(i18n.Tf("logs.stats", len(filteredLogs), logLevels[state.LogLevel]))
	if state.DroppedLogs != 0 {
		dropped := lipgloss.NewStyle().Foreground(theme.Resolve(theme.Current().Warning)).
			Render(i18n.Tf("logs.dropped", state.DroppedLogs))
		stats = dropped + "  " + stats
	}
	if state.Paused {
		badgeText := i18n.T("logs.paused_badge")
		if state.PausedNewCount > 0 {
			badgeText = i18n.Tf("logs.paused_badge_new", state.PausedNewCount)
		}
		badge := lipgloss.NewStyle().
			Foreground(theme.Resolve(theme.Current().Background)).
			Background(theme.Resolve(theme.Current().Warning)).
			Bold(true).
			Render(" " + common.SymbolPause + " " + badgeText + " ")
		stats += "  " + badge
	}
	sections = append(sections, stats)
	sections = append(sections, "")

	// 计算可显示的日志行数 (级别栏 + 搜索框 + 统计 + 间隔)
	availableHeight := logListHeight(state.Height)

	// 渲染日志列表
	logList := renderLogList(filteredLogs, state.SelectedLog, state.ScrollTop, availableHeight, state.Width, state.HScrollOffset)
	sections = append(sections, logList)

	base := strings.Join(sections, "\n")

	// 详情模式：渲染日志详情弹窗
	if state.DetailMode && state.DetailLog != nil {
		return OverlayLogDetailPopup(
			base,
			state.DetailLog,
			state.DetailParsed,
			state.DetailResolved,
			state.DetailSourcePrivate,
			state.Width,
			state.Height,
			state.DetailScroll,
		)
	}

	return base

}

// ============================================================
//  内联帮助提示面板（右下角浮层）
// ============================================================
//
// 渲染逻辑（InlineHelpHint / FormatInlineHintRow / OverlayHelpAtBottomRight）
// 共享自 components/common。

func (s State) InlineHelpHints() []common.InlineHelpHint {
	// 详情模式：滚动 + 关闭
	if s.detailMode {
		return []common.InlineHelpHint{
			{Key: "↑/↓", Desc: i18n.T("help.logs_detail.scroll")},
			{Key: "Esc/q", Desc: i18n.T("help.logs_detail.close")},
		}
	}

	// 过滤输入模式：确认 + 取消 + 删除
	if s.logFilterMode {
		return []common.InlineHelpHint{
			{Key: "↵", Desc: i18n.T("help.logs_search.confirm")},
			{Key: "Esc", Desc: i18n.T("help.logs_search.cancel")},
			{Key: "⌫", Desc: i18n.T("help.logs_search.backspace")},
		}
	}

	// 普通模式：核心操作
	pauseHint := common.InlineHelpHint{Key: "p", Desc: i18n.T("help.logs.hint_pause")}
	if s.paused {
		pauseHint = common.InlineHelpHint{Key: "p", Desc: i18n.T("help.logs.hint_resume"), Tone: common.InlineHelpToneWarning}
	}
	return []common.InlineHelpHint{
		{Key: "↑↓", Desc: i18n.T("help.logs.hint_select")},
		{Key: "↵", Desc: i18n.T("help.logs.hint_detail")},
		{Key: "[/]", Desc: i18n.T("help.logs.hint_level")},
		{Key: "←→", Desc: i18n.T("help.logs.hint_scroll")},
		pauseHint,
		{Key: "/", Desc: i18n.T("help.logs.hint_search")},
		{Key: "c", Desc: i18n.T("help.logs.hint_clear"), Tone: common.InlineHelpToneDanger},
		{Key: "?", Desc: i18n.T("core.inline.help")},
	}
}

func logPauseLabel(paused bool, pausedNewCount int) string {
	if paused {
		if pausedNewCount > 0 {
			return " " + common.SymbolPause + " " + i18n.Tf("logs.paused_badge_new", pausedNewCount) + " "
		}
		return " " + common.SymbolPause + " " + i18n.T("logs.paused_badge") + " "
	}
	return " " + common.SymbolPause + " " + i18n.T("logs.pause_btn") + " "
}

func renderLogPausePill(paused bool, pausedNewCount int) string {
	label := logPauseLabel(paused, pausedNewCount)
	if paused {
		return lipgloss.NewStyle().
			Background(theme.Resolve(theme.Current().Warning)).
			Foreground(theme.Resolve(theme.Current().Background)).
			Bold(true).
			Render(label)
	}
	return lipgloss.NewStyle().
		Foreground(common.TokyoMuted()).
		Render(label)
}

// renderLevelBar 渲染日志级别标签栏（带边框，右侧含暂停/恢复状态按钮）
func renderLevelBar(selectedLevel int, width int, paused bool, pausedNewCount int) string {
	// 渲染原始的级别标签
	var tabs []string
	for i, level := range logLevels {
		tabs = append(tabs, renderLogLevelTab(level, i == selectedLevel))
	}
	content := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	// 计算内边框宽度
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	pausePill := renderLogPausePill(paused, pausedNewCount)
	pauseWidth := lipgloss.Width(pausePill)

	// 填充内容到指定宽度（空间足够时右侧展示暂停/恢复按钮）
	contentWidth := lipgloss.Width(content)
	if contentWidth+1+pauseWidth <= innerWidth {
		content += strings.Repeat(" ", innerWidth-contentWidth-pauseWidth) + pausePill
	} else if contentWidth < innerWidth {
		content += strings.Repeat(" ", innerWidth-contentWidth)
	} else {
		content = ansi.Truncate(content, innerWidth, "")
	}

	// 渲染带边框的级别栏
	borderStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue())
	topLine := borderStyle.Render(theme.Frame().TopLeft + strings.Repeat("─", innerWidth) + theme.Frame().TopRight)
	middleLine := borderStyle.Render("│") + content + borderStyle.Render("│")
	bottomLine := borderStyle.Render(theme.Frame().BottomLeft + strings.Repeat("─", innerWidth) + theme.Frame().BottomRight)

	return topLine + "\n" + middleLine + "\n" + bottomLine
}

func renderLogLevelTab(level string, active bool) string {
	style := resolveLogLevelTabStyle(level, active)
	indicator := lipgloss.NewStyle().Foreground(style.Indicator)
	label := lipgloss.NewStyle().Foreground(style.Foreground).Bold(style.Bold)
	if active {
		indicator = indicator.Background(style.Background)
		label = label.Background(style.Background)
	}
	return indicator.Render(" ●") + label.Render(" "+level+" ")
}

func resolveLogLevelTabStyle(level string, active bool) logLevelTabStyle {
	style := logLevelTabStyle{
		Background: common.TokyoSelected(),
		Foreground: common.TokyoMuted(),
		Indicator:  logLevelColors()[level],
	}
	if active {
		style.Background = common.TokyoSelected()
		style.Foreground = common.TokyoCyan()
		style.Bold = true
		if style.Indicator == style.Background {
			style.Indicator = common.TokyoCyan()
		}
	}
	return style
}

// ClickedPause 判断是否点击了级别栏右侧的暂停/恢复按钮
func ClickedPause(pageX, pageWidth, selectedLevel int, paused bool, pausedNewCount int) bool {
	innerWidth := pageWidth - 2
	if innerWidth < 1 || pageX < 1 || pageX >= pageWidth-1 {
		return false
	}
	leftWidth := 0
	for i, level := range logLevels {
		leftWidth += lipgloss.Width(renderLogLevelTab(level, i == selectedLevel))
	}
	pauseWidth := lipgloss.Width(logPauseLabel(paused, pausedNewCount))
	if leftWidth+1+pauseWidth > innerWidth {
		return false
	}
	startX := 1 + innerWidth - pauseWidth
	return pageX >= startX && pageX < startX+pauseWidth
}

// ClickedLevel 根据点击的 X 坐标返回对应的级别索引，未点击级别栏返回 -1
func ClickedLevel(pageX int, pageWidth int, selectedLevel int) int {
	if pageX < 1 || pageX >= pageWidth-1 {
		return -1
	}
	end := 1 // 左边框占一列。
	for i, level := range logLevels {
		end += lipgloss.Width(renderLogLevelTab(level, i == selectedLevel))
		if pageX < end {
			return i
		}
	}
	return -1
}

// renderLogSearchBox 渲染搜索框
func renderLogSearchBox(filterText string, filterMode bool) string {
	if filterMode {
		inputStyle := lipgloss.NewStyle().Foreground(common.TokyoPanel()).Background(common.TokyoCyan())
		label := common.TokyoMutedStyle().Render(i18n.T("logs.search"))
		input := inputStyle.Render(filterText + "█")
		return label + input
	}

	label := common.TokyoMutedStyle().Render(i18n.T("logs.search"))
	input := lipgloss.NewStyle().Foreground(common.TokyoForeground()).Render(filterText)
	return label + input
}

func logListHeight(pageHeight int) int {
	return max(logsMinHeight, pageHeight-logsFixedLines)
}

// renderLogList 渲染日志列表
func renderLogList(logs []model.LogEntry, selectedIdx, scrollTop, maxLines, width, hOffset int) string {
	if len(logs) == 0 {
		return common.MutedStyle().Render(i18n.T("logs.empty"))
	}

	var lines []string

	window := common.ResolveListWindow(len(logs), selectedIdx, scrollTop, maxLines)

	for i := window.Start; i < window.End; i++ {
		log := logs[i]
		line := RenderLogEntry(log, i == window.Selected, width, hOffset)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// RenderLogEntry 渲染单条日志
func RenderLogEntry(log model.LogEntry, selected bool, maxWidth int, hOffset int) string {
	color := logLevelColors()[log.Type]
	if color == "" {
		color = common.TokyoMuted()
	}

	levelStyle := lipgloss.NewStyle().
		Foreground(color).
		Width(logsLevelWidth)

	timeStr := log.Timestamp.Format("15:04:05")
	timePart := common.DimStyle().Render(timeStr)

	contentStyle := lipgloss.NewStyle().Foreground(common.TokyoMuted())

	content := log.Payload
	if decoded, err := url.QueryUnescape(content); err == nil {
		content = decoded
	}

	// 符号 (2) + 时间 (8) + 空格 (1) + 级别 (logsLevelWidth) + 空格 (1) = 20
	prefixWidth := 2 + len(timeStr) + 1 + logsLevelWidth + 1
	usableWidth := maxWidth - prefixWidth
	if usableWidth < 1 {
		usableWidth = 1
	}

	if hOffset > 0 {
		var b strings.Builder
		w := 0
		for _, r := range content {
			rw := runewidth.RuneWidth(r)
			if w+rw <= hOffset {
				w += rw
				continue
			}
			w += rw
			b.WriteRune(r)
		}
		content = b.String()
	}

	content = runewidth.Truncate(content, usableWidth, "")

	line := fmt.Sprintf("%s %s %s",
		timePart,
		levelStyle.Render(strings.ToUpper(log.Type)),
		contentStyle.Render(content),
	)

	if selected {
		line = common.SymbolSelectActive + line
		line = lipgloss.NewStyle().
			Background(common.TokyoSelected()).
			Width(maxWidth).
			Render(line)
	} else {
		line = common.SymbolSelectInactive + line
		line = lipgloss.NewStyle().
			Width(maxWidth).
			Render(line)
	}

	return line
}
