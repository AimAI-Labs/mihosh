package nodes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNodesBatchRespectsCapacity(t *testing.T) {
	const total = model.TestConcurrency * 5
	started := make(chan string, total)
	finished := make(chan tea.Msg, total)
	release := make([]chan struct{}, total)
	names := make([]string, total)
	for i := range names {
		names[i] = fmt.Sprintf("node-%d", i)
		release[i] = make(chan struct{}, 1)
	}
	var active, peak atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
		}
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/proxies/"), "/delay")
		var index int
		if _, err := fmt.Sscanf(name, "node-%d", &index); err != nil || index < 0 || index >= total {
			http.Error(w, "unexpected task", http.StatusBadRequest)
			return
		}
		started <- name
		select {
		case <-release[index]:
			fmt.Fprint(w, `{"delay":42}`)
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	// Release every handler even if an assertion aborts the test.
	t.Cleanup(func() {
		for _, ch := range release {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	})
	client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 5000)
	state := NewState(client, "http://test.invalid", 5000)
	state.CurrentProxies = names
	state, cmd := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	launch := func(cmd tea.Cmd, count int) {
		t.Helper()
		var commands []tea.Cmd
		if cmd != nil {
			// A multi-task launch must expose a bounded BatchMsg, never all pending work.
			if count == 1 {
				commands = []tea.Cmd{cmd}
			} else if batch, ok := cmd().(tea.BatchMsg); ok {
				commands = batch
			}
		}
		if len(commands) != count {
			t.Fatalf("launched %d commands, want %d", len(commands), count)
		}
		for _, command := range commands {
			go func() { finished <- command() }()
		}
	}
	awaitStart := func() string {
		t.Helper()
		select {
		case name := <-started:
			return name
		case <-time.After(5 * time.Second):
			t.Fatal("task did not start")
			return ""
		}
	}
	launch(cmd, model.TestConcurrency)
	for i := 0; i < model.TestConcurrency; i++ {
		awaitStart()
	}
	state, cmd = state.LaunchBatchTests()
	if cmd != nil {
		t.Fatal("full batch launched more work")
	}
	for completed := 0; completed < total; completed++ {
		release[completed] <- struct{}{}
		var result messages.TestDoneMsg
		select {
		case raw := <-finished:
			if bound, ok := raw.(messages.CoreHTTPMsg); ok {
				raw = bound.Payload
			}
			var ok bool
			result, ok = raw.(messages.TestDoneMsg)
			if !ok || result.Err != nil {
				t.Fatalf("unexpected completion: %+v", raw)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("task did not finish")
		}
		state, cmd = state.CompleteTest(result)
		if completed < total-model.TestConcurrency {
			launch(cmd, 1)
			awaitStart()
		} else if state.Testing && cmd != nil {
			t.Fatal("draining batch launched unexpected work")
		}
	}
	if peak.Load() != model.TestConcurrency || active.Load() != 0 || state.Testing {
		t.Fatalf("peak=%d active=%d testing=%t", peak.Load(), active.Load(), state.Testing)
	}
}

func TestNodeRetestRejectsOldAndDuplicateResults(t *testing.T) {
	state := State{CurrentProxies: make([]string, model.TestConcurrency*5)}
	state, cmd := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	old := cmd().(tea.BatchMsg)[0]().(messages.TestDoneMsg)
	state, cmd = state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	commands := cmd().(tea.BatchMsg)
	state, cmd = state.CompleteTest(old)
	if done, _ := state.testBatch.Progress(); done != 0 || len(state.TestResults()) != 0 || cmd != nil {
		t.Fatal("old result advanced a new batch")
	}
	result := commands[len(commands)-1]().(messages.TestDoneMsg)
	state, cmd = state.CompleteTest(result)
	if done, _ := state.testBatch.Progress(); done != 1 || len(state.TestResults()) != 1 || cmd == nil {
		t.Fatal("failed out-of-order result did not complete and refill one task")
	}
	if state.TestResults()[0].Error == "" {
		t.Fatal("failed result was not retained")
	}
	state, cmd = state.CompleteTest(result)
	if done, _ := state.testBatch.Progress(); done != 1 || len(state.TestResults()) != 1 || cmd != nil {
		t.Fatal("duplicate result changed progress or results")
	}
	state, cmd = state.StartSingleTest("single")
	single := cmd().(messages.TestDoneMsg)
	state, cmd = state.CompleteTest(result)
	if state.TestingTarget != "single" || cmd != nil {
		t.Fatal("old batch result completed a new single test")
	}
	state, _ = state.CompleteTest(single)
	if state.Testing || state.TestingTarget != "" || len(state.TestResults()) != 2 {
		t.Fatal("single test did not finish")
	}
	state, _ = state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	state = state.ResetTesting()
	if _, cmd := state.LaunchBatchTests(); cmd != nil {
		t.Fatal("cancelled batch launched waiting tasks")
	}
}
