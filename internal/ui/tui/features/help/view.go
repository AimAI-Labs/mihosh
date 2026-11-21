package help

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ── 页面类型常量（与 layout 包保持一致，避免循环依赖）──
const (
	PageNodes       = 0
	PageConnections = 1
	PageLogs        = 2
	PageRules       = 3
	PageSub         = 4
	PageSettings    = 5
	PageCore        = 6
)

// ── 连接页视图模式常量（与 connections 包保持一致）──
const (
	ConnViewTraffic = 0
	ConnViewActive  = 1
	ConnViewHistory = 2
)

// HelpContext 帮助弹窗所需的当前界面上下文
type HelpContext struct {
	CurrentPage      int // 当前页面（PageNodes / PageConnections / ...）
	CoreReleases     bool
	CorePlatform     string
	CoreAsset        string
	CoreReleaseNotes []string
	ScrollOffset     int

	// 连接页子状态
	ConnViewMode int  // ConnViewTraffic / ConnViewActive / ConnViewHistory
	ConnDetail   bool // 连接详情弹窗
	ConnTopN     bool // TopN 弹窗
	ConnFilter   bool // 过滤输入

	// 节点页子状态
	NodesTestDetail bool // 测速结果详情弹窗
	NodesFilterMode bool // 搜索输入
	NodesChainModal bool // 链式代理弹窗

	// 日志页子状态
	LogsDetail bool // 日志详情弹窗
	LogsFilter bool // 过滤输入

	// 规则页子状态
	RulesTypeFilter    bool // 类型筛选栏
	RulesFilter        bool // 过滤输入
	RulesDeleteConfirm bool // 删除确认弹窗
	RulesEditForm      bool // 编辑规则弹窗

	// 订阅页子状态
	SubMode int // sub 页交互模式（见 features/sub SubMode* 常量）

	// 设置页子状态
	SettingsEdit     bool // 编辑模式
	SettingsLanguage bool // 当前选中项是语言
	SettingsTheme    bool // 当前选中项是主题
}

// keybinding 单条快捷键条目
type keybinding struct {
	key  string
	desc string
}

// section 快捷键分组
type section struct {
	title    string
	bindings []keybinding
	notes    []string
}

