package connections

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/x/ansi"
)

func TestConnectionListRenderedRowsAndInlineBounds(t *testing.T) {
	for _, width := range []int{99, 100} {
		for _, height := range []int{18, 32} {
			for _, inline := range []bool{false, true} {
				for _, mode := range []int{ConnViewActive, ConnViewHistory} {
					for _, filter := range []string{"", "inactive", "editing", "empty-input"} {
						t.Run(fmt.Sprintf("%d/%d/inline=%t/mode=%d/%s", width, height, inline, mode, filter), func(t *testing.T) {
							var conns []model.Connection
							for i := 0; i < 40; i++ {
								conns = append(conns, model.Connection{ID: fmt.Sprint(i), Metadata: model.Metadata{Host: fmt.Sprintf("example-%02d.test", i)}})
							}
							state := PageState{
								Width: width, Height: height, ViewMode: mode, InlineMode: inline,
								Connections: &model.ConnectionsResponse{Connections: conns}, ClosedConnections: conns,
								SelectedIndex: 24, ScrollTop: 18,
							}
							if filter != "" {
								state.FilterText = "example"
							}
							if filter == "editing" || filter == "empty-input" {
								state.FilterMode = true
							}
							if filter == "editing" {
								state.FilterInput = "search: example"
							}
							found := 0
							for y, raw := range strings.Split(RenderConnectionsPage(state), "\n") {
								line := ansi.Strip(raw)
								if inline && strings.Contains(line, "╭") && strings.Contains(line, i18n.T("conns.detail.title")) {
									hit := ResolveMouseHit(state, 1, y)
									if hit.Target != MouseTargetInlineDetail {
										t.Fatalf("inline border y=%d: got %+v", y, hit)
									}
									if hit := ResolveMouseHit(state, 1, y-1); hit.Target == MouseTargetInlineDetail {
										t.Fatalf("row before inline detail y=%d belongs to the list", y-1)
									}
									break
								}
								for i := range conns {
									if strings.Contains(line, conns[i].Metadata.Host) {
										found++
										hit := ResolveMouseHit(state, 0, y)
										if hit.Target != MouseTargetConnection || hit.Index != i {
											t.Fatalf("rendered connection[%d] at y=%d: got %+v", i, y, hit)
										}
									}
								}
							}
							if found == 0 {
								t.Fatal("no rendered connection rows checked")
							}
						})
					}
				}
			}
		}
	}
}

func TestDetailWheelUsesRenderedPanel(t *testing.T) {
	for _, width := range []int{99, 100} {
		for _, height := range []int{20, 36} {
			for _, focus := range []int{0, 1} {
				t.Run(fmt.Sprintf("%d/%d/focus=%d", width, height, focus), func(t *testing.T) {
					state := NewState("", nil, nil, 0, nil)
					state.connDetailMode = true
					state.connDetailSnapshot = &model.Connection{ID: "detail"}
					state.connDetailFocusPanel = focus
					state.connDetailLeftPanel.ScrollTop = 2
					state.connDetailRightPanel.ScrollTop = 2
					lines := strings.Split(RenderConnectionsPage(state.ToPageState(nil, width, height)), "\n")
					for y, raw := range lines {
						line := ansi.Strip(raw)
						if offset := strings.Index(line, i18n.T("conns.detail.title_json")); offset >= 0 {
							x := ansi.StringWidth(line[:offset])
							got, _ := state.HandleMouseScroll(true, x, y, width, height)
							if got.connDetailRightPanel.ScrollTop != 1 || got.connDetailLeftPanel.ScrollTop != 2 || got.connDetailFocusPanel != 1 {
								t.Fatalf("wheel at JSON title (%d,%d) scrolled wrong panel: left=%d right=%d focus=%d", x, y, got.connDetailLeftPanel.ScrollTop, got.connDetailRightPanel.ScrollTop, got.connDetailFocusPanel)
							}
							return
						}
					}
					t.Fatal("JSON panel title not rendered")
				})
			}
		}
	}
}

