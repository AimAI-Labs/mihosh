package nodes

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// PageState 节点页面状态（由 Model 传入）
type PageState struct {
	Mode             string
	Layout           NodesLayout
	Groups           map[string]model.Group
	Proxies          map[string]model.Proxy
	GroupNames       []string
	SelectedGroup    int
	SelectedProxy    int
	GroupScrollTop   int
	ProxyScrollTop   int
	CurrentProxies   []string
	Testing          bool
	TestingTarget    string
	TestResults      []TestResultEntry
	ShowTestDetail   bool     // 是否显示测速结果弹窗
	DetailScrollTop  int      // 测速结果弹窗滚动偏移
	SortOrderLabels  []string // 排序选项文本
	CurrentSortOrder int      // 当前排序模式
	Width            int
	Height           int    // 终端高度
	FilterText       string // 节点搜索关键词
	FilterMode       bool   // 是否处于搜索输入模式
	FilterEngine     FilterEngine
}

// displayWidth 委托给 common.DisplayWidth
func displayWidth(s string) int {
	return common.DisplayWidth(s)
}

// RenderNodesPage 渲染节点管理页面
func RenderNodesPage(state PageState) string {
	modeSwitch := RenderModeSwitchComponent(state.Mode, state.Width, state.Layout)

	// ── 底部固定行：搜索提示 + 失败徽标 ──────────────────────────────
	// 搜索行始终占一行（空行或内容行），确保其位置固定在底栏上方，
	// 不随卡片内容多少而上下漂移。
	var searchLine string
	engineIndicator := common.RenderSearchEngineBadge(state.FilterEngine)

	if state.FilterMode {
		searchLine = common.TableHeaderStyle().Render(i18n.Tf("nodes.search_active", state.FilterText)) + engineIndicator
	} else if state.FilterText != "" {
		searchLine = common.MutedStyle().Render(i18n.Tf("nodes.search_inactive", state.FilterText)) + engineIndicator
	} else {
		searchLine = "" // 占位空行由 clampPanelArea 负责填充
	}

	var resultBadge string
	if len(state.TestResults) > 0 {
		failCount := 0
		for _, r := range state.TestResults {
			if r.Error != "" {
				failCount++
			}
		}
		badgeText := i18n.Tf("nodes.result_badge", len(state.TestResults))
		if failCount > 0 {
			badgeText += " " + common.ErrorStyle().Render(i18n.Tf("nodes.result_fail_count", failCount))
		}
		resultBadge = common.MutedStyle().Render(badgeText) +
			" " + common.MutedStyle().Render(i18n.T("nodes.view_detail"))
	}

	// 主体区域：按所选布局渲染卡片流或经典双列列表。
	areaHeight := cardAreaHeight(state)
	var bodyArea string
	if state.Layout == LayoutColumns {
		bodyArea = clampToLines(renderColumnsArea(state, areaHeight), areaHeight)
	} else {
		bodyArea = clampToLines(renderCardArea(state, areaHeight), areaHeight)
	}

	// ── 拼接：模式切换 + 主体区 + 搜索行 + 结果徽标 ──────────────
	bottomParts := []string{searchLine}
	if resultBadge != "" {
		bottomParts = append(bottomParts, resultBadge)
	}
	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		append([]string{modeSwitch, "", bodyArea}, bottomParts...)...,
	)

	mainContent = renderNodesInlineHelp(mainContent, state)
	if state.ShowTestDetail {
		modal := buildTestResultModal(state)
		mainContent = dialog.New(modal, state.Width, state.Height).Clipped().Overlay(mainContent, true)
	}
	return mainContent
}

// clampToLines 将字符串精确约束为 h 行：不足时末尾补空行，超出时截断。
func clampToLines(content string, h int) string {
	if h <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) < h {
		for len(lines) < h {
			lines = append(lines, "")
		}
	} else if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