// buildHelpSections 根据当前上下文动态构建帮助分区
func buildHelpSections(ctx HelpContext) []section {
	var sections []section

	// 1. 全局快捷键（始终显示）
	sections = append(sections, section{
		title: i18n.T("help.section.global"),
		bindings: []keybinding{
			{"?", i18n.T("help.global.toggle")},
			{"Tab / Shift+Tab", i18n.T("help.global.switch")},

			{"r", i18n.T("help.global.refresh")},
			{"q / Ctrl+C", i18n.T("help.global.quit")},
		},
	})

	// 2. 当前页面快捷键
	switch ctx.CurrentPage {
	case PageNodes:
		sections = append(sections, buildNodesSections(ctx)...)
		// 延迟颜色图例仅在节点页显示
		sections = append(sections, section{
			title: i18n.T("help.section.latency_colors"),
			bindings: []keybinding{
				{"green", i18n.T("help.latency.green")},
				{"yellow", i18n.T("help.latency.yellow")},
				{"red", i18n.T("help.latency.red")},
			},
		})
	case PageConnections:
		sections = append(sections, buildConnectionsSections(ctx)...)
	case PageLogs:
		sections = append(sections, buildLogsSections(ctx)...)
	case PageRules:
		sections = append(sections, buildRulesSections(ctx)...)
	case PageSub:
		sections = append(sections, buildSubSections(ctx)...)
	case PageCore:
		if ctx.CoreReleases {
			sections = append(sections, section{title: i18n.T("core.catalog.title"), bindings: []keybinding{
				{"↑↓", i18n.T("core.catalog.help.select")},
				{"Enter / i", i18n.T("core.catalog.help.download")},
				{"a", i18n.T("core.catalog.help.activate")},
				{"PgUp / PgDn", i18n.T("core.catalog.help.pages")},
				{"r / v", i18n.T("core.action.releases")},
				{"e", i18n.T("core.action.network")},
				{"z", i18n.T("core.action.restart")},
				{"t", i18n.T("core.action.version")},
				{"l", i18n.T("core.action.import")},
				{"g / b", i18n.T("core.catalog.help.update")},
				{"1–3", i18n.T("core.help.sections")},
				{"Esc", i18n.T("core.cancel")},
			}})
			details := append([]string{}, ctx.CoreReleaseNotes...)
			details = append(details, i18n.T("core.catalog.platform")+": "+ctx.CorePlatform)
			if ctx.CoreAsset != "" {
				details = append(details, i18n.T("core.catalog.asset")+": "+ctx.CoreAsset)
			}
			sections = append(sections,
				section{title: i18n.T("core.catalog.selection"), notes: details},
				section{title: i18n.T("core.catalog.help.instructions"), notes: []string{
					i18n.T("core.catalog.download_hint"), i18n.T("core.desc.activate"),
					i18n.T("core.catalog.unverified"),
					i18n.T("core.catalog.more_hint"), i18n.T("core.catalog.fallback"),
				}},
			)
			break
		}
		sections = append(sections, section{title: i18n.T("menu.core"), bindings: []keybinding{
			{"↑↓ / Enter", i18n.T("core.help.select")},
			{"←→ / 1–5", i18n.T("core.help.sections")},
			{"p", i18n.T("core.action.mihomo")},
			{"s / x", i18n.T("core.help.runtime")},
			{"2 → e", i18n.T("core.action.network")},
			{"w", i18n.T("core.action.shell-proxy")},
			{"h", i18n.T("core.action.service")},
			{"Esc", i18n.T("core.cancel")},
		}})
	case PageSettings:
		sections = append(sections, buildSettingsSections(ctx)...)
	}

	return sections
}

// buildNodesSections 节点页帮助分区
func buildNodesSections(ctx HelpContext) []section {
	if ctx.NodesChainModal {
		return []section{{
			title: i18n.T("help.section.nodes_chain"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.nodes_chain.navigate")},
				{"↵ / Space", i18n.T("help.nodes_chain.toggle")},
				{"Tab", i18n.T("help.nodes_chain.focus")},
				{"[ / ]", i18n.T("help.nodes_chain.reorder")},
				{"Backspace / d", i18n.T("help.nodes_chain.remove")},
				{"x", i18n.T("help.nodes_chain.clear")},
				{"Ctrl+S / s", i18n.T("help.nodes_chain.apply")},
				{"Esc", i18n.T("help.nodes_chain.close")},
			},
		}}
	}

	if ctx.NodesFilterMode {
		return []section{{
			title: i18n.T("help.section.nodes_search"),
			bindings: []keybinding{
				{"输入字符", i18n.T("help.nodes_search.append")},
				{"Backspace", i18n.T("help.nodes_search.backspace")},
				{"Ctrl+R", i18n.T("help.nodes_search.regex")},
				{"Ctrl+F", i18n.T("help.nodes_search.fuzzy")},
				{"↵", i18n.T("help.nodes_search.confirm")},
				{"Esc", i18n.T("help.nodes_search.cancel")},
			},
		}}
	}

	if ctx.NodesTestDetail {
		return []section{{
			title: i18n.T("help.section.nodes_test_detail"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.nodes_test_detail.scroll")},
				{"Home / End", i18n.T("help.nodes_test_detail.jump")},
				{"f / Esc", i18n.T("help.nodes_test_detail.close")},
			},
		}}
	}

	return []section{{
		title: i18n.T("help.section.nodes_mgr"),
		bindings: []keybinding{
			{"↑/↓  k/j", i18n.T("help.nodes.select")},
			{"←/→  h/l", i18n.T("help.nodes.switch_group")},
			{"↵", i18n.T("help.nodes.switch_node")},
			{"t", i18n.T("help.nodes.test")},
			{"a", i18n.T("help.nodes.test_all")},
			{"m", i18n.T("help.nodes.mode")},
			{"v", i18n.T("help.nodes.layout")},
			{"g", i18n.T("help.nodes.locate")},
			{"c", i18n.T("help.nodes.chain")},
			{"s", i18n.T("help.nodes.sort")},
			{"/", i18n.T("help.nodes.search")},
			{"f", i18n.T("help.nodes.test_detail")},
		},
	}}
}