func TestConnectionWheelFollowsPointerInsteadOfInlineFocus(t *testing.T) {
	state := NewState("", nil, nil, 0, nil)
	state.connViewMode = ConnViewActive
	state.inlineDetailMode = true
	state.Connections = &model.ConnectionsResponse{Connections: []model.Connection{
		{ID: "one", Metadata: model.Metadata{Host: "wheel-one.test"}},
		{ID: "two", Metadata: model.Metadata{Host: "wheel-two.test"}},
	}}
	state.filterList.Cursor = 1
	state.inlineDetailPanel.ScrollTop = 2
	const width, height = 100, 32
	var rowY, inlineY int
	for y, line := range strings.Split(RenderConnectionsPage(state.ToPageState(nil, width, height)), "\n") {
		if strings.Contains(line, "wheel-two.test") && rowY == 0 {
			rowY = y
		}
		if strings.Contains(line, "╭") && strings.Contains(line, i18n.T("conns.detail.title")) {
			inlineY = y
		}
	}
	if rowY == 0 || inlineY == 0 {
		t.Fatal("missing rendered list or inline detail")
	}
	got, _ := state.HandleMouseScroll(true, 1, inlineY, width, height)
	if got.inlineDetailPanel.ScrollTop != 1 || got.filterList.Cursor != 1 {
		t.Fatalf("wheel over inline detail moved list: %+v", got.filterList)
	}
	state.inlineDetailFocused = true
	got, _ = state.HandleMouseScroll(true, 1, rowY, width, height)
	if got.filterList.Cursor != 0 || got.inlineDetailFocused {
		t.Fatalf("wheel over list stayed in inline detail: cursor=%d focused=%t", got.filterList.Cursor, got.inlineDetailFocused)
	}
}

func TestEmptyConnectionListHasNoInlineHits(t *testing.T) {
	state := PageState{Width: 99, Height: 24, ViewMode: ConnViewActive, InlineMode: true, Connections: &model.ConnectionsResponse{}}
	for y := 0; y < state.Height; y++ {
		hit := ResolveMouseHit(state, 1, y)
		if hit.Target == MouseTargetConnection || hit.Target == MouseTargetInlineDetail {
			t.Fatalf("empty list has content hit at y=%d: %+v", y, hit)
		}
	}
}

func TestResolveConnectionsMouseHit_FindsSiteTargetInTrafficTab(t *testing.T) {
	state := PageState{
		Width:    120,
		Height:   30,
		ViewMode: ConnViewTraffic,
		SiteTests: []model.SiteTest{
			{Name: "A", URL: "http://a.test"},
			{Name: "B", URL: "http://b.test"},
			{Name: "C", URL: "http://c.test"},
			{Name: "D", URL: "http://d.test"},
		},
	}

	siteFound := false
	for y := 0; y < state.Height; y++ {
		for x := 0; x < state.Width; x++ {
			hit := ResolveMouseHit(state, x, y)
			if hit.Target == MouseTargetSiteTest && hit.Index == 2 {
				siteFound = true
			}
		}
	}

	if !siteFound {
		t.Fatalf("expected to find site-test hit for index 2 in traffic tab")
	}
}

func TestResolveConnectionsMouseHit_FindsConnectionTargetInActiveTab(t *testing.T) {
	state := PageState{
		Connections: &model.ConnectionsResponse{
			Connections: []model.Connection{
				{
					ID: "conn-1",
					Metadata: model.Metadata{
						Host:          "example.com",
						DestinationIP: "1.1.1.1",
					},
				},
			},
		},
		Width:    120,
		Height:   30,
		ViewMode: ConnViewActive,
	}

	connFound := false
	for y := 0; y < state.Height; y++ {
		for x := 0; x < state.Width; x++ {
			hit := ResolveMouseHit(state, x, y)
			if hit.Target == MouseTargetConnection && hit.Index == 0 {
				connFound = true
			}
		}
	}

	if !connFound {
		t.Fatalf("expected to find connection-row hit for index 0 in active tab")
	}
}

