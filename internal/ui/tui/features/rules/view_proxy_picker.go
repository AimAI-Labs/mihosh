package rules

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ============================================================
//  策略选择二级弹窗（renderProxyPickerOverlay / buildProxyPickerModal）
// ============================================================
//
// 镜像类型筛选弹窗：暗化底层 → 居中弹窗 → 逐行嵌入。
// 内部布局：Tab 行 + 搜索框 + 可滚动列表 + 统计栏。

// proxyPickerModalWidth 弹窗整体宽度（含边框）。
const proxyPickerModalWidth = 52

// proxyPickerLayout 描述策略弹窗的内部布局，供渲染与鼠标命中复用。
type proxyPickerLayout struct {
	modalWidth   int // 弹窗整体宽度（含边框）
	modalHeight  int // 弹窗整体高度（含边框）
	listStartY   int // 列表第一行相对弹窗顶部的偏移
	listHeight   int // 列表可见行数
	visibleStart int // 列表可见起始索引（含滚动）
}

// computeProxyPickerLayout 计算弹窗尺寸与列表布局。
// 必须与 buildProxyPickerModal 保持完全一致。
func computeProxyPickerLayout(state PageState, width, height int) proxyPickerLayout {
	modalWidth := proxyPickerModalWidth
	if modalWidth > width-4 {
		modalWidth = width - 4
	}
	if modalWidth < 24 {
		modalWidth = 24
	}

	form := state.AddForm
	filtered := form.pickerFiltered()
	total := len(filtered)

	// 总高 = 上边框1 + Tab行1 + 搜索行1 + 列表listHeight + 统计行1 + 下边框1 = listHeight + 5
	listHeight := total
	if listHeight > addPickerListMax {
		listHeight = addPickerListMax
	}
	if listHeight < 3 {
		listHeight = 3
	}
	modalHeight := listHeight + 5
	if modalHeight > height-4 {
		modalHeight = height - 4
		listHeight = modalHeight - 5
		if listHeight < 3 {
			listHeight = 3
			modalHeight = 8
		}
	}

	// 列表在弹窗内的起始行（上边框1 + Tab行1 + 搜索行1 = 3）
	listStartY := 3

	// 滚动偏移
	visibleStart := form.pickerScrollTop
	if visibleStart < 0 {
		visibleStart = 0
	}
	if visibleStart > total-listHeight && total > listHeight {
		visibleStart = total - listHeight
	}
	if visibleStart < 0 {
		visibleStart = 0
	}

	return proxyPickerLayout{
		modalWidth:   modalWidth,
		modalHeight:  modalHeight,
		listStartY:   listStartY,
		listHeight:   listHeight,
		visibleStart: visibleStart,
	}
}