// buildConnectionsSections 连接页帮助分区
func buildConnectionsSections(ctx HelpContext) []section {
	if ctx.ConnDetail {
		return []section{{
			title: i18n.T("help.section.conns_detail"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.conns_detail.scroll")},
				{"←/→  h/l", i18n.T("help.conns_detail.switch_panel")},
				{"Esc / q", i18n.T("help.conns_detail.close")},
			},
		}}
	}

	if ctx.ConnTopN {
		return []section{{
			title: i18n.T("help.section.conns_topn"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.conns_topn.scroll")},
				{"↵", i18n.T("help.conns_topn.detail")},
				{"Esc / q", i18n.T("help.conns_topn.close")},
			},
		}}
	}

	if ctx.ConnFilter {
		return []section{{
			title: i18n.T("help.section.conns_search"),
			bindings: []keybinding{
				{"输入字符", i18n.T("help.conns_search.append")},
				{"↵", i18n.T("help.conns_search.confirm")},
				{"Esc", i18n.T("help.conns_search.cancel")},
			},
		}}
	}

	switch ctx.ConnViewMode {
	case ConnViewTraffic:
		return []section{{
			title: i18n.T("help.section.conns_traffic"),
			bindings: []keybinding{
				{"h", i18n.T("help.conns_traffic.switch")},
				{"←/→", i18n.T("help.conns_traffic.select_site")},
				{"s", i18n.T("help.conns_traffic.test_site")},
				{"S", i18n.T("help.conns_traffic.test_all")},
				{"U", i18n.T("conns.unlock.title")},
				{"D", i18n.T("conns.dns.title")},
			},
		}}

	case ConnViewActive:
		return []section{{
			title: i18n.T("help.section.conns_active"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.conns_active.select")},
				{"↵", i18n.T("help.conns_active.detail")},
				{"p / Space", i18n.T("help.conns.pause")},
				{"s / S", i18n.T("help.conns.sort")},
				{"t", i18n.T("help.conns.filter_proto")},
				{"f", i18n.T("help.conns.filter_field")},
				{"x", i18n.T("help.conns_active.close_conn")},
				{"X", i18n.T("help.conns_active.close_all")},
				{"/", i18n.T("help.conns_active.search")},
				{"h", i18n.T("help.conns_active.switch")},
				{"Esc", i18n.T("help.conns_active.clear")},
				{"U", i18n.T("conns.unlock.title")},
				{"D", i18n.T("conns.dns.title")},
			},
		}}

	case ConnViewHistory:
		return []section{{
			title: i18n.T("help.section.conns_history"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.conns_history.select")},
				{"↵", i18n.T("help.conns_history.detail")},
				{"p / Space", i18n.T("help.conns.pause")},
				{"s / S", i18n.T("help.conns.sort")},
				{"t", i18n.T("help.conns.filter_proto")},
				{"f", i18n.T("help.conns.filter_field")},
				{"/", i18n.T("help.conns_history.search")},
				{"h", i18n.T("help.conns_history.switch")},
				{"Esc", i18n.T("help.conns_history.clear")},
				{"U", i18n.T("conns.unlock.title")},
				{"D", i18n.T("conns.dns.title")},
			},
		}}
	}

	return nil
}

