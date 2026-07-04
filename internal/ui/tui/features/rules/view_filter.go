package rules

import (
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// renderTypeFilterOverlay 渲染类型筛选弹窗叠加层。
//
// 步骤：
//  1. 对背景每行整体套 Faint，让底层内容降亮（半透明效果）
//  2. 渲染弹窗本体 buildTypeFilterModal，并计算居中偏移
//  3. 逐行用 ansi.Cut 截取底层的左右两侧，将弹窗内容嵌入中间
func renderTypeFilterOverlay(background string, state PageState, width, height int) string {
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
	modal := buildTypeFilterModal(state, width, height)
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

// typeFilterModalLayout 描述弹窗的内部布局，供渲染与鼠标命中复用
type typeFilterModalLayout struct {
	modalWidth   int // 弹窗整体宽度（含边框）
	modalHeight  int // 弹窗整体高度（含边框）
	listStartY   int // 列表第一行相对弹窗顶部的偏移
	listHeight   int // 列表可见行数
	visibleStart int // 列表可见起始索引（含滚动）
}

// computeTypeFilterModalLayout 计算弹窗尺寸与列表布局。
// 该尺寸计算必须与 buildTypeFilterModal 保持完全一致，否则鼠标命中会错位。
func computeTypeFilterModalLayout(state PageState, width, height int) typeFilterModalLayout {
	// 弹窗宽度（与 buildTypeFilterModal 保持一致）
	modalWidth := 50
	if modalWidth > width-4 {
		modalWidth = width - 4
	}

	total := len(state.AvailableTypes)

	// 弹窗高度根据实际类型数量动态收缩，避免列表项少时底部留白过大。
	// 总高 = 上边框1 + 分隔空行1 + listHeight + 分隔空行1 + 统计1 + 下边框1 = listHeight + 5
	// listHeight 取「实际类型数」与「上限」的较小值。
	maxListHeight := 14 // 单屏最多展示的列表项数，超出则滚动
	listHeight := total
	if listHeight > maxListHeight {
		listHeight = maxListHeight
	}
	if listHeight < 3 {
		listHeight = 3
	}
	modalHeight := listHeight + 5
	// 不超过可用屏幕高度的 2/3，保留呼吸空间
	if modalHeight > height-6 {
		modalHeight = height - 6
		listHeight = modalHeight - 5
		if listHeight < 3 {
			listHeight = 3
			modalHeight = 8
		}
	}

	// 列表在弹窗内的起始行（上边框1 + 分隔空行1）
	listStartY := 2

	// 滚动偏移
	visibleStart := 0
	if state.TypeFilterCursor >= listHeight {
		visibleStart = state.TypeFilterCursor - listHeight + 1
	}
	if visibleStart > total-listHeight && total > listHeight {
		visibleStart = total - listHeight
	}
	if visibleStart < 0 {
		visibleStart = 0
	}

	return typeFilterModalLayout{
		modalWidth:   modalWidth,
		modalHeight:  modalHeight,
		listStartY:   listStartY,
		listHeight:   listHeight,
		visibleStart: visibleStart,
	}
}

// ResolveTypeFilterModalBounds 返回类型筛选弹窗在页面坐标系中的边界（右下为开区间）。
// 通过渲染真实弹窗取尺寸，确保与 renderTypeFilterOverlay 的位置完全一致。
func ResolveTypeFilterModalBounds(state PageState, width, height int) (left, top, right, bottom int) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0
	}
	modal := buildTypeFilterModal(state, width, height)
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

// ResolveTypeFilterListItemAt 判断页面坐标 (pageX, pageY) 命中的类型列表项索引。
// 若不在列表行上则返回 -1。bounds 与 layout 用于复用渲染时的计算。
func ResolveTypeFilterListItemAt(state PageState, pageX, pageY, width, height int) int {
	left, top, right, bottom := ResolveTypeFilterModalBounds(state, width, height)
	if pageX < left || pageX >= right || pageY < top || pageY >= bottom {
		return -1
	}
	layout := computeTypeFilterModalLayout(state, width, height)

	// 列表在弹窗内的起始行（含 padding）
	listTopRow := top + layout.listStartY
	listBottomRow := listTopRow + layout.listHeight
	if pageY < listTopRow || pageY >= listBottomRow {
		return -1
	}

	rowOffset := pageY - listTopRow
	idx := layout.visibleStart + rowOffset
	if idx < 0 || idx >= len(state.AvailableTypes) {
		return -1
	}
	return idx
}

// buildTypeFilterModal 渲染类型筛选弹窗本体（无暗化背景）。
// 标题嵌入顶部边框左上角（╭─ title ───╮），尺寸计算与 computeTypeFilterModalLayout 必须一致。
func buildTypeFilterModal(state PageState, width, height int) string {
	layout := computeTypeFilterModalLayout(state, width, height)
	modalWidth := layout.modalWidth
	listHeight := layout.listHeight

	// 渲染类型列表
	total := len(state.AvailableTypes)
	visibleStart := layout.visibleStart
	visibleEnd := visibleStart + listHeight
	if visibleEnd > total {
		visibleEnd = total
	}

	var typeLines []string
	for i := visibleStart; i < visibleEnd; i++ {
		typeName := state.AvailableTypes[i]
		isSelected := false
		for _, st := range state.SelectedTypes {
			if st == typeName {
				isSelected = true
				break
			}
		}

		// 获取类型颜色
		color := getAdjustedRuleTypeColor(typeName, nil)

		// 构建行内容
		var line string
		if i == state.TypeFilterCursor {
			// 当前光标行 — Tokyo Night 选中态
			cursorStyle := lipgloss.NewStyle().
				Background(common.TokyoSelected()).
				Foreground(common.TokyoCyan())
			var checkMark string
			if isSelected {
				checkMark = lipgloss.NewStyle().Foreground(common.TokyoGreen()).Render("✓ ")
			} else {
				checkMark = "  "
			}
			typeStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
			line = cursorStyle.Render(" " + checkMark + typeStyle.Render(typeName) + " ")
		} else {
			var checkMark string
			if isSelected {
				checkMark = lipgloss.NewStyle().Foreground(common.TokyoGreen()).Render("✓ ")
			} else {
				checkMark = "  "
			}
			typeStyle := lipgloss.NewStyle().Foreground(color)
			line = " " + checkMark + typeStyle.Render(typeName)
		}
		typeLines = append(typeLines, line)
	}

	// 填充空行
	for len(typeLines) < listHeight {
		typeLines = append(typeLines, "")
	}

	typeList := strings.Join(typeLines, "\n")

	// 统计信息 — Tokyo Night
	statsText := i18n.Tf("rules.filter_stats", len(state.SelectedTypes), len(state.AvailableTypes))
	stats := common.TokyoMutedStyle().Render(statsText)

	// 组装内容区（标题已嵌入边框；列表上方留一空行与标题边框分隔）
	modalContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		typeList,
		"",
		stats,
	)

	// 标题嵌入顶部边框左上角，padding(1,2) 对齐 RenderBorderedPanel 的左右各1空格
	return common.RenderBorderedPanel(
		i18n.T("rules.filter_title"),
		modalContent,
		modalWidth,
		common.TokyoBlue(),
		common.TokyoForeground(),
	)
}
