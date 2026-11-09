package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	corepage "github.com/AimAI-Labs/mihosh/internal/ui/tui/features/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
)

func TestOfflineStartupRoutesToCoreWithoutStartingStreams(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	svc := service.NewCoreService()
	defer svc.Close()
	m := Model{config: &config.Config{}, coreSvc: svc, coreState: corepage.NewState(svc), width: 100, height: 30}
	updated, cmd := m.Update(messages.CoreConnectionMsg{Err: errors.New("not installed")})
	m = updated.(Model)
	if m.currentPage != layout.PageCore || m.wsStarted || cmd != nil {
		t.Fatal("offline startup started online tasks")
	}
	if !strings.Contains(m.View(), "Mihomo Core") {
		t.Fatal("Core page not rendered")
	}
}
func TestCoreIsReachableFromNavigationByMouse(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	m := Model{width: 100, height: 30}
	x := -1
	for candidate := 1; candidate < 99; candidate++ {
		if layout.GetClickedTopNavPage(candidate, 1, 100) == layout.PageCore {
			x = candidate
			break
		}
	}
	if x < 0 {
		t.Fatal("Core navigation missing")
	}
	result, _ := m.Update(tea.MouseMsg{X: x, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if result.(Model).currentPage != layout.PageCore {
		t.Fatal("Core click did not route")
	}
}

func TestStaleCoreConnectionCannotOverrideCurrentTarget(t *testing.T) {
	m := Model{currentPage: layout.PageSub, connectionEpoch: 2, connectionReady: true}
	updated, cmd := m.Update(messages.CoreConnectionMsg{Epoch: 1, Err: errors.New("old target failed")})
	next := updated.(Model)
	if next.currentPage != layout.PageSub || !next.connectionReady || cmd != nil {
		t.Fatal("stale health result changed current target")
	}
}

func TestQueuedWebSocketMessageFromPreviousControllerIsIgnored(t *testing.T) {
	ws := api.NewWSClient("127.0.0.1:9090", "")
	old := ws.Generation()
	ws.UpdateEndpoint("127.0.0.1:9091", "")
	m := Model{wsClient: ws, chartData: model.NewChartData(10)}
	updated, _ := m.Update(messages.CoreWSMsg{Generation: old, Payload: messages.MemoryWSMsg{Memory: 1234}})
	if len(updated.(Model).chartData.MemoryHistory) != 0 {
		t.Fatal("old controller message polluted new chart")
	}
}

func TestCoreReleaseCatalogueSurvivesPageChangeAndGlobalRefresh(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	svc := service.NewCoreService()
	defer svc.Close()
	s := corepage.NewState(svc)
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	defer s.Close()
	m := Model{currentPage: layout.PageSub, coreSvc: svc, coreState: s, width: 120, height: 30}
	page := coreinfra.ReleasePage{Page: 1, Releases: []coreinfra.Release{coreinfra.PinnedRelease()}}
	updated, _ := m.Update(messages.CoreOperationMsg{ID: s.OperationID(), Action: "releases", Releases: &page})
	m = updated.(Model)
	if m.currentPage != layout.PageSub || m.coreState.Busy {
		t.Fatal("release query completion changed the page or left the task running")
	}
	m.currentPage = layout.PageCore
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	if !strings.Contains(m.View(), "Available core versions") {
		t.Fatal("background query did not reach the version browser")
	}
	previous := m.coreState.OperationID()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(Model)
	defer m.coreState.Close()
	if !m.coreState.Busy || m.coreState.OperationID() != previous+1 || cmd == nil {
		t.Fatal("global r did not refresh the release list")
	}
}
