package help

type State struct {
	offset int
}

func (s State) Offset() int { return s.offset }

func (s State) Scroll(delta, width, height int, ctx HelpContext) State {
	_, limit := renderScrollablePopup(width, height, ctx)
	s.offset = max(0, min(limit, s.offset+delta))
	return s
}
