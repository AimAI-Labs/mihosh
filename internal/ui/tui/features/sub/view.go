package sub

// view.go — 订阅页渲染与弹窗叠加。
//
// 布局复用 rules 页的 header + 列表 + overlay 弹窗模式：
//   1. header（搜索框 + 统计）
//   2. 订阅卡片或列表
//   3. 可选叠加：添加表单 / merge 编辑器 / 删除确认

import (
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	subFixedLines   = 4 // header(3) + 间距(1)
	subScrollWidth  = 1
	subHeaderHeight = 3
)

// PageState 订阅页渲染状态。
type PageState struct {
	Subs        []profile.Profile
	FilteredIdx []int
	ActiveUID   string
	FilterText  string
	FilterMode  bool
	FilterEngine
	Selected  int
	ScrollTop int
	ListMode  bool
	Width     int
	Height    int

	ShowAddForm    bool
	AddForm        addForm
	ShowEditForm   bool
	EditForm       addForm
	ShowDeleteConf bool
	DeleteUID      string
	UpdatingUID    string
	File           fileView
}

// RenderSubPage 渲染订阅页。
func RenderSubPage(state PageState) string {
	var sections []string

	stats := i18n.Tf("sub.stats", len(state.FilteredIdx))
	if state.FilterText != "" {
		stats += i18n.Tf("sub.stats_filtered", len(state.Subs))
	}

	searchBox := renderSearchBox(state.FilterText, state.FilterMode, state.FilterEngine)
	header := renderHeader(stats, searchBox, state.Width, state.ListMode)
	sections = append(sections, header, "")

	list := renderList(state, layoutSubList(state))
	sections = append(sections, list)

	result := strings.Join(sections, "\n")

	if state.ShowAddForm {
		result = renderAddFormOverlay(result, state, state.Width, state.Height)
	}
	if state.ShowEditForm {
		result = renderEditFormOverlay(result, state, state.Width, state.Height)
	}
	if state.ShowDeleteConf {
		result = renderDeleteConfirmOverlay(result, state, state.Width, state.Height)
	}

	result = renderInlineHelp(result, state)
	if state.File.mode != fileClosed {
		return state.File.dialog(state.Width, state.Height).Overlay(result, true)
	}
	if (state.ShowAddForm || state.ShowEditForm) && state.Width > 0 && state.Height > 0 {
		lines := strings.Split(result, "\n")
		lines = lines[:min(len(lines), state.Height)]
		for i := range lines {
			lines[i] = ansi.Truncate(lines[i], state.Width, "")
		}
		result = strings.Join(lines, "\n")
	}
	return result
}

func renderHeader(stats, searchBox string, width int, listMode bool) string {
	borderStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue())
	statsStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue())

	innerWidth := max(1, width-2)
	menu := subLayoutTabs(width).render(listMode)
	available := innerWidth - lipgloss.Width(menu)

	statsText := statsStyle.Render(stats)
	var content string
	if room := available - lipgloss.Width(statsText) - 1; room >= min(lipgloss.Width(searchBox), 10) {
		content = common.PadString(ansi.Truncate(searchBox, room, "…"), room) + " " + statsText
	} else {
		content = common.PadString(ansi.Truncate(searchBox, available, "…"), available)
	}
	searchRow := borderStyle.Render("│") + content + menu + borderStyle.Render("│")
	top := borderStyle.Render(theme.Frame().TopLeft + strings.Repeat("─", innerWidth) + theme.Frame().TopRight)
	bottom := borderStyle.Render(theme.Frame().BottomLeft + strings.Repeat("─", innerWidth) + theme.Frame().BottomRight)
	return top + "\n" + searchRow + "\n" + bottom
}

func renderSearchBox(filterText string, filterMode bool, engine FilterEngine) string {
	inputStyle := lipgloss.NewStyle().Foreground(common.Bright())
	if filterMode {
		inputStyle = inputStyle.Background(common.Highlight())
	}
	label := common.MutedStyle().Render(i18n.T("sub.search"))
	input := inputStyle.Render(filterText)
	if filterMode {
		input += inputStyle.Render("█")
	}
	engineIndicator := common.RenderSearchEngineBadge(engine)

	hint := ""
	if filterText == "" {
		switch engine {
		case FilterRegex:
			hint = common.MutedStyle().Render(i18n.T("sub.search_hint_regex"))
		case FilterFuzzy:
			hint = common.MutedStyle().Render(i18n.T("sub.search_hint_fuzzy"))
		default:
			hint = common.MutedStyle().Render(i18n.T("sub.search_hint"))
		}
	}
	return label + input + hint + engineIndicator
}