// buildLogsSections 日志页帮助分区
func buildLogsSections(ctx HelpContext) []section {
	if ctx.LogsDetail {
		return []section{{
			title: i18n.T("help.section.logs_detail"),
			bindings: []keybinding{
				{"↑/↓  k/j", i18n.T("help.logs_detail.scroll")},
				{"Esc / q", i18n.T("help.logs_detail.close")},
			},
		}}
	}

	if ctx.LogsFilter {
		return []section{{
			title: i18n.T("help.section.logs_search"),
			bindings: []keybinding{
				{"输入字符", i18n.T("help.logs_search.append")},
				{"Backspace", i18n.T("help.logs_search.backspace")},
				{"↵", i18n.T("help.logs_search.confirm")},
				{"Esc", i18n.T("help.logs_search.cancel")},
			},
		}}
	}

	return []section{{
		title: i18n.T("help.section.logs"),
		bindings: []keybinding{
			{"↑/↓  k/j", i18n.T("help.logs.select")},
			{"↵", i18n.T("help.logs.detail")},
			{"[ / ]", i18n.T("help.logs.level_down")},
			{"←/→  h/l", i18n.T("help.logs.scroll")},
			{"p / Space", i18n.T("help.logs.pause")},
			{"/", i18n.T("help.logs.search")},
			{"c", i18n.T("help.logs.clear")},
			{"Esc", i18n.T("help.logs.clear_search")},
		},
	}}
}

// buildRulesSections 规则页帮助分区
func buildRulesSections(ctx HelpContext) []section {
	if ctx.RulesDeleteConfirm {
		return []section{{
			title: i18n.T("help.section.rules_delete"),
			bindings: []keybinding{
				{"↵ / y", i18n.T("help.rules_delete.confirm")},
				{"Esc / n", i18n.T("help.rules_delete.cancel")},
			},
		}}
	}

	if ctx.RulesEditForm {
		return []section{
			{
				title: i18n.T("rules.edit_title"),
				bindings: []keybinding{
					{"← / →", i18n.T("help.rules_add.type")},
					{"Space", i18n.T("help.rules_add.type_input")},
					{"↑ / ↓", i18n.T("help.rules_add.field")},
					{"↵", i18n.T("help.rules_add.confirm")},
					{"Esc", i18n.T("help.rules_add.cancel")},
				},
			},
		}
	}

	if ctx.RulesTypeFilter && !ctx.RulesFilter {
		return []section{{
			title: i18n.T("help.section.rules_type_filter"),
			bindings: []keybinding{
				{"← / →", i18n.T("help.rules_filter.select")},
				{"Space", i18n.T("help.rules_filter.toggle")},
				{"t", i18n.T("help.rules.hint_type")},
				{"/", i18n.T("help.rules.hint_search")},
			},
		}}
	}

	if ctx.RulesFilter {
		return []section{{
			title: i18n.T("help.section.rules_search"),
			bindings: []keybinding{
				{"输入字符", i18n.T("help.rules_search.append")},
				{"Backspace", i18n.T("help.rules_search.backspace")},
				{"↵", i18n.T("help.rules_search.confirm")},
				{"Esc", i18n.T("help.rules_search.cancel")},
			},
		}}
	}

	return []section{{
		title: i18n.T("help.section.rules"),
		bindings: []keybinding{
			{"↑/↓  k/j", i18n.T("help.rules.select")},
			{"/", i18n.T("help.rules.search")},
			{"t", i18n.T("help.rules.type")},
			{"n", i18n.T("help.rules.add")},
			{"d", i18n.T("help.rules.delete")},
			{"e", i18n.T("help.rules.edit")},
			{"Esc", i18n.T("help.rules.clear")},
		},
	}}
}

