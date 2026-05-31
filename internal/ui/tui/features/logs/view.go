package logs

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/charmbracelet/lipgloss"
)

const (
	logsFixedLines     = 10 // 级别栏(3) + 间距(1) + 搜索框(1) + 间距(1) + 统计(1) + 间隔(1) + 底部(2)
	logsMinHeight      = 5
	logsDefaultPadding = 20
	logsLevelWidth     = 8
	logsModeSwitchHeight = 3 // 级别栏边框高度
)

// 日志级别列表
var logLevels = []string{"debug", "info", "warning", "error", "silent"}

// 日志级别颜色
var logLevelColors = map[string]lipgloss.Color{
	"debug":   common.CMuted,
	"info":    common.CSecondary,
	"warning": common.CWarning,
	"error":   common.CDanger,
	"silent":  common.CPurple,
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
}

// RenderLogsPage 渲染日志页面
func RenderLogsPage(state PageState) string {
	// 详情模式：渲染日志详情弹窗
	if state.DetailMode && state.DetailLog != nil {
		return renderLogDetailModal(
			state.DetailLog,
			state.DetailParsed,
			state.DetailResolved,
			state.DetailSourcePrivate,
			state.Width,
			state.Height,
			state.DetailScroll,
		)
	}

	var sections []string

	// 渲染日志级别标签栏
	levelBar := renderLevelBar(state.LogLevel, state.Width)
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
	stats := fmt.Sprintf("共 %d 条日志 (级别: %s)", len(filteredLogs), logLevels[state.LogLevel])
	sections = append(sections, common.MutedStyle.Render(stats))
	sections = append(sections, "")

	// 计算可显示的日志行数 (级别栏 + 搜索框 + 统计 + 间隔)
	availableHeight := state.Height - logsFixedLines
	if availableHeight < logsMinHeight {
		availableHeight = logsMinHeight
	}

	// 渲染日志列表
	logList := renderLogList(filteredLogs, state.SelectedLog, state.ScrollTop, availableHeight, state.Width, state.HScrollOffset)
	sections = append(sections, logList)

	// 统一底部的提示信息
	helpText := "[↑/↓]选择 [Enter]详情 [{/}]级别 [/]搜索 [c]清空 [Esc]清除搜索 [r]刷新"
	mainContent := strings.Join(sections, "\n")
	contentLines := strings.Count(mainContent, "\n") + 1

	footer := common.RenderFooter(state.Width, state.Height, contentLines, helpText)
	return mainContent + footer
}

// renderLevelBar 渲染日志级别标签栏（带边框）
func renderLevelBar(selectedLevel int, width int) string {
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

	// 填充内容到指定宽度
	contentWidth := lipgloss.Width(content)
	if contentWidth < innerWidth {
		content += strings.Repeat(" ", innerWidth-contentWidth)
	}

	// 渲染带边框的级别栏
	borderStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue)
	topLine := borderStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")
	middleLine := borderStyle.Render("│") + content + borderStyle.Render("│")
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")

	return topLine + "\n" + middleLine + "\n" + bottomLine
}

func renderLogLevelTab(level string, active bool) string {
	style := resolveLogLevelTabStyle(level, active)

	if active {
		// 激活状态：只有文字部分有背景
		indicator := lipgloss.NewStyle().Foreground(style.Indicator).Render("●")
		text := lipgloss.NewStyle().
			Background(style.Background).
			Foreground(style.Foreground).
			Bold(style.Bold).
			Render(" " + level + " ")
		return indicator + text
	}

	// 非激活状态：无背景
	indicator := lipgloss.NewStyle().Foreground(style.Indicator).Render("●")
	text := lipgloss.NewStyle().
		Foreground(style.Foreground).
		Render(" " + level + " ")
	return indicator + text
}

func resolveLogLevelTabStyle(level string, active bool) logLevelTabStyle {
	style := logLevelTabStyle{
		Background: common.TokyoSelected,
		Foreground: common.TokyoMuted,
		Indicator:  logLevelColors[level],
	}
	if active {
		style.Background = common.TokyoSelected
		style.Foreground = common.TokyoCyan
		style.Bold = true
		if style.Indicator == style.Background {
			style.Indicator = common.TokyoCyan
		}
	}
	return style
}

