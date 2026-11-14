package common

// ListWindow is a bounded, selection-following window over a linear list.
type ListWindow struct {
	Selected int
	Start    int
	End      int
}

func ResolveListWindow(total, selected, scrollTop, capacity int) ListWindow {
	if total <= 0 {
		return ListWindow{}
	}
	capacity = max(1, capacity)
	selected = max(0, min(selected, total-1))
	scrollTop = max(0, min(scrollTop, total-1))
	if selected >= scrollTop+capacity {
		scrollTop = selected - capacity + 1
	}
	scrollTop = min(scrollTop, selected)
	return ListWindow{Selected: selected, Start: scrollTop, End: min(total, scrollTop+capacity)}
}
