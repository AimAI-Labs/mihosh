package connections

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func TestSiteBatchCapacityRefillAndCancellation(t *testing.T) {
	const total = model.TestConcurrency * 5
	started := make(chan int, total)
	finished := make(chan tea.Msg, total)
	release := make(chan struct{})
	var active, peak atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
		}
		var index int
		fmt.Sscanf(r.URL.Path, "/site-%d", &index)
		started <- index
		select {
		case <-release:
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	sites := make([]model.SiteTest, total)
	for i := range sites {
		sites[i] = model.SiteTest{Name: fmt.Sprint(i), URL: fmt.Sprintf("%s/site-%d", server.URL, i)}
	}
	state := NewState("", sites, nil, 5000, nil)
	t.Cleanup(func() { state.CancelTest() })
	state, cmd := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != model.TestConcurrency {
		t.Fatalf("initial launch is %T with %d tasks", batch, len(batch))
	}
	for _, command := range batch {
		go func() { finished <- command() }()
	}
	awaitStart := func() {
		t.Helper()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("site test did not start")
		}
	}
	for i := 0; i < model.TestConcurrency; i++ {
		awaitStart()
	}
	if _, cmd := state.launchSiteTests(5000); cmd != nil {
		t.Fatal("full site batch launched more work")
	}
	// One response opens exactly one slot; the replacement remains blocked.
	release <- struct{}{}
	var first messages.SiteTestMsg
	select {
	case msg := <-finished:
		first = msg.(messages.SiteTestMsg)
	case <-time.After(5 * time.Second):
		t.Fatal("site test did not finish")
	}
	state, cmd = state.CompleteSiteTest(first)
	if cmd == nil {
		t.Fatal("site batch did not refill")
	}
	go func() { finished <- cmd() }()
	awaitStart()
	if _, duplicate := state.CompleteSiteTest(first); duplicate != nil {
		t.Fatal("duplicate result launched a task")
	}
	state.CancelTest()
	for i := 0; i < model.TestConcurrency; i++ {
		select {
		case msg := <-finished:
			state, cmd = state.CompleteSiteTest(msg.(messages.SiteTestMsg))
			if cmd != nil {
				t.Fatal("cancelled result launched waiting work")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("cancelled site test did not exit")
		}
	}
	if peak.Load() != model.TestConcurrency || len(started) != 0 {
		t.Fatalf("peak=%d extra starts=%d", peak.Load(), len(started))
	}
	for i, site := range state.siteTests {
		if site.Testing || site.Error != "" {
			t.Fatalf("cancelled task %d changed the current result: %+v", i, site)
		}
	}
}

func TestSiteRetestRejectsOldAndDuplicateResults(t *testing.T) {
	state := NewState("", []model.SiteTest{{Name: "same", URL: "://invalid"}, {Name: "same", URL: "://invalid"}}, nil, 1000, nil)
	state, cmd := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	commands := cmd().(tea.BatchMsg)
	old := commands[0]().(messages.SiteTestMsg)
	state, cmd = state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	commands = cmd().(tea.BatchMsg)
	if _, cmd := state.CompleteSiteTest(old); cmd != nil || !state.siteTests[0].Testing {
		t.Fatal("old result completed a new run")
	}
	second := commands[1]().(messages.SiteTestMsg)
	state, _ = state.CompleteSiteTest(second)
	if !state.siteTests[0].Testing || state.siteTests[1].Testing || state.siteTests[1].Error != "timeout" {
		t.Fatal("failed out-of-order task completed the wrong site")
	}
	state, _ = state.CompleteSiteTest(second)
	first := commands[0]().(messages.SiteTestMsg)
	state, _ = state.CompleteSiteTest(first)
	if state.siteBatch.Active() || state.siteTests[0].Testing {
		t.Fatal("site batch did not finish")
	}
}