// buildSettingsSections 设置页帮助分区
func buildSettingsSections(ctx HelpContext) []section {
	if ctx.SettingsEdit {
		if ctx.SettingsLanguage {
			return []section{{
				title: i18n.T("help.section.settings_edit_lang"),
				bindings: []keybinding{
					{"←/→ / Tab", i18n.T("help.settings_edit_lang.switch")},
					{"↵", i18n.T("help.settings_edit_lang.confirm")},
					{"Esc", i18n.T("help.settings_edit_lang.cancel")},
				},
			}}
		}
		if ctx.SettingsTheme {
			return []section{{
				title: i18n.T("settings.label.theme"),
				bindings: []keybinding{
					{"←/→ / Tab", i18n.T("help.settings_edit_theme.switch")},
					{"↵", i18n.T("help.settings_edit_theme.confirm")},
					{"Esc", i18n.T("help.settings_edit_theme.cancel")},
				},
			}}
		}
		return []section{{
			title: i18n.T("help.section.settings_edit_config"),
			bindings: []keybinding{
				{"←/→", i18n.T("help.settings_edit_config.move")},
				{"Home / End", i18n.T("help.settings_edit_config.jump")},
				{"Backspace", i18n.T("help.settings_edit_config.backspace")},
				{"Delete", i18n.T("help.settings_edit_config.delete")},
				{"↵", i18n.T("help.settings_edit_config.confirm")},
				{"Esc", i18n.T("help.settings_edit_config.cancel")},
			},
		}}
	}

	return []section{{
		title: i18n.T("help.section.settings"),
		bindings: []keybinding{
			{"↑/↓", i18n.T("help.settings.select")},
			{"Enter / 双击", i18n.T("help.settings.edit")},
		},
	}}
}

// ── 订阅页交互模式常量（与 features/sub SubMode* 保持一致，避免循环依赖）──
const (
	subModeNormal     = 0 // 普通（列表浏览）
	subModeSearch     = 1 // 搜索输入
	subModeAddForm    = 2 // 添加订阅表单
	subModeDeleteConf = 4 // 删除确认
)

// buildSubSections 订阅页帮助分区。
func buildSubSections(ctx HelpContext) []section {
	switch ctx.SubMode {
	case subModeSearch:
		return []section{{
			title: i18n.T("help.section.sub_search"),
			bindings: []keybinding{
				{"输入字符", i18n.T("help.sub_search.append")},
				{"Backspace", i18n.T("help.sub_search.backspace")},
				{"↵", i18n.T("help.sub_search.confirm")},
				{"Esc", i18n.T("help.sub_search.cancel")},
			},
		}}
	case subModeAddForm:
		return []section{{
			title: i18n.T("help.section.sub_add"),
			bindings: []keybinding{
				{"↑/↓", i18n.T("help.sub_add.field")},
				{"↵", i18n.T("help.sub_add.confirm")},
				{"Esc", i18n.T("help.sub_add.cancel")},
			},
		}}
	case subModeDeleteConf:
		return []section{{
			title: i18n.T("help.section.sub_delete"),
			bindings: []keybinding{
				{"↵ / y", i18n.T("help.sub_delete.confirm")},
				{"Esc / n", i18n.T("help.sub_delete.cancel")},
			},
		}}
	default:
		return []section{{
			title: i18n.T("help.section.sub"),
			bindings: []keybinding{
				{"↑/↓", i18n.T("help.sub.select")},
				{"Enter / 双击", i18n.T("help.sub.activate")},
				{"u", i18n.T("help.sub.update")},
				{"U", i18n.T("help.sub.update_all")},
				{"m", i18n.T("help.sub.edit_merge")},
				{"H", i18n.T("help.sub.history")},
				{"e", i18n.T("help.sub.edit_profile")},
				{"n", i18n.T("help.sub.add")},
				{"d", i18n.T("help.sub.delete")},
				{"r", i18n.T("help.sub.refresh")},
				{"/", i18n.T("help.sub.search")},
				{"v", i18n.T("help.sub.layout")},
			},
		}}
	}
}

// ── 颜色访问——使用 common 包函数，支持主题热切换 ──

// OverlayHelpPopup 将帮助弹窗居中叠加在 base 页面之上。
//
// 步骤：
//  1. 对 base 每行整体套 Faint，让背景内容降亮但不消失
//  2. 渲染弹窗本体，并计算居中偏移量
//  3. 逐行用 ansi.Cut 截取底层的左侧和右侧，将弹窗内容嵌入中间
func OverlayHelpPopup(base string, width, height int, ctx HelpContext) string {
	popup := renderPopup(width, height, ctx)
	return dialog.New(popup, width, height).Clipped().Overlay(base, true)
}

