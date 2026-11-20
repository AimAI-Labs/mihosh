package sub

import "github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"

type subListLayout struct {
	window     common.ListWindow
	rowsY      int
	cardHeight int
}

const subCardHeight = 7

func layoutSubList(state PageState) subListLayout {
	available := max(1, state.Height-subFixedLines-1)
	cardHeight := min(subCardHeight, available)
	capacity := available / cardHeight
	return subListLayout{
		window:     common.ResolveListWindow(len(state.FilteredIdx), state.Selected, state.ScrollTop, capacity),
		rowsY:      subHeaderHeight + 1,
		cardHeight: cardHeight,
	}
}

func (l subListLayout) itemAt(y int) int {
	offset := y - l.rowsY
	if offset < 0 || offset >= (l.window.End-l.window.Start)*l.cardHeight {
		return -1
	}
	return l.window.Start + offset/l.cardHeight
}
