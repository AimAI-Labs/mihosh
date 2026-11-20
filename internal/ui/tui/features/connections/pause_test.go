package connections

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestConnectionsPauseFreezesActiveAndHistoryWhileContinuingWS(t *testing.T) {
	i18n.Init()
	s := NewState("", nil, nil, 0, nil)
	s.connViewMode = ConnViewActive

	s = s.ApplyWSConnections(api.ConnectionsData{
		Connections: []api.ConnectionData{
			{ID: "c1", Metadata: api.ConnectionMeta{Host: "alpha.test"}},
			{ID: "c2", Metadata: api.ConnectionMeta{Host: "beta.test"}},
		},
	})
	// Close c2 so history has 1 entry before pausing.
	s = s.ApplyWSConnections(api.ConnectionsData{
		Connections: []api.ConnectionData{
			{ID: "c1", Metadata: api.ConnectionMeta{Host: "alpha.test"}},
		},
	})
	if len(s.ClosedConnections()) != 1 || s.ClosedConnections()[0].ID != "c2" {
		t.Fatalf("expected 1 closed connection c2 before pause, got %+v", s.ClosedConnections())
	}

	// Pause via 'p'
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if !s.Paused() {
		t.Fatal("expected connections to be paused after pressing p")
	}

	// Push several WS snapshots closing c1 and adding new connections (which mutates cachedClosedConns backing array).
	for i := 0; i < 5; i++ {
		s = s.ApplyWSConnections(api.ConnectionsData{
			Connections: []api.ConnectionData{
				{ID: fmt.Sprintf("live-%d", i), Metadata: api.ConnectionMeta{Host: fmt.Sprintf("live-%d.test", i)}},
			},
		})
	}

	// Frozen active list still shows only c1, and frozen history still shows only c2.
	ps := s.ToPageState(nil, 120, 30)
	if !ps.Paused || ps.PausedUpdates != 5 {
		t.Fatalf("expected Paused=true PausedUpdates=5, got paused=%t updates=%d", ps.Paused, ps.PausedUpdates)
	}
	if len(ps.Connections.Connections) != 1 || ps.Connections.Connections[0].ID != "c1" {
		t.Fatalf("frozen active list mutated during WS updates: %+v", ps.Connections.Connections)
	}
	if len(ps.ClosedConnections) != 1 || ps.ClosedConnections[0].ID != "c2" {
		t.Fatalf("frozen history list mutated during WS updates: %+v", ps.ClosedConnections)
	}

	// Rendered active page displays the paused status badge with update count.
	rendered := ansi.Strip(RenderConnectionsPage(ps))
	if !strings.Contains(rendered, i18n.Tf("conns.paused_badge_new", 5)) {
		t.Fatalf("expected paused badge in rendered connections page:\n%s", rendered)
	}

	// Resume via Space shows latest live connections and all closed connections accumulated while paused.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeySpace})
	if s.Paused() {
		t.Fatal("expected Space to resume connections")
	}
	psResumed := s.ToPageState(nil, 120, 30)
	if len(psResumed.Connections.Connections) != 1 || psResumed.Connections.Connections[0].ID != "live-4" {
		t.Fatalf("resumed active list missing live snapshot: %+v", psResumed.Connections.Connections)
	}
	if len(psResumed.ClosedConnections) < 5 {
		t.Fatalf("resumed history missing connections closed while paused: len=%d", len(psResumed.ClosedConnections))
	}
}

func TestConnectionsMouseClickPauseToggle(t *testing.T) {
	i18n.Init()
	s := NewState("", nil, nil, 0, nil)
	s.connViewMode = ConnViewActive
	s.Connections = &model.ConnectionsResponse{
		Connections: []model.Connection{{ID: "c1", Metadata: model.Metadata{Host: "a.test"}}},
	}

	const width, height = 120, 30
	x, y, ok := findConnMousePoint(s, width, height, MouseTargetPauseToggle, -1)
	if !ok {
		t.Fatal("pause toggle button not hit in connections mode bar")
	}
	next, _ := s.HandleMouseLeft(x, y, width, height)
	if !next.Paused() {
		t.Fatal("clicking pause button did not pause")
	}
	x2, y2, ok := findConnMousePoint(next, width, height, MouseTargetPauseToggle, -1)
	if !ok {
		t.Fatal("paused badge not hit in connections mode bar")
	}
	next, _ = next.HandleMouseLeft(x2, y2, width, height)
	if next.Paused() {
		t.Fatal("clicking paused badge did not resume")
	}
}
