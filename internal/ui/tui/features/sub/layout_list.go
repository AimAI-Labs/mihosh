package sub

import "github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"

const subCardHeight = 9

func subCardWidths(width int) []int {
	available := max(1, width-subScrollWidth)
	if width < 100 {
		return []int{available}
	}
	available -= common.CardGap
	left := available * 43 / 100
	return []int{left, available - left}
}

func layoutSubList(state PageState) common.CardGrid {
	return common.LayoutCardGrid(len(state.FilteredIdx), state.Selected, state.ScrollTop,
		state.Height-subFixedLines-1, subCardHeight, subCardWidths(state.Width)...)
}
