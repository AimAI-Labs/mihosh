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
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	subFixedLines   = 5 // header(3) + 间距(1) + 底部(1)
	subMinHeight    = 5
	subScrollWidth  = 1
	subHeaderHeight = 3
	// subVisibleLinesHint 列表可视行数估算（渲染时按实际高度重算，此处仅用于滚动钳制）。
	subVisibleLinesHint = 10
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
	ShowMergeEdit  bool
	MergeEditor    mergeEditor
	ShowDeleteConf bool
	DeleteUID      string
	UpdatingUID    string
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

	availableHeight := state.Height - subFixedLines
	if availableHeight < subMinHeight {
		availableHeight = subMinHeight
	}
	list := renderList(state, availableHeight)
	sections = append(sections, list)

	result := strings.Join(sections, "\n")

	if state.ShowAddForm {
		result = renderAddFormOverlay(result, state, state.Width, state.Height)
	}
	if state.ShowEditForm {
		result = renderEditFormOverlay(result, state, state.Width, state.Height)
	}
	if state.ShowMergeEdit {
		result = renderMergeEditorOverlay(result, state, state.Width, state.Height)
	}
	if state.ShowDeleteConf {
		result = renderDeleteConfirmOverlay(result, state, state.Width, state.Height)
	}

	return renderInlineHelp(result, state)
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

	top := borderStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")
	bottom := borderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")
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
	var engineIndicator string
	switch engine {
	case FilterRegex:
		engineIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7")).Render(" [RE]")
	case FilterFuzzy:
		engineIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render(" [Fuzzy]")
	}
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

