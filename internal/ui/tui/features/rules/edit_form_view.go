package rules

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ============================================================
//  编辑规则弹窗（renderEditRuleOverlay / buildEditRuleModal）
// ============================================================
//
//  编辑弹窗复用 addForm 渲染（与添加弹窗结构完全一致），仅标题和底部提示文案不同。
//  标题使用 "rules.edit_title"，底部提示使用 "rules.edit_*" 系列。

// renderEditRuleOverlay 渲染编辑规则弹窗叠加层（与 renderAddRuleOverlay 结构一致）。
func renderEditRuleOverlay(background string, state PageState, width, height int) string {
	// ── 1. 暗化底层 ──
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

	// ── 2. 弹窗从顶部开始计算 ──
	modal := buildEditRuleModal(state, width, height)
	modalLines := strings.Split(modal, "\n")
	modalHeight := len(modalLines)
	if modalHeight == 0 {
		return strings.Join(dimmed, "\n")
	}

	modalWidth := lipgloss.Width(modalLines[0])
	leftOffset := (width - modalWidth) / 2
	if leftOffset < 0 {
		leftOffset = 0
	}
	topOffset := rulesHeaderHeight + 1 // 从规则头部下方开始，避免视觉疲劳
	if topOffset < 0 {
		topOffset = 0
	}
	// 确保弹窗底部不会超出屏幕
	if topOffset+modalHeight > height {
		topOffset = height - modalHeight
		if topOffset < 0 {
			topOffset = 0
		}
	}

	// ── 3. 弹窗行嵌入暗化底层 ──
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

// buildEditRuleModal 渲染编辑规则弹窗本体（与 buildAddRuleModal 结构一致，仅标题/提示不同）。
func buildEditRuleModal(state PageState, width, height int) string {
	layout := computeAddFormLayout(state, width, height)
	innerWidth := layout.modalWidth - 4
	if innerWidth < 10 {
		innerWidth = 10
	}

	form := state.AddForm

	// ── 类型行 ──
	typeRow := renderAddFormTypeRow(form, innerWidth)

	// ── 分隔行 ──
	divider := lipgloss.NewStyle().Foreground(common.TokyoBlue()).Render(strings.Repeat("─", innerWidth))

	// ── 字段渲染 ──
	payloadRow := renderAddFormFieldRow(form, addFieldPayload, i18n.T("rules.add_field_payload"), innerWidth, form.isMatchType())
	proxyRow := renderAddFormProxyRow(form, innerWidth)
	indexRow := renderAddFormFieldRow(form, addFieldIndex, i18n.T("rules.add_field_index"), innerWidth, false)

	// ── 第二分隔行 ──
	divider2 := divider

	// ── no-resolve 复选框行（仅 IP 类规则显示）──
	var noResolveRow string
	if form.isIPType() {
		noResolveRow = renderAddFormNoResolveRow(form, innerWidth)
	}

	// ── 说明/错误行 ──
	var footer string
	if form.errMsg != "" {
		footer = lipgloss.NewStyle().Foreground(common.Danger()).Render("✗ " + form.errMsg)
	} else if form.isTypeField() {
		footer = common.TokyoMutedStyle().Render(i18n.T("rules.edit_type_hint"))
	} else if form.isProxyField() {
		footer = common.TokyoMutedStyle().Render(i18n.T("rules.edit_proxy_hint"))
	} else if form.isNoResolveField() {
		footer = common.TokyoMutedStyle().Render(i18n.T("rules.edit_noresolve_hint"))
	} else {
		footer = common.TokyoMutedStyle().Render(i18n.T("rules.edit_index_hint"))
	}

	var contentRows []string
	contentRows = append(contentRows, typeRow, divider)
	if payloadRow != "" {
		contentRows = append(contentRows, payloadRow)
	}
	contentRows = append(contentRows, proxyRow, indexRow)
	if noResolveRow != "" {
		contentRows = append(contentRows, noResolveRow)
	}
	contentRows = append(contentRows, divider2, footer)

	modalContent := strings.Join(contentRows, "\n")

	return common.RenderBorderedPanel(
		i18n.T("rules.edit_title"),
		modalContent,
		layout.modalWidth,
		common.TokyoBlue(),
		common.TokyoForeground(),
	)
}

// ResolveEditFormBounds 返回编辑规则弹窗在页面坐标系中的边界（右下为开区间）。
func ResolveEditFormBounds(state PageState, width, height int) (left, top, right, bottom int) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0
	}
	modal := buildEditRuleModal(state, width, height)
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)

	leftGap := width - modalWidth
	if leftGap < 0 {
		leftGap = 0
	}
	left = leftGap / 2
	top = rulesHeaderHeight + 1
	if top < 0 {
		top = 0
	}
	// 确保弹窗底部不会超出屏幕
	if top+modalHeight > height {
		top = height - modalHeight
		if top < 0 {
			top = 0
		}
	}
	right = left + modalWidth
	bottom = top + modalHeight
	return left, top, right, bottom
}

// ResolveFormFieldAt 判断页面坐标 (pageX, pageY) 命中的表单字段索引。
// 返回 addFieldType/addFieldPayload/addFieldProxy/addFieldIndex/addFieldNoResolve 之一，
// 或 -1 表示未命中任何字段。
func ResolveFormFieldAt(state PageState, pageX, pageY, width, height int) int {
	form := state.AddForm

	// 确定弹窗边界
	var left, top, right, bottom int
	if state.ShowEditForm {
		left, top, right, bottom = ResolveEditFormBounds(state, width, height)
	} else if state.ShowAddForm {
		left, top, right, bottom = ResolveAddFormBounds(state, width, height)
	} else {
		return -1
	}

	// 点击在弹窗外
	if pageX < left || pageX >= right || pageY < top || pageY >= bottom {
		return -1
	}

	// 弹窗内相对坐标
	relY := pageY - top

	// 构建字段行的 Y 偏移映射（与 buildAddRuleModal/buildEditRuleModal 布局一致）
	// 行 0: 上边框
	// 行 1: 类型行 (addFieldType)
	// 行 2: 分隔行
	// 行 3: payload (addFieldPayload, 仅非 MATCH)
	// 行 4: proxy (addFieldProxy)
	// 行 5: index (addFieldIndex)
	// 行 6: 分隔行 2
	// 行 7: no-resolve (addFieldNoResolve, 仅 IP 类型)
	// 行 8: 说明/错误
	// 行 9: 下边框

	// 构建可见字段的行号映射
	rowToField := make(map[int]int)
	currentRow := 1 // 类型行从第 1 行开始（0 是边框）

	// 类型行始终可见
	rowToField[currentRow] = addFieldType
	currentRow += 2 // +1 类型行, +1 分隔行

	// payload 行（仅非 MATCH 类型）
	if !form.isMatchType() {
		rowToField[currentRow] = addFieldPayload
		currentRow++
	}

	// proxy 行
	rowToField[currentRow] = addFieldProxy
	currentRow++

	// index 行
	rowToField[currentRow] = addFieldIndex
	currentRow++

	// 分隔行 2
	currentRow++

	// no-resolve 行（仅 IP 类型）
	if form.isIPType() {
		rowToField[currentRow] = addFieldNoResolve
		currentRow++
	}

	// 检查点击落在哪个字段行
	if fieldIdx, ok := rowToField[relY]; ok {
		return fieldIdx
	}

	return -1
}
