package common

import "testing"

func TestResolveListWindow(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		total, selected, top, capacity int
		want                           ListWindow
	}{
		{"empty", 0, 20, 10, 5, ListWindow{}},
		{"follow down", 40, 20, 0, 5, ListWindow{20, 16, 21}},
		{"follow up", 40, 3, 20, 5, ListWindow{3, 3, 8}},
		{"keep window", 40, 20, 18, 5, ListWindow{20, 18, 23}},
		{"clamp negative", 40, -1, -1, 5, ListWindow{0, 0, 5}},
		{"clamp overflow", 40, 80, 80, 5, ListWindow{39, 39, 40}},
		{"zero capacity", 40, 20, 0, 0, ListWindow{20, 20, 21}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveListWindow(tc.total, tc.selected, tc.top, tc.capacity); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
