package sub

import "github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"

const subCardHeight = 9

func subCardColumns(width int) int {
	switch {
	case width >= 150:
		return 3
	case width >= 100:
		return 2
	default:
		return 1
	}
}

func subCardWidths(width int) []int {
	columns := subCardColumns(width)
	available := max(1, width-subScrollWidth-common.CardGap*(columns-1))
	switch columns {
	case 3:
		column := available / 3
		return []int{column, column, available - 2*column}
	case 2:
		column := available / 2
		return []int{column, column}
	default:
		return []int{available}
	}
}

func layoutSubList(state PageState) common.CardGrid {
	return common.LayoutCardGrid(len(state.FilteredIdx), state.Selected, state.ScrollTop,
		state.Height-subFixedLines-1, subCardHeight, subCardWidths(state.Width)...)
}
