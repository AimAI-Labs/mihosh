package connections

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/connections/components"
)

func TestResolveConnectionsMouseHit_FindsSiteAndConnectionTargets(t *testing.T) {
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
		ViewMode: 0,
		SiteTests: []model.SiteTest{
			{Name: "A", URL: "http://a.test"},
			{Name: "B", URL: "http://b.test"},
			{Name: "C", URL: "http://c.test"},
			{Name: "D", URL: "http://d.test"},
		},
	}

	siteFound := false
	connFound := false

	for y := 0; y < state.Height; y++ {
		for x := 0; x < state.Width; x++ {
			hit := ResolveMouseHit(state, x, y)
			if hit.Target == MouseTargetSiteTest && hit.Index == 2 {
				siteFound = true
			}
			if hit.Target == MouseTargetConnection && hit.Index == 0 {
				connFound = true
			}
		}
	}

	if !siteFound {
		t.Fatalf("expected to find site-test hit for index 2")
	}
	if !connFound {
		t.Fatalf("expected to find connection-row hit for index 0")
	}
}

func TestResolveConnectionsMouseHit_FindsViewModeTabs(t *testing.T) {
	state := PageState{
		Connections: &model.ConnectionsResponse{},
		Width:       120,
		Height:      30,
		ViewMode:    ConnViewActive,
	}

	activeFound := false
	historyFound := false

	for y := 0; y < state.Height; y++ {
		for x := 0; x < state.Width; x++ {
			hit := ResolveMouseHit(state, x, y)
			if hit.Target == MouseTargetViewActive {
				activeFound = true
			}
			if hit.Target == MouseTargetViewHistory {
				historyFound = true
			}
		}
	}

	if !activeFound {
		t.Fatalf("expected to find active-view tab hit")
	}
	if !historyFound {
		t.Fatalf("expected to find history-view tab hit")
	}
}

func TestResolveConnectionsMouseHit_WithCharts_AlignsFirstVisibleRow(t *testing.T) {
	chart := model.NewChartData(60)
	for i := 0; i < 10; i++ {
		chart.AddSpeedData(int64(i*100), int64(i*90))
		chart.AddMemoryData(int64(1000000 + i*1000))
		chart.AddConnCountData(i)
	}

	state := PageState{
		Connections: &model.ConnectionsResponse{
			Connections: []model.Connection{
				{ID: "c1", Metadata: model.Metadata{Host: "alpha.example.com", DestinationIP: "1.1.1.1"}},
				{ID: "c2", Metadata: model.Metadata{Host: "beta.example.com", DestinationIP: "2.2.2.2"}},
				{ID: "c3", Metadata: model.Metadata{Host: "gamma.example.com", DestinationIP: "3.3.3.3"}},
				{ID: "c4", Metadata: model.Metadata{Host: "delta.example.com", DestinationIP: "4.4.4.4"}},
			},
		},
		Width:     120,
		Height:    32,
		ViewMode:  0,
		ChartData: chart,
		SiteTests: model.DefaultSiteTests(),
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

func TestResolveConnectionsMouseHit_WithTopN_AlignsFirstVisibleRow(t *testing.T) {
	chart := model.NewChartData(60)
	for i := 0; i < 10; i++ {
		chart.AddSpeedData(int64(i*100), int64(i*90))
		chart.AddMemoryData(int64(1000000 + i*1000))
		chart.AddConnCountData(i)
	}

	state := PageState{
		Connections: &model.ConnectionsResponse{
			Connections: []model.Connection{
				{ID: "c1", Metadata: model.Metadata{Host: "alpha.example.com", DestinationIP: "1.1.1.1"}},
				{ID: "c2", Metadata: model.Metadata{Host: "beta.example.com", DestinationIP: "2.2.2.2"}},
				{ID: "c3", Metadata: model.Metadata{Host: "gamma.example.com", DestinationIP: "3.3.3.3"}},
				{ID: "c4", Metadata: model.Metadata{Host: "delta.example.com", DestinationIP: "4.4.4.4"}},
			},
		},
		Width:     120,
		Height:    40,
		ViewMode:  0,
		ChartData: chart,
		SiteTests: model.DefaultSiteTests(),
		TopNItems: []components.TopNItem{
			{Name: "p1", TotalBytes: 1000},
			{Name: "p2", TotalBytes: 900},
			{Name: "p3", TotalBytes: 800},
			{Name: "p4", TotalBytes: 700},
			{Name: "p5", TotalBytes: 600},
			{Name: "p6", TotalBytes: 500},
			{Name: "p7", TotalBytes: 400},
		},
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