// relativeTime 把 unix 时间戳转为相对时间描述。
func relativeTime(ts int64) string {
	if ts <= 0 {
		return i18n.T("sub.never_updated")
	}
	d := time.Since(time.Unix(ts, 0))
	switch {
	case d < time.Minute:
		return i18n.T("sub.time_just_now")
	case d < time.Hour:
		return i18n.Tf("sub.time_minutes_ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return i18n.Tf("sub.time_hours_ago", int(d.Hours()))
	default:
		return i18n.Tf("sub.time_days_ago", int(d.Hours()/24))
	}
}

// ============================================================
// 内联帮助（右下角浮层）
// ============================================================

func visibleInlineHints(state PageState) []common.InlineHelpHint {
	if state.File.mode != fileClosed {
		return nil
	}
	hints := buildHints(state)
	if len(hints) == 0 {
		return nil
	}
	return common.FitInlineHints(hints, max(1, state.Width-1))
}

// InlineHelpHints 返回订阅页当前可见的右下角交互菜单条目。
func (s State) InlineHelpHints(width, height int) []common.InlineHelpHint {
	return visibleInlineHints(s.ToPageState(width, height))
}

func renderInlineHelp(page string, state PageState) string {
	hints := visibleInlineHints(state)
	if len(hints) == 0 {
		return page
	}
	body := common.FormatInlineHintRow(hints)
	return common.OverlayHelpAtBottomRight(page, body, state.Width, state.Height)
}

func buildHints(state PageState) []common.InlineHelpHint {
	if state.ShowDeleteConf {
		return []common.InlineHelpHint{
			{Key: "Enter/y", Desc: i18n.T("help.sub_delete.confirm"), Tone: common.InlineHelpToneDanger},
			{Key: "Esc/n", Desc: i18n.T("help.sub_delete.cancel")},
		}
	}
	if state.ShowAddForm || state.ShowEditForm {
		return []common.InlineHelpHint{
			{Key: "↵", Desc: i18n.T("help.sub_add.confirm"), Tone: common.InlineHelpToneSuccess},
			{Key: "Esc", Desc: i18n.T("help.sub_add.cancel")},
			{Key: "Tab", Desc: i18n.T("help.sub_add.field")},
		}
	}
	if state.FilterMode {
		return []common.InlineHelpHint{
			{Key: "↵", Desc: i18n.T("help.sub_search.confirm")},
			{Key: "Esc", Desc: i18n.T("help.sub_search.cancel")},
			{Key: "⌫", Desc: i18n.T("help.sub_search.backspace")},
			{Key: "Ctrl+R/F", Desc: i18n.T("help.rules_search.regex_fuzzy")},
		}
	}
	selection := "↑↓"
	if !state.ListMode && subCardColumns(state.Width) > 1 {
		selection = "↑↓←→"
	}
	return []common.InlineHelpHint{
		{Key: selection, Desc: i18n.T("help.sub.select")},
		{Key: "↵", Desc: i18n.T("help.sub.activate")},
		{Key: "u", Desc: i18n.T("help.sub.update"), Tone: common.InlineHelpToneSuccess},
		{Key: "U", Desc: i18n.T("help.sub.update_all"), Tone: common.InlineHelpToneSuccess},
		{Key: "m", Desc: i18n.T("help.sub.edit_merge")},
		{Key: "c", Desc: i18n.T("help.sub.view_raw")},
		{Key: "e", Desc: i18n.T("help.sub.edit_profile")},
		{Key: "n", Desc: i18n.T("help.sub.add")},
		{Key: "d", Desc: i18n.T("help.sub.delete")},
		{Key: "/", Desc: i18n.T("help.sub.search")},
		{Key: "v", Desc: i18n.T("help.sub.layout")},
		{Key: "?", Desc: i18n.T("core.inline.help")},
	}
}

// ============================================================
// 弹窗叠加（暗化底层 → 居中弹窗 → 逐行嵌入）
// ============================================================

const ()

func formLabelWidth() int {
	width := 0
	for _, key := range []string{
		"sub.add_field_name",
		"sub.add_field_kind",
		"sub.add_field_src",
		"sub.add_field_proxy",
		"sub.add_field_ua",
		"sub.add_field_skip_tls",
		"sub.add_field_interval",
	} {
		width = max(width, common.DisplayWidth(i18n.T(key)))
	}
	return width
}

func formDimensions(width, height int) (modalWidth, fieldWidth, sourceHeight int) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	modalWidth = max(24, min(76, width-4))
	fieldWidth = max(1, modalWidth-4-formLabelWidth()-1)
	sourceHeight = max(1, min(6, height-10))
	return
}

