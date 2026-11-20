package sub

// view.go — 订阅页渲染与弹窗叠加。
//
// 布局复用 rules 页的 header + 列表 + overlay 弹窗模式：
//   1. header（搜索框 + 统计）
//   2. 列表（序号 / 名称 / 类型 / 来源 / 更新时间）
//   3. 可选叠加：添加表单 / merge 编辑器 / 删除确认

import (
	"fmt"
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
	subMinHeight    = 5
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

	// 统计
	remoteCount := 0
	for _, idx := range state.FilteredIdx {
		if idx >= 0 && idx < len(state.Subs) && state.Subs[idx].Source.Kind == profile.SourceRemote {
			remoteCount++
		}
	}
	stats := i18n.Tf("sub.stats", len(state.FilteredIdx))
	if state.FilterText != "" {
		stats += i18n.Tf("sub.stats_filtered", len(state.Subs))
	} else {
		stats += "  " + i18n.Tf("sub.stats_remote", remoteCount)
	}

	searchBox := renderSearchBox(state.FilterText, state.FilterMode, state.FilterEngine)
	header := renderHeader(stats, searchBox, state.Width)
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

func renderHeader(stats, searchBox string, width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue())
	statsStyle := lipgloss.NewStyle().Foreground(common.TokyoBlue())

	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	statsText := statsStyle.Render(stats)
	searchW := lipgloss.Width(searchBox)
	statsW := lipgloss.Width(statsText)
	gap := innerWidth - searchW - statsW
	if gap < 1 {
		gap = 1
	}
	searchRow := borderStyle.Render("│") +
		searchBox + strings.Repeat(" ", gap) + statsText +
		borderStyle.Render("│")
	// 极窄补齐
	rowContentWidth := lipgloss.Width(searchBox) + gap + statsW
	if rowContentWidth < innerWidth {
		searchRow = borderStyle.Render("│") +
			searchBox + strings.Repeat(" ", gap) + statsText +
			strings.Repeat(" ", innerWidth-rowContentWidth) +
			borderStyle.Render("│")
	}

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

func renderList(state PageState, layout subListLayout) string {
	maxLines := layout.capacity
	if len(state.FilteredIdx) == 0 {
		return common.MutedStyle().Render(i18n.T("sub.empty"))
	}

	scrollTop, endIdx := layout.window.Start, layout.window.End

	listWidth := state.Width - subScrollWidth
	var lines []string
	for i := scrollTop; i < endIdx; i++ {
		subIdx := state.FilteredIdx[i]
		if subIdx < 0 || subIdx >= len(state.Subs) {
			continue
		}
		p := state.Subs[subIdx]
		lines = append(lines, renderSubEntry(p, i+1, subIdx == 0, p.UID == state.ActiveUID, p.UID == state.UpdatingUID, i == layout.window.Selected, listWidth))
	}
	listStr := strings.Join(lines, "\n")

	if len(state.FilteredIdx) <= maxLines {
		return listStr
	}

	// 滚动条
	thumbStart, thumbEnd := common.CalcThumbRange(maxLines, len(state.FilteredIdx), scrollTop)
	barLines := make([]string, maxLines)
	for i := range barLines {
		barLines[i] = common.SymbolScrollbarTrack
	}
	for i := thumbStart; i < thumbEnd && i < maxLines; i++ {
		barLines[i] = common.SymbolScrollbarThumb
	}
	barStr := strings.Join(barLines, "\n")
	styledBar := ""
	for i, ch := range strings.Split(barStr, "\n") {
		if i >= thumbStart && i < thumbEnd {
			styledBar += common.MutedStyle().Foreground(common.Gray()).Render(ch) + "\n"
		} else {
			styledBar += common.DimStyle().Render(ch) + "\n"
		}
	}
	styledBar = strings.TrimRight(styledBar, "\n")

	fixedList := lipgloss.NewStyle().Width(listWidth).Render(listStr)
	return lipgloss.JoinHorizontal(lipgloss.Top, fixedList, styledBar)
}

func renderSubEntry(p profile.Profile, seq int, _, isActive, isUpdating, selected bool, width int) string {
	// 列宽常量（固定列宽保证各列对齐）
	const (
		colIndexW  = 5  // 序号列
		colUidW    = 8  // UID列
		colKindW   = 6  // 类型列（remote/local 最长 6）
		colTimeW   = 14 // 更新时间列
		colNameMin = 8  // 名称列最小宽度
		colSrcMin  = 16 // 来源列最小宽度
	)
	// 固定开销：选择符前缀(2) + 激活标记(2) + 列间分隔符(6) + 各固定列宽
	const fixedOverhead = 2 + 2 + 6 + colIndexW + colUidW + colKindW + colTimeW // = 43

	// 名称/来源列共享剩余宽度。URL 通常更长，来源列占 5/6、名称列占 1/8。
	flex := width - fixedOverhead
	if flex < colNameMin+colSrcMin {
		flex = colNameMin + colSrcMin
	}
	nameMax := flex * 1 / 8
	if nameMax < colNameMin {
		nameMax = colNameMin
	}
	srcMax := flex - nameMax
	if srcMax < colSrcMin {
		srcMax = colSrcMin
	}

	// 激活标记：绿色圆点表示激活，灰色空位表示未激活（不再使用 [激活] 文字徽标）
	var mark string
	if isActive {
		mark = lipgloss.NewStyle().Foreground(common.Success()).Render("● ")
	} else {
		mark = lipgloss.NewStyle().Foreground(common.Gray()).Render("  ")
	}

	// 序号
	indexStr := lipgloss.NewStyle().Foreground(common.Success()).Width(colIndexW).Render(fmt.Sprintf("%d.", seq))

	// 名称（截断）
	name := p.Name
	if common.DisplayWidth(name) > nameMax {
		name = common.TruncateDisplay(name, nameMax)
	}
	nameStr := lipgloss.NewStyle().Foreground(common.Bright()).Width(nameMax).Render(name)

	// UID
	uid := p.UID
	if common.DisplayWidth(uid) > colUidW {
		uid = common.TruncateDisplay(uid, colUidW)
	}
	uidStr := lipgloss.NewStyle().Foreground(common.TokyoMuted()).Width(colUidW).Render(uid)

	// 类型
	kindColor := common.TokyoBlue()
	if p.Source.Kind == profile.SourceLocal {
		kindColor = common.TokyoPurple()
	}
	kindStr := lipgloss.NewStyle().Foreground(kindColor).Width(colKindW).Render(p.Source.Kind.String())

	// 来源（截断）
	src := p.Source.Display()
	if common.DisplayWidth(src) > srcMax {
		src = common.TruncateDisplay(src, srcMax)
	}
	srcStr := lipgloss.NewStyle().Foreground(common.Secondary()).Width(srcMax).Render(src)

	// 更新时间（预截断，避免 lipgloss.Width 因内容超宽而自动换行破坏单行布局）
	tm := relativeTime(p.UpdatedAt)
	if common.DisplayWidth(tm) > colTimeW {
		tm = common.TruncateDisplay(tm, colTimeW)
	}
	timeStr := lipgloss.NewStyle().Foreground(common.TokyoMuted()).Width(colTimeW).Render(tm)

	var updatingBadge string
	if isUpdating {
		updatingBadge = " " + lipgloss.NewStyle().Foreground(common.TokyoCyan()).Render("["+i18n.T("sub.updating")+"]")
	}

	line := fmt.Sprintf("%s %s %s %s %s %s %s%s", mark, indexStr, nameStr, uidStr, kindStr, srcStr, timeStr, updatingBadge)
	if selected {
		line = lipgloss.NewStyle().Background(common.Highlight()).Render(common.SymbolSelectActive + line)
	} else {
		line = common.SymbolSelectInactive + line
	}
	return line
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

// resolveListItemAt 根据 pageY 定位命中的列表项索引（filteredIdx 内的索引）。
func resolveListItemAt(state PageState, pageY, pageHeight int) int {
	state.Height = pageHeight
	return layoutSubList(state).itemAt(pageY)
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
	return []common.InlineHelpHint{
		{Key: "↑↓", Desc: i18n.T("help.sub.select")},
		{Key: "↵", Desc: i18n.T("help.sub.activate")},
		{Key: "u", Desc: i18n.T("help.sub.update"), Tone: common.InlineHelpToneSuccess},
		{Key: "m", Desc: i18n.T("help.sub.edit_merge")},
		{Key: "c", Desc: i18n.T("help.sub.view_raw")},
		{Key: "e", Desc: i18n.T("help.sub.edit_profile")},
		{Key: "n", Desc: i18n.T("help.sub.add")},
		{Key: "d", Desc: i18n.T("help.sub.delete")},
		{Key: "/", Desc: i18n.T("help.sub.search")},
		{Key: "?", Desc: i18n.T("core.inline.help")},
	}
}

// ============================================================
// 弹窗叠加（暗化底层 → 居中弹窗 → 逐行嵌入）
// ============================================================

const ()

func formLabelWidth() int {
	width := 0
	for _, key := range []string{"sub.add_field_name", "sub.add_field_kind", "sub.add_field_src"} {
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
	sourceHeight = max(1, min(6, height-7))
	return
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
	modalWidth, fieldWidth, _ := formDimensions(width, height)
	form.resize(width, height)
	innerWidth := modalWidth - 4

	// 名称行
	nameRow := renderFieldRow(i18n.T("sub.add_field_name"), form.nameField.View(), form.isNameField(), innerWidth)

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
	kindRow := renderFieldRow(i18n.T("sub.add_field_kind"), kindVal, form.isKindField(), innerWidth)

	// 来源值行
	srcLabel := i18n.T("sub.add_field_src")
	srcValue := form.srcField.View()
	if button := form.copyButton(fieldWidth); button != "" {
		style := lipgloss.NewStyle().Background(common.TokyoCyan()).Foreground(common.Background()).Bold(true)
		srcValue = lipgloss.JoinHorizontal(lipgloss.Top, srcValue, " ", style.Render(button))
	}
	srcRow := renderFieldRow(srcLabel, srcValue, form.isSrcField(), innerWidth)

	// 说明/错误行
	var footer string
	if form.errMsg != "" {
		footer = lipgloss.NewStyle().Foreground(common.Danger()).Render("✗ " + form.errMsg)
	} else if form.isKindField() {
		footer = common.TokyoMutedStyle().Render(i18n.T("sub.add_kind_hint"))
	} else {
		footer = common.TokyoMutedStyle().Render(i18n.T("sub.add_src_hint"))
	}

	divider := lipgloss.NewStyle().Foreground(common.TokyoBlue()).Render(strings.Repeat("─", innerWidth))
	content := strings.Join([]string{nameRow, kindRow, srcRow, divider, footer}, "\n")

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