func TestResolveConnectionsMouseHit_FindsViewModeTabs(t *testing.T) {
	state := PageState{
		Connections: &model.ConnectionsResponse{},
		Width:       120,
		Height:      30,
		ViewMode:    ConnViewActive,
	}

	trafficFound := false
	activeFound := false
	historyFound := false

	for y := 0; y < state.Height; y++ {
		for x := 0; x < state.Width; x++ {
			hit := ResolveMouseHit(state, x, y)
			if hit.Target == MouseTargetViewTraffic {
				trafficFound = true
			}
			if hit.Target == MouseTargetViewActive {
				activeFound = true
			}
			if hit.Target == MouseTargetViewHistory {
				historyFound = true
			}
		}
	}

	if !trafficFound {
		t.Fatalf("expected to find traffic-view tab hit")
	}
	if !activeFound {
		t.Fatalf("expected to find active-view tab hit")
	}
	if !historyFound {
		t.Fatalf("expected to find history-view tab hit")
	}
}

func TestResolveConnectionsMouseHit_ActiveTab_ConnectionAlignment(t *testing.T) {
	state := PageState{
		Connections: &model.ConnectionsResponse{
			Connections: []model.Connection{
				{ID: "c1", Metadata: model.Metadata{Host: "alpha.example.com", DestinationIP: "1.1.1.1"}},
				{ID: "c2", Metadata: model.Metadata{Host: "beta.example.com", DestinationIP: "2.2.2.2"}},
				{ID: "c3", Metadata: model.Metadata{Host: "gamma.example.com", DestinationIP: "3.3.3.3"}},
				{ID: "c4", Metadata: model.Metadata{Host: "delta.example.com", DestinationIP: "4.4.4.4"}},
			},
		},
		Width:    120,
		Height:   32,
		ViewMode: ConnViewActive,
	}

	rendered := RenderConnectionsPage(state)
	lines := strings.Split(rendered, "\n")

	firstRowY := -1
	for i, line := range lines {
		if strings.Contains(line, "alpha.example.com") {
			firstRowY = i
			break
		}
	}
	if firstRowY < 0 {
		t.Fatalf("failed to find first connection row in rendered output")
	}

	hit := ResolveMouseHit(state, 0, firstRowY)
	if hit.Target != MouseTargetConnection || hit.Index != 0 {
		t.Fatalf("expected first row hit => connection[0], got target=%v index=%d (y=%d)", hit.Target, hit.Index, firstRowY)
	}
}

func TestResolveConnectionsMouseHit_TrafficTab_TopNAndChart(t *testing.T) {
	chart := model.NewChartData(60)
	for i := 0; i < 10; i++ {
		chart.AddSpeedData(int64(i*100), int64(i*90))
		chart.AddMemoryData(int64(1000000 + i*1000))
		chart.AddConnCountData(i)
	}

	state := PageState{
		Width:     120,
		Height:    40,
		ViewMode:  ConnViewTraffic,
		ChartData: chart,
		SiteTests: model.DefaultSiteTests(),
		TopNItems: []components.TopNItem{
			{Name: "p1", TotalBytes: 1000},
			{Name: "p2", TotalBytes: 900},
			{Name: "p3", TotalBytes: 800},
		},
	}

	chartFound := false
	topNFound := false

	for y := 0; y < state.Height; y++ {
		for x := 0; x < state.Width; x++ {
			hit := ResolveMouseHit(state, x, y)
			if hit.Target == MouseTargetChart {
				chartFound = true
			}
			if hit.Target == MouseTargetTopN {
				topNFound = true
			}
		}
	}

	if !chartFound {
		t.Fatalf("expected to find chart hit in traffic tab")
	}
	if !topNFound {
		t.Fatalf("expected to find topN hit in traffic tab")
	}
}
