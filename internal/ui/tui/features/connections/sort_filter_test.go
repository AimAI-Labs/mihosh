package connections

import (
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestConnectionsSortActiveAndHistory(t *testing.T) {
	i18n.Init()
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	conns := []model.Connection{
		{
			ID: "c1", Download: 100, Upload: 50, DownloadSpeed: 500, UploadSpeed: 100,
			Start:    now.Add(-10 * time.Second).Format(time.RFC3339),
			ClosedAt: now.Add(-5 * time.Second), // 5s duration
		},
		{
			ID: "c2", Download: 1000, Upload: 200, DownloadSpeed: 100, UploadSpeed: 50,
			Start:    now.Add(-60 * time.Second).Format(time.RFC3339),
			ClosedAt: now.Add(-10 * time.Second), // 50s duration
		},
		{
			ID: "c3", Download: 300, Upload: 100, DownloadSpeed: 2000, UploadSpeed: 0,
			Start:    now.Add(-2 * time.Second).Format(time.RFC3339),
			ClosedAt: now, // 2s duration
		},
	}

	assertOrder := func(name string, got []model.Connection, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: got len %d, want %d", name, len(got), len(want))
		}
		for i := range want {
			if got[i].ID != want[i] {
				t.Fatalf("%s: pos %d got %s, want %s", name, i, got[i].ID, want[i])
			}
		}
	}

	assertOrder("speed desc", queryConnections(conns, ConnQuery{SortBy: ConnSortSpeed, SortDesc: true}, now), "c3", "c1", "c2")
	assertOrder("speed asc", queryConnections(conns, ConnQuery{SortBy: ConnSortSpeed, SortDesc: false}, now), "c2", "c1", "c3")
	assertOrder("traffic desc", queryConnections(conns, ConnQuery{SortBy: ConnSortTraffic, SortDesc: true}, now), "c2", "c3", "c1")
	assertOrder("duration desc (closed)", queryConnections(conns, ConnQuery{SortBy: ConnSortDuration, SortDesc: true}, now), "c2", "c1", "c3")
	assertOrder("start desc (newest)", queryConnections(conns, ConnQuery{SortBy: ConnSortStart, SortDesc: true}, now), "c3", "c1", "c2")
	assertOrder("start asc (oldest)", queryConnections(conns, ConnQuery{SortBy: ConnSortStart, SortDesc: false}, now), "c2", "c1", "c3")
}

func TestConnectionsFilterByProcessSourceIPProtoAndChain(t *testing.T) {
	now := time.Now()
	conns := []model.Connection{
		{
			ID: "a", Chains: []string{"HK-01", "Proxy"},
			Metadata: model.Metadata{Host: "a.test", Network: "tcp", Process: "curl", SourceIP: "192.168.1.10"},
		},
		{
			ID: "b", Chains: []string{"US-01", "Proxy"},
			Metadata: model.Metadata{Host: "b.test", Network: "udp", Process: "chrome", SourceIP: "10.0.0.5"},
		},
		{
			ID: "c", Chains: []string{"DIRECT"},
			Metadata: model.Metadata{Host: "c.test", Network: "tcp", Process: "chrome", SourceIP: "192.168.1.10"},
		},
	}

	// Protocol filter
	udp := queryConnections(conns, ConnQuery{Protocol: "udp"}, now)
	if len(udp) != 1 || udp[0].ID != "b" {
		t.Fatalf("protocol udp filter mismatch: %+v", udp)
	}

	// Process field filter
	proc := queryConnections(conns, ConnQuery{Field: ConnFilterProcess, Text: "curl"}, now)
	if len(proc) != 1 || proc[0].ID != "a" {
		t.Fatalf("process filter mismatch: %+v", proc)
	}

	// Source IP field filter
	src := queryConnections(conns, ConnQuery{Field: ConnFilterSourceIP, Text: "10.0.0"}, now)
	if len(src) != 1 || src[0].ID != "b" {
		t.Fatalf("source IP filter mismatch: %+v", src)
	}

	// Chain field filter
	chain := queryConnections(conns, ConnQuery{Field: ConnFilterChain, Text: "hk-01"}, now)
	if len(chain) != 1 || chain[0].ID != "a" {
		t.Fatalf("chain filter mismatch: %+v", chain)
	}

	// Combined structured token filter
	combo := queryConnections(conns, ConnQuery{Text: "proc:chrome proto:tcp src:192.168 chain:direct"}, now)
	if len(combo) != 1 || combo[0].ID != "c" {
		t.Fatalf("structured token filter mismatch: %+v", combo)
	}
}

func TestConnectionsComputesSpeedFromConsecutiveWSSamples(t *testing.T) {
	s := NewState("", nil, nil, 0, nil)
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	s = s.applyWSConnectionsAt(api.ConnectionsData{
		Connections: []api.ConnectionData{
			{ID: "c1", Download: 1000, Upload: 500},
		},
	}, t0)
	s = s.applyWSConnectionsAt(api.ConnectionsData{
		Connections: []api.ConnectionData{
			{ID: "c1", Download: 5000, Upload: 1500}, // +4000 DL, +1000 UL in 2s -> 2000/s, 500/s
		},
	}, t0.Add(2*time.Second))

	got := s.Connections.Connections[0]
	if got.DownloadSpeed != 2000 || got.UploadSpeed != 500 {
		t.Fatalf("expected computed speeds 2000/500, got %d/%d", got.DownloadSpeed, got.UploadSpeed)
	}
}

func TestConnectionsSortFilterRenderMouseAndSelectionShareSameResult(t *testing.T) {
	i18n.Init()
	s := NewState("", nil, nil, 0, nil)
	s.connViewMode = ConnViewActive
	s.Connections = &model.ConnectionsResponse{
		Connections: []model.Connection{
			{ID: "low", Download: 10, Metadata: model.Metadata{Host: "low.test", Network: "tcp"}},
			{ID: "high", Download: 9000, Metadata: model.Metadata{Host: "high.test", Network: "udp"}},
			{ID: "mid", Download: 500, Metadata: model.Metadata{Host: "mid.test", Network: "tcp"}},
		},
	}

	// Press 's' twice: Default -> Speed -> Traffic (descending)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	// Press 't' once: filter TCP only -> remaining should be [mid (500), low (10)]
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	sel := s.selectedConnection()
	if sel == nil || sel.ID != "mid" {
		t.Fatalf("expected first sorted+filtered TCP connection to be mid, got %+v", sel)
	}

	ps := s.ToPageState(nil, 120, 30)
	rendered := ansi.Strip(RenderConnectionsPage(ps))
	if strings.Contains(rendered, "high.test") {
		t.Fatalf("UDP connection high.test should be filtered out:\n%s", rendered)
	}
	for y, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, "mid.test") {
			hit := ResolveMouseHit(ps, 5, y)
			if hit.Target != MouseTargetConnection || hit.Index != 0 {
				t.Fatalf("rendered mid.test at y=%d did not map to index 0, got %+v", y, hit)
			}
			return
		}
	}
	t.Fatalf("mid.test not found in rendered output:\n%s", rendered)
}
