package rules

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ============================================================
//  添加自定义规则弹窗（renderAddRuleOverlay / buildAddRuleModal）
// ============================================================
//
// 布局完全复刻类型筛选弹窗：暗化底层 → 居中弹窗 → 逐行嵌入。
// 表单字段使用 bubbles/textinput 的 View() 渲染（自带光标）。

// addFormModalWidth 弹窗整体宽度（含边框）。
const addFormModalWidth = 54

// addFormLayout 描述添加规则弹窗的内部布局，供渲染与鼠标命中复用。
type addFormLayout struct {
	modalWidth  int // 弹窗整体宽度（含边框）
	modalHeight int // 弹窗整体高度（含边框）
}

// computeAddFormLayout 计算弹窗尺寸，需与 buildAddRuleModal 保持一致。
func computeAddFormLayout(state PageState, width, height int) addFormLayout {
	modalWidth := addFormModalWidth
	if modalWidth > width-4 {
		modalWidth = width - 4
	}
	if modalWidth < 24 {
		modalWidth = 24
	}
	// 基础高度：上边框1 + 类型行1 + 分隔1 + 三字段3 + 分隔1 + 说明/错误1 + 下边框1 = 9
	// IP 类规则额外显示 no-resolve 复选框行 +1
	modalHeight := 9
	if state.AddForm.isIPType() {
		modalHeight = 10
	}
	if modalHeight > height-2 {
		modalHeight = height - 2
	}
	return addFormLayout{modalWidth: modalWidth, modalHeight: modalHeight}
}

// renderAddRuleOverlay 渲染添加规则弹窗叠加层（同 renderTypeFilterOverlay 结构）。
func renderAddRuleOverlay(background string, state PageState, width, height int) string {
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
	modal := buildAddRuleModal(state, width, height)
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

// buildAddRuleModal 渲染添加规则弹窗本体（无暗化背景）。
// 尺寸计算与 computeAddFormLayout 必须一致。
func buildAddRuleModal(state PageState, width, height int) string {
	layout := computeAddFormLayout(state, width, height)
	innerWidth := layout.modalWidth - 4 // 减去左右边框 + padding
	if innerWidth < 10 {
		innerWidth = 10
	}

	form := state.AddForm

	// ── 类型行：◀ TYPE ▶（可聚焦，聚焦时整体高亮 + ←/→ 切换提示）──
	typeRow := renderAddFormTypeRow(form, innerWidth)

	// ── 分隔行 ──
	divider := lipgloss.NewStyle().Foreground(common.TokyoBlue()).Render(strings.Repeat("─", innerWidth))

	// ── 字段渲染 ──
	// MATCH 无 payload，省略匹配值字段
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
		footer = common.TokyoMutedStyle().Render(i18n.T("rules.add_type_hint"))
	} else if form.isProxyField() {
		// 策略行聚焦时提示 ←/→ 切换、来源为源配置文件
		footer = common.TokyoMutedStyle().Render(i18n.T("rules.add_proxy_hint"))
	} else if form.isNoResolveField() {
		footer = common.TokyoMutedStyle().Render(i18n.T("rules.add_noresolve_hint"))
	} else {
		footer = common.TokyoMutedStyle().Render(i18n.T("rules.add_index_hint"))
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
		i18n.T("rules.add_title"),
		modalContent,
		layout.modalWidth,
		common.TokyoBlue(),
		common.TokyoForeground(),
	)
}

// renderAddFormFieldRow 渲染单个表单字段行（label + textinput.View）。
// skip=true 时返回空字符串（用于 MATCH 类型省略 payload 字段）。
// 聚焦行整体填充 TokyoSelected 背景（同顶部导航栏选中态），跨整行宽度，便于上下切换时辨识。
func renderAddFormFieldRow(form addForm, fieldIdx int, label string, innerWidth int, skip bool) string {
	if skip {
		return ""
	}
	focused := form.fieldCursor == fieldIdx

	// 标签：聚焦时用青色（参考导航栏选中态前景），否则弱化色
	labelStyle := common.TokyoMutedStyle()
	if focused {
		labelStyle = lipgloss.NewStyle().Foreground(common.TokyoCyan())
	}
	labelText := labelStyle.Render(label)

	// textinput 的 View() 自带光标
	field := form.fields[fieldIdx]
	inputView := field.View()
	if focused {
		inputView = lipgloss.NewStyle().Foreground(common.TokyoCyan()).Render(inputView)
	} else {
		inputView = lipgloss.NewStyle().Foreground(common.TokyoForeground()).Render(inputView)
	}

	return applyFieldRowBackground(labelText+" "+inputView, focused, innerWidth)
}

