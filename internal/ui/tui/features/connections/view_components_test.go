package connections

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
)

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