// renderPopup 渲染帮助弹窗（无背景色，使用终端默认背景）
func renderPopup(termWidth, termHeight int, ctx HelpContext) string {
	popup, _ := renderScrollablePopup(termWidth, termHeight, ctx)
	return popup
}

func renderScrollablePopup(termWidth, termHeight int, ctx HelpContext) (string, int) {
	if termWidth <= 0 || termHeight <= 0 {
		return "", 0
	}
	// ── 动态构建帮助分区 ──
	helpSections := buildHelpSections(ctx)

	// Measure the rendered cards, including wrapped descriptions and notes.
	sectionHeights := make([]int, len(helpSections))

	// ── 弹窗尺寸 ──
	popupWidth := termWidth * 88 / 100
	if popupWidth > 112 {
		popupWidth = 112
	}
	if popupWidth < 60 {
		popupWidth = 60
	}
	popupWidth = min(popupWidth, termWidth)

	popupHeight := termHeight * 85 / 100
	if popupHeight > 42 {
		popupHeight = 42
	}
	if popupHeight < 20 {
		popupHeight = 20
	}
	popupHeight = min(popupHeight, termHeight)

	// 内容区宽度 = 弹窗宽度 - 边框(2) - 内边距(左右各2=4)
	innerWidth := max(1, popupWidth-6)

	// ── 智能列数：根据 section 数量和可用宽度决定 ──
	maxCols := 3
	if innerWidth < 80 {
		maxCols = 2
	}
	if innerWidth < 48 {
		maxCols = 1
	}
	cols := maxCols
	if len(helpSections) < cols {
		cols = len(helpSections)
	}
	if cols < 1 {
		cols = 1
	}

	// ── 标题行 ──
	title := lipgloss.NewStyle().Bold(true).Foreground(common.TokyoForeground()).Render(i18n.T("help.popup_title"))
	closeHint := lipgloss.NewStyle().Foreground(common.TokyoMuted()).Render(i18n.T("help.close_hint"))
	gap := innerWidth - lipgloss.Width(title) - lipgloss.Width(closeHint)
	if gap < 1 {
		gap = 1
	}
	titleLine := ansi.Truncate(title+strings.Repeat(" ", gap)+closeHint, innerWidth, "")

	divider := lipgloss.NewStyle().Foreground(common.TokyoMuted()).Render(strings.Repeat("─", innerWidth))

	// ── 各分区卡片 ──
	colW := max(1, innerWidth/cols-1)
	cards := make([]string, 0, len(helpSections))
	for i, sec := range helpSections {
		card := renderSection(sec, colW)
		cards = append(cards, card)
		sectionHeights[i] = lipgloss.Height(card)
	}

	// ── 按高度贪心分配列（避免内容高度差异过大）──
	colContents := distributeCardsToColumns(cards, sectionHeights, cols, colW)

	var body string
	if len(colContents) == 1 {
		body = colContents[0]
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, colContents...)
	}

	// Keep the title and hint fixed while scrolling all explanatory content.
	bodyLines := strings.Split(body, "\n")
	budget := max(1, popupHeight-8)
	limit := max(0, len(bodyLines)-budget)
	offset := max(0, min(ctx.ScrollOffset, limit))
	bodyLines = bodyLines[offset:min(len(bodyLines), offset+budget)]
	for len(bodyLines) < budget {
		bodyLines = append(bodyLines, "")
	}
	hint := ansi.Truncate(i18n.T("help.scroll_hint"), innerWidth, "")
	content := lipgloss.JoinVertical(lipgloss.Left, titleLine, divider, "", strings.Join(bodyLines, "\n"),
		lipgloss.NewStyle().Foreground(common.TokyoMuted()).Render(hint))

	// ── 弹窗外壳：圆角边框，无背景色 ──
	popupStyle := lipgloss.NewStyle().
		Border(theme.Frame()).
		BorderForeground(common.TokyoBlue()).
		Padding(1, 2).
		Width(max(1, popupWidth-2))

	lines := strings.Split(popupStyle.Render(content), "\n")
	lines = lines[:min(len(lines), termHeight)]
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], termWidth, "")
	}
	return strings.Join(lines, "\n"), limit
}