// renderAddFormTypeRow 渲染类型选择器行：类型: ◀ TYPE ▶。
// 类型值颜色取自 ruleTypeColors；聚焦时标签青色 + 类型值加粗青色，
// 整行铺满 TokyoSelected 背景（与 payload/proxy 行视觉一致）。
func renderAddFormTypeRow(form addForm, innerWidth int) string {
	focused := form.isTypeField()

	labelStyle := common.TokyoMutedStyle()
	if focused {
		labelStyle = lipgloss.NewStyle().Foreground(common.TokyoCyan())
	}
	labelText := labelStyle.Render(i18n.T("rules.add_field_type"))

	// 类型值颜色：聚焦时用青色强调（盖过类型配色），否则用类型自身配色
	var typeVal string
	if focused {
		typeVal = lipgloss.NewStyle().Foreground(common.TokyoCyan()).Bold(true).
			Render(fmt.Sprintf("◀ %s ▶", form.currentType()))
	} else {
		typeColor := getAdjustedRuleTypeColor(form.currentType(), nil)
		typeVal = lipgloss.NewStyle().Foreground(typeColor).Bold(true).
			Render(fmt.Sprintf("◀ %s ▶", form.currentType()))
	}
	return applyFieldRowBackground(labelText+" "+typeVal, focused, innerWidth)
}

// renderAddFormProxyRow 渲染策略选择器行：策略: NAME (Enter 改)。
// 策略来源颜色区分：策略组（TokyoBlue）/ 具体节点（TokyoGreen）。
// 聚焦行整体填充 TokyoSelected 背景，跨整行宽度。
func renderAddFormProxyRow(form addForm, innerWidth int) string {
	focused := form.isProxyField()

	labelStyle := common.TokyoMutedStyle()
	if focused {
		labelStyle = lipgloss.NewStyle().Foreground(common.TokyoCyan())
	}
	labelText := labelStyle.Render(i18n.T("rules.add_field_proxy"))

	// 策略值颜色：策略组用蓝，节点用绿
	valColor := common.TokyoBlue()
	if !form.isProxyGroupSelected() {
		valColor = common.TokyoGreen()
	}
	val := form.currentProxy()
	var valView string
	if focused {
		valView = lipgloss.NewStyle().Foreground(valColor).Bold(true).Render(val)
		valView += " " + common.TokyoMutedStyle().Render(i18n.T("rules.add_proxy_change"))
	} else {
		valView = lipgloss.NewStyle().Foreground(common.TokyoForeground()).Render(val)
	}
	return applyFieldRowBackground(labelText+" "+valView, focused, innerWidth)
}

// renderAddFormNoResolveRow 渲染 no-resolve 复选框行。
// 仅当规则类型为 IP-CIDR/IP-CIDR6/SRC-IP-CIDR 时显示。
// 聚焦行整体填充 TokyoSelected 背景，跨整行宽度。
func renderAddFormNoResolveRow(form addForm, innerWidth int) string {
	focused := form.isNoResolveField()

	labelStyle := common.TokyoMutedStyle()
	if focused {
		labelStyle = lipgloss.NewStyle().Foreground(common.TokyoCyan())
	}
	labelText := labelStyle.Render(i18n.T("rules.add_field_noresolve"))

	checkbox := "[ ]"
	if form.noResolve {
		checkbox = "[✓]"
	}

	var valView string
	if focused {
		valView = lipgloss.NewStyle().Foreground(common.TokyoCyan()).Bold(true).Render(checkbox)
	} else {
		valView = lipgloss.NewStyle().Foreground(common.TokyoForeground()).Render(checkbox)
	}
	return applyFieldRowBackground(labelText+" "+valView, focused, innerWidth)
}

// applyFieldRowBackground 为聚焦的表单字段行铺满整行 TokyoSelected 背景，
// 并在左侧加 1 个空格前导与右侧用空格补齐至 innerWidth，使高亮条对齐稳定。
// 非聚焦行原样返回。背景色与顶部导航栏选中项保持一致（#292E42）。
func applyFieldRowBackground(row string, focused bool, innerWidth int) string {
	if !focused {
		return row
	}
	// 左侧加一个空格作为光标条视觉间隔，便于辨识当前编辑项
	styled := " " + row
	// 补齐至 innerWidth：内容显示宽度 + 1（前导空格）
	pad := innerWidth - 1 - common.DisplayWidth(row)
	if pad < 0 {
		pad = 0
	}
	return lipgloss.NewStyle().Background(common.TokyoSelected()).Render(styled + strings.Repeat(" ", pad))
}

// ResolveAddFormBounds 返回添加规则弹窗在页面坐标系中的边界（右下为开区间）。
// 通过渲染真实弹窗取尺寸，确保与 renderAddRuleOverlay 位置完全一致。
func ResolveAddFormBounds(state PageState, width, height int) (left, top, right, bottom int) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0
	}
	modal := buildAddRuleModal(state, width, height)
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
