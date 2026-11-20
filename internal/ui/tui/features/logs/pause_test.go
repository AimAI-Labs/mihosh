package logs

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestLogsPauseFreezesVisibleSnapshotWhileIngestingLiveLogs(t *testing.T) {
	i18n.Init()
	s := NewState(nil)
	s = s.AppendLog("info", "first-entry")
	s = s.AppendLog("info", "second-entry")

	// Pause via 'p'
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if !s.Paused() {
		t.Fatal("expected logs to be paused after pressing p")
	}
	if len(s.Logs()) != 2 || s.Logs()[0].Payload != "second-entry" {
		t.Fatalf("unexpected frozen snapshot: %+v", s.Logs())
	}

	// Continue ingesting WS and process logs; cachedLogs backing array is reused in place,
	// so frozen snapshot must remain intact.
	for i := 0; i < 10; i++ {
		s = s.AppendLog("info", fmt.Sprintf("live-%d", i))
	}
	s, _ = s.Update(messages.CoreProcessLogsMsg{
		Start:  0,
		Output: "level=warning msg=\"process-log-while-paused\"\n",
	})

	if len(s.Logs()) != 2 || s.Logs()[0].Payload != "second-entry" || s.Logs()[1].Payload != "first-entry" {
		t.Fatalf("frozen logs mutated during background ingestion: %+v", s.Logs())
	}
	if s.pausedNewCount != 11 {
		t.Fatalf("expected pausedNewCount=11, got %d", s.pausedNewCount)
	}

	// Rendered view shows paused badge with buffered count.
	rendered := ansi.Strip(RenderLogsPage(s.ToPageState(100, 24)))
	if !strings.Contains(rendered, i18n.Tf("logs.paused_badge_new", 11)) {
		t.Fatalf("expected paused badge with count in rendered logs page:\n%s", rendered)
	}

	// Filtering works against frozen snapshot while paused.
	s.logFilter = "first"
	s.updateFilteredLogs()
	if len(s.filteredLogIndices) != 1 || s.Logs()[s.filteredLogIndices[0]].Payload != "first-entry" {
		t.Fatalf("filtering while paused did not match frozen snapshot")
	}
	s.logFilter = ""
	s.updateFilteredLogs()

	// Resume via Space catches up to all buffered logs.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeySpace})
	if s.Paused() {
		t.Fatal("expected Space to resume logs")
	}
	if len(s.Logs()) != 13 || s.Logs()[0].Payload != "process-log-while-paused" {
		t.Fatalf("resumed logs missing background entries: len=%d top=%q", len(s.Logs()), s.Logs()[0].Payload)
	}
}

func TestLogsClickPauseButtonInLevelBarTogglesPause(t *testing.T) {
	i18n.Init()
	s := NewState(nil)
	const width = 100
	rendered := ansi.Strip(renderLevelBar(s.logLevel, width, s.paused, s.pausedNewCount))
	lines := strings.Split(rendered, "\n")
	btn := i18n.T("logs.pause_btn")
	idx := strings.Index(lines[1], btn)
	if idx < 0 {
		t.Fatalf("pause button %q not found in level bar: %q", btn, lines[1])
	}
	x := ansi.StringWidth(lines[1][:idx])
	next, _ := s.HandleMouseLeft(1, x, width)
	if !next.Paused() {
		t.Fatal("clicking pause button in level bar did not pause")
	}
	next, _ = next.HandleMouseLeft(1, x, width)
	if next.Paused() {
		t.Fatal("clicking pause button again did not resume")
	}
}