type formRowSpan struct {
	field int
	start int
	end   int
}

func formBodyLayout(form addForm, width, height int) (lines []string, spans []formRowSpan, offset int) {
	modalWidth, fieldWidth, _ := formDimensions(width, height)
	form.resize(width, height)
	innerWidth := modalWidth - 4

	appendField := func(field int, rendered string) {
		start := len(lines)
		lines = append(lines, strings.Split(rendered, "\n")...)
		spans = append(spans, formRowSpan{field: field, start: start, end: len(lines)})
	}

	renderBoolToggle := func(enabled bool) string {
		offLabel := i18n.T("sub.opt_off")
		onLabel := i18n.T("sub.opt_on")
		offStyle := common.TokyoMutedStyle()
		onStyle := common.TokyoMutedStyle()
		if enabled {
			onStyle = onStyle.Foreground(common.TokyoCyan()).Bold(true)
		} else {
			offStyle = offStyle.Foreground(common.TokyoCyan()).Bold(true)
		}
		if fieldWidth < common.DisplayWidth(offLabel+" │ "+onLabel) {
			if enabled {
				return "◀ " + onLabel + " ▶"
			}
			return "◀ " + offLabel + " ▶"
		}
		return offStyle.Render(offLabel) + " │ " + onStyle.Render(onLabel)
	}

	// 名称行
	appendField(addFieldName, renderFieldRow(i18n.T("sub.add_field_name"), form.nameField.View(), form.isNameField(), innerWidth))

	var options []string
	for i, label := range []string{"remote", "local"} {
		style := common.TokyoMutedStyle()
		if (i == 0) == form.kindRemote {
			style = style.Foreground(common.TokyoCyan()).Bold(true)
		}
		options = append(options, style.Render(label))
	}
	kindVal := strings.Join(options, " │ ")
	if fieldWidth < common.DisplayWidth("remote │ local") {
		label := "remote"
		if !form.kindRemote {
			label = "local"
		}
		kindVal = "◀ " + label + " ▶"
	}
	appendField(addFieldKind, renderFieldRow(i18n.T("sub.add_field_kind"), kindVal, form.isKindField(), innerWidth))

	// 来源值行
	srcLabel := i18n.T("sub.add_field_src")
	srcValue := form.srcField.View()
	if button := form.copyButton(fieldWidth); button != "" {
		style := lipgloss.NewStyle().Background(common.TokyoCyan()).Foreground(common.Background()).Bold(true)
		srcValue = lipgloss.JoinHorizontal(lipgloss.Top, srcValue, " ", style.Render(button))
	}
	appendField(addFieldSrc, renderFieldRow(srcLabel, srcValue, form.isSrcField(), innerWidth))

	if form.kindRemote {
		appendField(addFieldProxy, renderFieldRow(i18n.T("sub.add_field_proxy"), renderBoolToggle(form.useProxy), form.isProxyField(), innerWidth))
		appendField(addFieldUA, renderFieldRow(i18n.T("sub.add_field_ua"), form.uaField.View(), form.isUAField(), innerWidth))
		appendField(addFieldSkipTLS, renderFieldRow(i18n.T("sub.add_field_skip_tls"), renderBoolToggle(form.skipTLS), form.isSkipTLSField(), innerWidth))
		intervalLabel := i18n.T("sub.interval_off")
		if form.updateInterval > 0 {
			intervalLabel = profile.FormatUpdateInterval(form.updateInterval)
		}
		intervalVal := "◀ " + lipgloss.NewStyle().Foreground(common.TokyoCyan()).Bold(true).Render(intervalLabel) + " ▶"
		appendField(addFieldInterval, renderFieldRow(i18n.T("sub.add_field_interval"), intervalVal, form.isIntervalField(), innerWidth))
	}

	// 高度预算：最大高度 height-1，减去边框(2)+分隔线(1)+底部提示(1) = height-5
	budget := max(1, height-5)
	if len(lines) > budget {
		focusStart, focusEnd := 0, 1
		for _, sp := range spans {
			if sp.field == form.fieldCursor {
				focusStart, focusEnd = sp.start, sp.end
				break
			}
		}
		if focusEnd > offset+budget {
			offset = focusEnd - budget
		}
		if focusStart < offset {
			offset = focusStart
		}
		maxOffset := len(lines) - budget
		if offset > maxOffset {
			offset = maxOffset
		}
		if offset < 0 {
			offset = 0
		}
		lines = lines[offset:min(len(lines), offset+budget)]
	}
	return lines, spans, offset
}

// ===== 添加/编辑表单（共用渲染） =====