func renderList(state PageState, maxLines int) string {
	if len(state.FilteredIdx) == 0 {
		return common.MutedStyle().Render(i18n.T("sub.empty"))
	}

	scrollTop := state.ScrollTop
	if state.Selected < scrollTop {
		scrollTop = state.Selected
	}
	if state.Selected >= scrollTop+maxLines {
		scrollTop = state.Selected - maxLines + 1
	}

	endIdx := scrollTop + maxLines
	if endIdx > len(state.FilteredIdx) {
		endIdx = len(state.FilteredIdx)
	}

	listWidth := state.Width - subScrollWidth
	var lines []string
	for i := scrollTop; i < endIdx; i++ {
		subIdx := state.FilteredIdx[i]
		if subIdx < 0 || subIdx >= len(state.Subs) {
			continue
		}
		p := state.Subs[subIdx]
		lines = append(lines, renderSubEntry(p, i+1, subIdx==0, p.UID == state.ActiveUID, p.UID == state.UpdatingUID, i == state.Selected, listWidth))
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
	// 激活标记
	var mark string
	if isActive {
		mark = lipgloss.NewStyle().Foreground(common.Success()).Render("● ")
	} else {
		mark = lipgloss.NewStyle().Foreground(common.Gray()).Render("  ")
	}

	// 序号
	indexStr := lipgloss.NewStyle().Foreground(common.Success()).Width(4).Render(fmt.Sprintf("%d.", seq))

	// 名称
	nameStr := lipgloss.NewStyle().Foreground(common.Bright()).Render(p.Name)

	// 激活徽标
	var badge string
	if isActive {
		badge = " " + lipgloss.NewStyle().Foreground(common.TokyoGreen()).Render("["+i18n.T("sub.badge_active")+"]")
	}

	var updatingBadge string
	if isUpdating {
		updatingBadge = " " + lipgloss.NewStyle().Foreground(common.TokyoCyan()).Render("["+i18n.T("sub.updating")+"]")
	}

	// 类型
	kindColor := common.TokyoBlue()
	if p.Source.Kind == profile.SourceLocal {
		kindColor = common.TokyoPurple()
	}
	kindStr := lipgloss.NewStyle().Foreground(kindColor).Render(p.Source.Kind.String())

	// 来源（截断）
	srcMax := width - 40
	if srcMax < 16 {
		srcMax = 16
	}
	src := p.Source.Display()
	if len([]rune(src)) > srcMax {
		src = string([]rune(src)[:srcMax-3]) + "..."
	}
	srcStr := lipgloss.NewStyle().Foreground(common.Secondary()).Render(src)

	// 更新时间
	timeStr := lipgloss.NewStyle().Foreground(common.TokyoMuted()).Render(relativeTime(p.UpdatedAt))

	line := fmt.Sprintf("%s%s %s%s %s %s %s%s", mark, indexStr, nameStr, badge, kindStr, srcStr, timeStr, updatingBadge)
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
	listStartY := subHeaderHeight + 1
	if pageY < listStartY {
		return -1
	}
	availableHeight := pageHeight - subFixedLines
	if availableHeight < subMinHeight {
		availableHeight = subMinHeight
	}
	offset := pageY - listStartY
	if offset < 0 || offset >= availableHeight {
		return -1
	}
	visibleIdx := state.ScrollTop + offset
	if visibleIdx < 0 || visibleIdx >= len(state.FilteredIdx) || offset >= availableHeight {
		return -1
	}
	return visibleIdx
}

// ============================================================
// 内联帮助（右下角浮层）
// ============================================================

func renderInlineHelp(page string, state PageState) string {
	hints := buildHints(state)
	if len(hints) == 0 {
		return page
	}
	body := common.FormatInlineHintRow(hints)
	return common.OverlayHelpAtBottomRight(page, body, state.Width, state.Height)
}

func buildHints(state PageState) []common.InlineHelpHint {
	if state.ShowMergeEdit {
		return []common.InlineHelpHint{
			{Key: "Ctrl+S", Desc: i18n.T("help.sub_merge.save")},
			{Key: "Esc", Desc: i18n.T("help.sub_merge.cancel")},
		}
	}
	if state.ShowDeleteConf {
		return []common.InlineHelpHint{
			{Key: "Enter/y", Desc: i18n.T("help.sub_delete.confirm")},
			{Key: "Esc/n", Desc: i18n.T("help.sub_delete.cancel")},
		}
	}
	if state.ShowAddForm || state.ShowEditForm {
		return []common.InlineHelpHint{
			{Key: "↑↓/Tab", Desc: i18n.T("help.sub_add.field")},
			{Key: "Enter", Desc: i18n.T("help.sub_add.confirm")},
			{Key: "Esc", Desc: i18n.T("help.sub_add.cancel")},
		}
	}
	if state.FilterMode {
		return []common.InlineHelpHint{
			{Key: "Enter", Desc: i18n.T("help.sub_search.confirm")},
			{Key: "Esc", Desc: i18n.T("help.sub_search.cancel")},
			{Key: "⌫", Desc: i18n.T("help.sub_search.backspace")},
			{Key: "Ctrl+R/F", Desc: i18n.T("help.rules_search.regex_fuzzy")},
		}
	}
	return []common.InlineHelpHint{
		{Key: "↑↓", Desc: i18n.T("help.sub.select")},
		{Key: "Enter", Desc: i18n.T("help.sub.activate")},
		{Key: "u", Desc: i18n.T("help.sub.update")},
		{Key: "m", Desc: i18n.T("help.sub.edit_merge")},
		{Key: "e", Desc: i18n.T("help.sub.edit_profile")},
		{Key: "n", Desc: i18n.T("help.sub.add")},
		{Key: "d", Desc: i18n.T("help.sub.delete")},
		{Key: "/", Desc: i18n.T("help.sub.search")},
	}
}

// ============================================================
// 弹窗叠加（暗化底层 → 居中弹窗 → 逐行嵌入）
// ============================================================

const (
	addFormModalWidth      = 54
	mergeEditorModalWidth  = 70
	mergeEditorModalHeight = 18
	deleteModalWidth       = 54
)

// overlayDim 底层暗化 + 弹窗居中嵌入的通用实现。
func overlayDim(background string, modal string, width, height int) string {
	baseLines := strings.Split(background, "\n")
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

	modalLines := strings.Split(modal, "\n")
	if len(modalLines) == 0 {
		return strings.Join(dimmed, "\n")
	}
	modalWidth := lipgloss.Width(modalLines[0])
	leftOffset := (width - modalWidth) / 2
	if leftOffset < 0 {
		leftOffset = 0
	}
	topOffset := subHeaderHeight + 1
	if topOffset < 0 {
		topOffset = 0
	}
	if topOffset+len(modalLines) > height {
		topOffset = height - len(modalLines)
		if topOffset < 0 {
			topOffset = 0
		}
	}

	for i, pl := range modalLines {
		y := topOffset + i
		if y >= height {
			break
		}
		leftPart := ansi.Cut(dimmed[y], 0, leftOffset)
		leftW := lipgloss.Width(leftPart)
		if leftW < leftOffset {
			leftPart += strings.Repeat(" ", leftOffset-leftW)
		}
		rightPart := ansi.Cut(dimmed[y], leftOffset+modalWidth, width)
		dimmed[y] = leftPart + pl + rightPart
	}
	return strings.Join(dimmed, "\n")
}

// ===== 添加/编辑表单（共用渲染） =====

func renderAddFormOverlay(background string, state PageState, width, height int) string {
	return overlayDim(background, buildAddFormModal(state, width, height), width, height)
}

func renderEditFormOverlay(background string, state PageState, width, height int) string {
	return overlayDim(background, buildEditFormModal(state, width, height), width, height)
}

func buildAddFormModal(state PageState, width, height int) string {
	return buildFormModal(i18n.T("sub.add_title"), state.AddForm, width, height)
}

func buildEditFormModal(state PageState, width, height int) string {
	return buildFormModal(i18n.T("sub.edit_title"), state.EditForm, width, height)
}

// buildFormModal 渲染订阅表单弹窗（添加/编辑共用，仅标题不同）。
func buildFormModal(title string, form addForm, width, height int) string {
	modalWidth := addFormModalWidth
	if modalWidth > width-4 {
		modalWidth = width - 4
	}
	if modalWidth < 30 {
		modalWidth = 30
	}
	innerWidth := modalWidth - 4
	if innerWidth < 10 {
		innerWidth = 10
	}

	// 名称行
	nameRow := renderFieldRow(i18n.T("sub.add_field_name"), form.fields[addFieldName].View(), form.isNameField(), innerWidth)

	// 类型行（◀ remote ▶）
	kindLabel := "remote"
	if !form.kindRemote {
		kindLabel = "local"
	}
	kindVal := fmt.Sprintf("◀ %s ▶", kindLabel)
	kindRow := renderFieldRow(i18n.T("sub.add_field_kind"), kindVal, form.isKindField(), innerWidth)

	// 来源值行
	srcRow := renderFieldRow(i18n.T("sub.add_field_src"), form.fields[addFieldSrc].View(), form.isSrcField(), innerWidth)

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
	row := labelStyle.Render(label) + " " + value
	if !focused {
		return row
	}
	styled := " " + row
	pad := innerWidth - 1 - common.DisplayWidth(row)
	if pad < 0 {
		pad = 0
	}
	return lipgloss.NewStyle().Background(common.TokyoSelected()).Render(styled + strings.Repeat(" ", pad))
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
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)
	leftGap := width - modalWidth
	if leftGap < 0 {
		leftGap = 0
	}
	left = leftGap / 2
	top = subHeaderHeight + 1
	if top < 0 {
		top = 0
	}
	if top+modalHeight > height {
		top = height - modalHeight
		if top < 0 {
			top = 0
		}
	}
	right = left + modalWidth
	bottom = top + modalHeight
	return
}

