package sub

import "github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"

type subListLayout struct {
	window   common.ListWindow
	rowsY    int
	capacity int
}

func layoutSubList(state PageState) subListLayout {
	capacity := max(subMinHeight, state.Height-subFixedLines)
	return subListLayout{
		window: common.ResolveListWindow(len(state.FilteredIdx), state.Selected, state.ScrollTop, capacity),
		rowsY:  subHeaderHeight + 1, capacity: capacity,
	}
}

func (l subListLayout) itemAt(y int) int {
	offset := y - l.rowsY
	if offset < 0 || offset >= l.window.End-l.window.Start {
		return -1
	}
	return l.window.Start + offset
}