func renderAddFormOverlay(background string, state PageState, width, height int) string {
	return dialog.New(buildAddFormModal(state, width, height), width, max(1, height-1)).Below(subHeaderHeight+1).Clipped().Overlay(background, true)
}

func renderEditFormOverlay(background string, state PageState, width, height int) string {
	return dialog.New(buildEditFormModal(state, width, height), width, max(1, height-1)).Below(subHeaderHeight+1).Clipped().Overlay(background, true)
}

func buildAddFormModal(state PageState, width, height int) string {
	return buildFormModal(i18n.T("sub.add_title"), state.AddForm, width, height)
}

func buildEditFormModal(state PageState, width, height int) string {
	return buildFormModal(i18n.T("sub.edit_title"), state.EditForm, width, height)
}

// buildFormModal 渲染订阅表单弹窗（添加/编辑共用，仅标题不同）。
func buildFormModal(title string, form addForm, width, height int) string {
	modalWidth, _, _ := formDimensions(width, height)
	innerWidth := modalWidth - 4

	bodyLines, _, _ := formBodyLayout(form, width, height)

	// 说明/错误行
	var footer string
	switch {
	case form.errMsg != "":
		footer = lipgloss.NewStyle().Foreground(common.Danger()).Render("✗ " + form.errMsg)
	case form.isKindField():
		footer = common.TokyoMutedStyle().Render(i18n.T("sub.add_kind_hint"))
	case form.isProxyField():
		footer = common.TokyoMutedStyle().Render(i18n.T("sub.add_proxy_hint"))
	case form.isUAField():
		footer = common.TokyoMutedStyle().Render(i18n.T("sub.add_ua_hint"))
	case form.isSkipTLSField():
		footer = common.TokyoMutedStyle().Render(i18n.T("sub.add_skip_tls_hint"))
	case form.isIntervalField():
		footer = common.TokyoMutedStyle().Render(i18n.T("sub.add_interval_hint"))
	default:
		footer = common.TokyoMutedStyle().Render(i18n.T("sub.add_src_hint"))
	}
	footer = ansi.Truncate(footer, innerWidth, "…")

	divider := lipgloss.NewStyle().Foreground(common.TokyoBlue()).Render(strings.Repeat("─", innerWidth))
	allRows := append(append([]string(nil), bodyLines...), divider, footer)
	content := strings.Join(allRows, "\n")

	return common.RenderBorderedPanel(
		title,
		content,
		modalWidth,
		common.TokyoBlue(),
		common.TokyoForeground(),
	)
}

// renderFieldRow 渲染 label + value 行，聚焦时整行铺 TokyoSelected 背景。
func renderFieldRow(label, value string, focused bool, innerWidth int) string {
	labelStyle := common.TokyoMutedStyle()
	if focused {
		labelStyle = lipgloss.NewStyle().Foreground(common.TokyoCyan())
	}

	// 使用 JoinHorizontal 确保多行 value（如 textarea）时，label 只在第一行对齐
	row := lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render(common.PadString(label, formLabelWidth())), " ", value)

	if !focused {
		return row
	}

	// 聚焦时使用 Lipgloss 的 Width 属性和 Padding 自动处理多行背景铺满
	return lipgloss.NewStyle().
		Background(common.TokyoSelected()).
		Width(innerWidth).
		Render(row)
}

// resolveFormBounds 返回表单弹窗边界（供鼠标点击外部关闭判断）。
// 添加/编辑表单尺寸完全一致，共用一份实现。
func resolveFormBounds(state PageState, width, height int) (left, top, right, bottom int) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0
	}
	var modal string
	if state.ShowEditForm {
		modal = buildEditFormModal(state, width, height)
	} else {
		modal = buildAddFormModal(state, width, height)
	}
	return dialog.New(modal, width, max(1, height-1)).Below(subHeaderHeight + 1).Clipped().Bounds()
}

// ===== 删除确认 =====

func renderDeleteConfirmOverlay(background string, state PageState, width, height int) string {
	var name, source, warning string
	for _, p := range state.Subs {
		if p.UID == state.DeleteUID {
			name = p.Name
			source = p.Source.Kind.String() + " " + p.Source.Display()
			if p.UID == state.ActiveUID {
				warning = i18n.T("sub.delete_warn_active")
			}
			break
		}
	}
	nameLine := lipgloss.NewStyle().Foreground(common.Bright()).Render(name)
	sourceLine := lipgloss.NewStyle().Foreground(common.Secondary()).Render(source)
	return dialog.DeleteConfirmation(width, height, i18n.T("sub.delete_title"), i18n.T("sub.delete_confirm_hint"),
		warning, nameLine, sourceLine).Below(subHeaderHeight+1).Overlay(background, true)
}