// distributeCardsToColumns 将卡片按高度贪心分配到指定列数，每列固定宽度并左对齐
func distributeCardsToColumns(cards []string, heights []int, cols int, colWidth int) []string {
	if len(cards) == 0 {
		return nil
	}
	if cols <= 1 || len(cards) <= 1 {
		return []string{lipgloss.JoinVertical(lipgloss.Left, cards...)}
	}

	// 贪心：每张卡片分配给当前最矮的列
	colCards := make([][]string, cols)
	colHeights := make([]int, cols)
	for i := range colCards {
		colCards[i] = make([]string, 0)
	}

	for i, card := range cards {
		minCol := 0
		for c := 1; c < cols; c++ {
			if colHeights[c] < colHeights[minCol] {
				minCol = c
			}
		}
		colCards[minCol] = append(colCards[minCol], card)
		if i < len(heights) {
			colHeights[minCol] += heights[i]
		}
	}

	// 固定列宽 + 左对齐，确保各列等宽、内容统一左对齐
	colStyle := lipgloss.NewStyle().Width(colWidth).Align(lipgloss.Left)

	var result []string
	for _, cc := range colCards {
		if len(cc) > 0 {
			joined := lipgloss.JoinVertical(lipgloss.Left, cc...)
			result = append(result, colStyle.Render(joined))
		}
	}
	return result
}

// renderSection 渲染单个快捷键分区（无背景色）
func renderSection(sec section, width int) string {
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(common.TokyoCyan())

	keyWidth := 16
	if width < 38 {
		keyWidth = 12
	}
	keyWidth = min(keyWidth, max(1, width/2))
	keyStyle := lipgloss.NewStyle().Foreground(common.TokyoYellow()).Width(keyWidth)
	descStyle := lipgloss.NewStyle().Foreground(common.Dim())

	var lines []string
	lines = append(lines, sectionStyle.Render(ansi.Wrap(sec.title, max(1, width), "")))
	for _, note := range sec.notes {
		lines = append(lines, descStyle.Render(ansi.Wrap(note, max(1, width), "")))
	}

	for _, b := range sec.bindings {
		k := b.key
		desc := b.desc

		if sec.title == i18n.T("help.section.latency_colors") {
			var dot string
			switch {
			case strings.Contains(k, "绿") || strings.Contains(strings.ToLower(k), "green"):
				dot = lipgloss.NewStyle().Foreground(common.TokyoGreen()).Render("●")
			case strings.Contains(k, "黄") || strings.Contains(strings.ToLower(k), "yellow"):
				dot = lipgloss.NewStyle().Foreground(common.TokyoYellow()).Render("●")
			case strings.Contains(k, "红") || strings.Contains(strings.ToLower(k), "red"):
				dot = lipgloss.NewStyle().Foreground(common.TokyoRed()).Render("●")
			default:
				dot = "●"
			}
			description := strings.ReplaceAll(ansi.Wrap(desc, max(1, width-4), ""), "\n", "\n    ")
			lines = append(lines, "  "+dot+" "+descStyle.Render(description))
			continue
		}

		keyLines := strings.Split(ansi.Wrap(k, keyWidth, ""), "\n")
		descLines := strings.Split(ansi.Wrap(desc, max(1, width-keyWidth-2), ""), "\n")
		for i := 0; i < max(len(keyLines), len(descLines)); i++ {
			key, description := "", ""
			if i < len(keyLines) {
				key = keyLines[i]
			}
			if i < len(descLines) {
				description = descLines[i]
			}
			lines = append(lines, "  "+keyStyle.Render(key)+descStyle.Render(description))
		}
	}

	lines = append(lines, "") // 分区间距
	return strings.Join(lines, "\n")
}

// RenderHelpPage 旧接口，保留兼容（不传上下文时使用全局+颜色图例）
func RenderHelpPage(width, height int) string {
	return renderPopup(width, height, HelpContext{CurrentPage: PageNodes})
}