// LevelBarPositions 返回每个级别标签的起始位置（用于鼠标点击检测）
func LevelBarPositions(pageWidth int) []int {
	positions := make([]int, len(logLevels))
	currentPos := 0

	// 使用实际渲染宽度计算（考虑 lipgloss 样式）
	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(common.TokyoBlue)

	for i, level := range logLevels {
		positions[i] = currentPos
		tag := activeStyle.Render(" ● " + level + " ")
		currentPos += lipgloss.Width(tag)
		// 级别之间没有空格分隔（因为 tab 自身有背景）
	}

	return positions
}

// ClickedLevel 根据点击的 X 坐标返回对应的级别索引，未点击级别栏返回 -1
func ClickedLevel(pageX int, pageWidth int, selectedLevel int) int {
	positions := LevelBarPositions(pageWidth)

	// 计算每个标签的结束位置
	for i := 0; i < len(positions); i++ {
		var endPos int
		if i < len(positions)-1 {
			endPos = positions[i+1] - 1 // 下一个标签开始位置减 1（减去间隔空格）
		} else {
			endPos = pageWidth
		}

		if pageX >= positions[i] && pageX < endPos {
			return i
		}
	}

	return -1
}

// renderLogSearchBox 渲染搜索框
func renderLogSearchBox(filterText string, filterMode bool) string {
	if filterMode {
		inputStyle := lipgloss.NewStyle().Foreground(common.CWhite).Background(common.CHighlight)
		label := common.MutedStyle.Render("搜索: ")
		input := inputStyle.Render(filterText + "█")
		return label + input
	}

	label := common.MutedStyle.Render("搜索: ")
	input := lipgloss.NewStyle().Foreground(common.CWhite).Render(filterText)
	return label + input
}

// getLevelIndex 获取日志级别索引
func getLevelIndex(level string) int {
	for i, l := range logLevels {
		if l == level {
			return i
		}
	}
	return 1 // 默认info
}

// renderLogList 渲染日志列表
func renderLogList(logs []model.LogEntry, selectedIdx, scrollTop, maxLines, width, hOffset int) string {
	if len(logs) == 0 {
		return common.MutedStyle.Render("暂无日志")
	}

	var lines []string
	maxWidth := width - 20 // 预留边距

	// 调整滚动位置确保选中项可见
	if selectedIdx < scrollTop {
		scrollTop = selectedIdx
	}
	if selectedIdx >= scrollTop+maxLines {
		scrollTop = selectedIdx - maxLines + 1
	}

	endIdx := scrollTop + maxLines
	if endIdx > len(logs) {
		endIdx = len(logs)
	}

	for i := scrollTop; i < endIdx; i++ {
		log := logs[i]
		line := renderLogEntry(log, i == selectedIdx, maxWidth, hOffset)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderLogEntry 渲染单条日志
func renderLogEntry(log model.LogEntry, selected bool, maxWidth int, hOffset int) string {
	color := logLevelColors[log.Type]
	if color == "" {
		color = common.CMuted
	}

	levelStyle := lipgloss.NewStyle().
		Foreground(color).
		Width(logsLevelWidth)

	timeStr := log.Timestamp.Format("15:04:05")
	timePart := common.DimStyle.Render(timeStr)

	contentStyle := lipgloss.NewStyle().Foreground(common.CMuted)

	content := log.Payload
	if decoded, err := url.QueryUnescape(content); err == nil {
		content = decoded
	}

	usableWidth := maxWidth - logsDefaultPadding
	if usableWidth < 1 {
		usableWidth = 1
	}

	if hOffset > 0 {
		contentWidth := 0
		for i, r := range content {
			if r > 127 {
				contentWidth += 2
			} else {
				contentWidth++
			}
			if contentWidth > hOffset {
				content = content[i:]
				break
			}
		}
		if contentWidth <= hOffset {
			content = ""
		}
	}

	displayWidth := usableWidth
	if len(content) > displayWidth {
		content = content[:displayWidth]
	}

	line := fmt.Sprintf("%s %s %s",
		timePart,
		levelStyle.Render(strings.ToUpper(log.Type)),
		contentStyle.Render(content),
	)

	if selected {
		line = lipgloss.NewStyle().
			Background(common.CHighlight).
			Render(common.SymbolSelectActive + line)
	} else {
		line = common.SymbolSelectInactive + line
	}

	return line
}