// buildTestResultModal 构建测速结果详情弹窗字符串
func buildTestResultModal(state PageState) string {
	results := state.TestResults

	// 弹窗内容区宽度（去掉左右边框各1 + 内边距各1 = 4）
	modalWidth := min(state.Width, max(24, min(100, state.Width-10)))
	innerWidth := modalWidth - 4

	// 可显示的最大行数（去掉标题、分隔线、空行、帮助行 = 4行）
	modalHeight := state.Height - 8
	if modalHeight < 6 {
		modalHeight = 6
	}
	maxDisplay := modalHeight - 4
	if maxDisplay < 1 {
		maxDisplay = 1
	}

	allLines := buildTestResultDetailLines(results, innerWidth)
	if len(allLines) == 0 {
		allLines = []string{i18n.T("nodes.empty_results")}
	}
	totalLines := len(allLines)

	// 限制滚动范围
	scrollTop := state.DetailScrollTop
	if scrollTop > totalLines-maxDisplay {
		scrollTop = totalLines - maxDisplay
	}
	if scrollTop < 0 {
		scrollTop = 0
	}
	endIdx := scrollTop + maxDisplay
	if endIdx > totalLines {
		endIdx = totalLines
	}

	// 构建内容行
	var bodyLines []string
	if scrollTop > 0 {
		bodyLines = append(bodyLines, common.DimStyle().Render(i18n.Tf("nodes.result_scroll_up", scrollTop)))
	}
	bodyLines = append(bodyLines, allLines[scrollTop:endIdx]...)
	if endIdx < totalLines {
		bodyLines = append(bodyLines, common.DimStyle().Render(i18n.Tf("nodes.result_scroll_down", totalLines-endIdx)))
	}
	body := strings.Join(bodyLines, "\n")

	// 标题：显示总数和失败数
	failCount := 0
	for _, r := range results {
		if r.Error != "" {
			failCount++
		}
	}
	title := i18n.Tf("nodes.result_modal_title", len(results))
	if failCount > 0 {
		title += " " + common.ErrorStyle().Render(i18n.Tf("nodes.result_fail_count", failCount))
	}
	subtitle := common.DimStyle().Render(i18n.T("nodes.result_modal_subtitle"))
	separator := common.DimStyle().Render(strings.Repeat("─", max(1, innerWidth)))
	content := lipgloss.JoinVertical(lipgloss.Left, subtitle, separator, body)
	return common.RenderBorderedPanel(title, content, modalWidth, common.TokyoBlue(), common.TokyoForeground())
}

func buildTestResultDetailLines(results []TestResultEntry, width int) []string {
	if width < 20 {
		width = 20
	}

	lines := make([]string, 0, len(results)*4)
	for i, entry := range results {
		if entry.Error != "" {
			// 失败条目
			lines = append(lines, common.ErrorStyle().Render(fmt.Sprintf("[%02d] %s", i+1, entry.Name)))
			rawMsg := entry.Error
			if entry.TestURL != "" {
				rawMsg = fmt.Sprintf("[%s] %s", entry.TestURL, entry.Error)
			}
			lines = append(lines, wrapWithPrefix(i18n.T("nodes.reason_prefix"), rawMsg, width)...)
		} else {
			// 成功条目
			lines = append(lines, fmt.Sprintf("[%02d] %s", i+1, entry.Name))
			delayText := fmt.Sprintf("%dms", entry.Delay)
			lines = append(lines, wrapWithPrefix(i18n.T("nodes.delay_prefix"), delayText, width)...)
			if entry.TestURL != "" {
				lines = append(lines, wrapWithPrefix(i18n.T("nodes.raw_prefix"), entry.TestURL, width)...)
			}
		}
		if i < len(results)-1 {
			lines = append(lines, "")
		}
	}
	return lines
}

func wrapWithPrefix(prefix, text string, width int) []string {
	prefixWidth := displayWidth(prefix)
	if width <= prefixWidth {
		width = prefixWidth + 1
	}

	parts := wrapByDisplayWidth(text, width-prefixWidth)
	if len(parts) == 0 {
		return []string{prefix}
	}

	indent := strings.Repeat(" ", prefixWidth)
	lines := make([]string, 0, len(parts))
	for i, line := range parts {
		if i == 0 {
			lines = append(lines, prefix+line)
			continue
		}
		lines = append(lines, indent+line)
	}
	return lines
}

func wrapByDisplayWidth(text string, width int) []string {
	if width < 1 {
		width = 1
	}

	var (
		lines []string
		sb    strings.Builder
		w     int
	)

	for _, r := range text {
		if r == '\n' {
			lines = append(lines, sb.String())
			sb.Reset()
			w = 0
			continue
		}

		rw := runewidth.RuneWidth(r)
		if rw < 0 {
			rw = 0
		}
		if w > 0 && w+rw > width {
			lines = append(lines, sb.String())
			sb.Reset()
			w = 0
		}

		sb.WriteRune(r)
		w += rw
	}

	if sb.Len() > 0 || len(lines) == 0 {
		lines = append(lines, sb.String())
	}

	return lines
}