// ===== merge 编辑器 =====

func renderMergeEditorOverlay(background string, state PageState, width, height int) string {
	return overlayDim(background, buildMergeEditorModal(state, width, height), width, height)
}

func buildMergeEditorModal(state PageState, width, height int) string {
	modalWidth := mergeEditorModalWidth
	if modalWidth > width-4 {
		modalWidth = width - 4
	}
	if modalWidth < 40 {
		modalWidth = 40
	}
	modalHeight := mergeEditorModalHeight
	if modalHeight > height-4 {
		modalHeight = height - 4
	}
	if modalHeight < 8 {
		modalHeight = 8
	}
	innerWidth := modalWidth - 4
	if innerWidth < 10 {
		innerWidth = 10
	}

	editor := state.MergeEditor
	// 调整 textarea 高度适配弹窗，尺寸设置后再 flush pending 内容
	// （textarea.SetValue 依赖 viewport 初始化，需在 SetWidth/SetHeight 之后调用）
	editor.editor.SetWidth(innerWidth)
	editor.editor.SetHeight(modalHeight - 6)
	editor = editor.flushPending()

	// 合法键提示
	hint := common.TokyoMutedStyle().Render(i18n.T("sub.merge_keys_hint"))

	var status string
	if !editor.ready {
		status = common.TokyoMutedStyle().Render(i18n.T("sub.merge_loading"))
	} else if editor.errMsg != "" {
		status = lipgloss.NewStyle().Foreground(common.Warning()).Render(editor.errMsg)
	} else {
		status = common.TokyoMutedStyle().Render(i18n.T("sub.merge_ready"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		hint,
		editor.editor.View(),
		status,
	)

	return common.RenderBorderedPanel(
		i18n.T("sub.merge_title"),
		content,
		modalWidth,
		common.TokyoBlue(),
		common.TokyoForeground(),
	)
}

// ===== 删除确认 =====

func renderDeleteConfirmOverlay(background string, state PageState, width, height int) string {
	return overlayDim(background, buildDeleteConfirmModal(state, width, height), width, height)
}

func buildDeleteConfirmModal(state PageState, width, height int) string {
	modalWidth := deleteModalWidth
	if modalWidth > width-4 {
		modalWidth = width - 4
	}
	if modalWidth < 36 {
		modalWidth = 36
	}
	innerWidth := modalWidth - 4
	if innerWidth < 10 {
		innerWidth = 10
	}

	// 查找待删除订阅信息
	var name, srcStr string
	isActive := false
	for _, p := range state.Subs {
		if p.UID == state.DeleteUID {
			name = p.Name
			srcStr = p.Source.Kind.String() + " " + p.Source.Display()
			isActive = p.UID == state.ActiveUID
			break
		}
	}

	nameLine := lipgloss.NewStyle().Foreground(common.Bright()).Render(name)
	srcLine := lipgloss.NewStyle().Foreground(common.Secondary()).Render(srcStr)

	var warn string
	if isActive {
		warn = lipgloss.NewStyle().Foreground(common.Warning()).Render(i18n.T("sub.delete_warn_active"))
	}

	divider := lipgloss.NewStyle().Foreground(common.TokyoBlue()).Render(strings.Repeat("─", innerWidth))
	prompt := common.TokyoMutedStyle().Render(i18n.T("sub.delete_confirm_hint"))

	rows := []string{"", nameLine, srcLine}
	if isActive {
		rows = append(rows, warn)
	}
	rows = append(rows, "", divider, prompt)
	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	return common.RenderBorderedPanel(
		i18n.T("sub.delete_title"),
		content,
		modalWidth,
		common.Danger(),
		common.TokyoForeground(),
	)
}