// renderProxyPickerOverlay 渲染策略选择弹窗叠加层（复用暗化+从顶部开始+嵌入三步）。
func renderProxyPickerOverlay(background string, state PageState, width, height int) string {
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

	modal := buildProxyPickerModal(state, width, height)
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

// buildProxyPickerModal 渲染策略选择弹窗本体（无暗化背景）。
func buildProxyPickerModal(state PageState, width, height int) string {
	layout := computeProxyPickerLayout(state, width, height)
	form := state.AddForm

	// ── Tab 行 ──
	tabGroup := i18n.T("rules.picker_tab_group")
	tabNode := i18n.T("rules.picker_tab_node")
	activeStyle := lipgloss.NewStyle().Foreground(common.TokyoCyan()).Bold(true).Underline(true)
	inactiveStyle := common.TokyoMutedStyle()
	var tabRow string
	if form.pickerTab == addPickerTabGroup {
		tabRow = activeStyle.Render("▸"+tabGroup) + "  " + inactiveStyle.Render(tabNode)
	} else {
		tabRow = inactiveStyle.Render(tabGroup) + "  " + activeStyle.Render("▸"+tabNode)
	}

	// ── 搜索行 ──
	searchLabel := common.TokyoMutedStyle().Render(i18n.T("rules.picker_search"))
	searchVal := lipgloss.NewStyle().Foreground(common.Bright()).Render(form.pickerSearch + "█")
	searchRow := searchLabel + searchVal

	// ── 列表 ──
	filtered := form.pickerFiltered()
	total := len(filtered)
	visibleStart := layout.visibleStart
	visibleEnd := visibleStart + layout.listHeight
	if visibleEnd > total {
		visibleEnd = total
	}

	var listLines []string
	for i := visibleStart; i < visibleEnd; i++ {
		name := filtered[i]
		isCurrentSelected := name == form.proxySelected
		isGroup := false
		for _, g := range form.proxyGroups {
			if g == name {
				isGroup = true
				break
			}
		}
		nameColor := common.TokyoBlue()
		if !isGroup {
			nameColor = common.TokyoGreen()
		}

		if i == form.pickerCursor {
			cursorStyle := lipgloss.NewStyle().Background(common.TokyoSelected()).Foreground(common.TokyoCyan())
			var mark string
			if isCurrentSelected {
				mark = lipgloss.NewStyle().Foreground(common.TokyoGreen()).Render("✓ ")
			} else {
				mark = "  "
			}
			nameStyle := lipgloss.NewStyle().Foreground(nameColor).Bold(true)
			listLines = append(listLines, cursorStyle.Render(" "+mark+nameStyle.Render(name)+" "))
		} else {
			var mark string
			if isCurrentSelected {
				mark = lipgloss.NewStyle().Foreground(common.TokyoGreen()).Render("✓ ")
			} else {
				mark = "  "
			}
			nameStyle := lipgloss.NewStyle().Foreground(nameColor)
			listLines = append(listLines, " "+mark+nameStyle.Render(name))
		}
	}

	// 填充空行
	for len(listLines) < layout.listHeight {
		listLines = append(listLines, "")
	}
	listStr := strings.Join(listLines, "\n")

	// ── 统计栏 ──
	var statsText string
	if total == 0 {
		statsText = common.TokyoMutedStyle().Render(i18n.T("rules.picker_empty"))
	} else {
		statsText = common.TokyoMutedStyle().Render(
			i18n.Tf("rules.picker_stats", form.proxySelected, total, len(form.pickerCandidates())),
		)
	}

	// ── 组装 ──
	modalContent := lipgloss.JoinVertical(lipgloss.Left,
		tabRow,
		searchRow,
		listStr,
		statsText,
	)

	return common.RenderBorderedPanel(
		i18n.T("rules.picker_title"),
		modalContent,
		layout.modalWidth,
		common.TokyoBlue(),
		common.TokyoForeground(),
	)
}

// ResolveProxyPickerBounds 返回策略选择弹窗在页面坐标系中的边界（右下为开区间）。
func ResolveProxyPickerBounds(state PageState, width, height int) (left, top, right, bottom int) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0
	}
	modal := buildProxyPickerModal(state, width, height)
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

// ResolveProxyPickerListItemAt 判断页面坐标 (pageX, pageY) 命中的策略列表项索引。
// 若不在列表行上则返回 -1。
func ResolveProxyPickerListItemAt(state PageState, pageX, pageY, width, height int) int {
	left, top, right, bottom := ResolveProxyPickerBounds(state, width, height)
	if pageX < left || pageX >= right || pageY < top || pageY >= bottom {
		return -1
	}
	layout := computeProxyPickerLayout(state, width, height)

	listTopRow := top + layout.listStartY
	listBottomRow := listTopRow + layout.listHeight
	if pageY < listTopRow || pageY >= listBottomRow {
		return -1
	}

	rowOffset := pageY - listTopRow
	idx := layout.visibleStart + rowOffset
	filtered := state.AddForm.pickerFiltered()
	if idx < 0 || idx >= len(filtered) {
		return -1
	}
	return idx
}
