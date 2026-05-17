package logs

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	tea "github.com/charmbracelet/bubbletea"
)

func TestState_UpdateMaxHScrollOffset_WideWindow(t *testing.T) {
	s := State{maxHScrollOffset: 0, logHScrollOffset: 50}
	s = s.UpdateMaxHScrollOffset(120, 40)
	if s.maxHScrollOffset <= 0 {
		t.Fatalf("expected positive maxHScrollOffset for wide window, got %d", s.maxHScrollOffset)
	}
}

func TestState_UpdateMaxHScrollOffset_NarrowWindow(t *testing.T) {
	s := State{maxHScrollOffset: 100, logHScrollOffset: 50}
	s = s.UpdateMaxHScrollOffset(30, 40)
	if s.maxHScrollOffset != 0 {
		t.Fatalf("expected maxHScrollOffset=0 for narrow window, got %d", s.maxHScrollOffset)
	}
	if s.logHScrollOffset != 0 {
		t.Fatalf("expected logHScrollOffset clamped to 0, got %d", s.logHScrollOffset)
	}
}

func TestState_UpdateMaxHScrollOffset_ClampCurrentOffset(t *testing.T) {
	s := State{maxHScrollOffset: 0, logHScrollOffset: 80}
	s = s.UpdateMaxHScrollOffset(60, 40)
	if s.maxHScrollOffset < s.logHScrollOffset {
		t.Fatalf("expected logHScrollOffset=%d clamped to maxHScrollOffset=%d", s.logHScrollOffset, s.maxHScrollOffset)
	}
}

func TestState_HScrollOffset_RightKeyBoundary(t *testing.T) {
	s := State{
		logHScrollOffset: 45,
		maxHScrollOffset: 50,
	}
	rightMsg := tea.KeyMsg{Type: tea.KeyRight}
	s, _ = s.Update(rightMsg, nil)
	if s.logHScrollOffset != 50 {
		t.Fatalf("expected logHScrollOffset=50 (reached max), got %d", s.logHScrollOffset)
	}
	s, _ = s.Update(rightMsg, nil)
	if s.logHScrollOffset != 50 {
		t.Fatalf("expected logHScrollOffset stays at 50 (at boundary, no-op), got %d", s.logHScrollOffset)
	}
}

func TestState_HScrollOffset_LeftKeyBoundary(t *testing.T) {
	s := State{
		logHScrollOffset: 0,
		maxHScrollOffset: 50,
	}
	leftMsg := tea.KeyMsg{Type: tea.KeyLeft}
	s, _ = s.Update(leftMsg, nil)
	if s.logHScrollOffset != 0 {
		t.Fatalf("expected logHScrollOffset stays at 0, got %d", s.logHScrollOffset)
	}
}

func TestState_HScrollOffset_RightKeyIncrements(t *testing.T) {
	s := State{
		logHScrollOffset: 0,
		maxHScrollOffset: 50,
	}
	rightMsg := tea.KeyMsg{Type: tea.KeyRight}
	s, _ = s.Update(rightMsg, nil)
	if s.logHScrollOffset != 10 {
		t.Fatalf("expected logHScrollOffset=10 after one right key, got %d", s.logHScrollOffset)
	}
	s, _ = s.Update(rightMsg, nil)
	if s.logHScrollOffset != 20 {
		t.Fatalf("expected logHScrollOffset=20 after two right keys, got %d", s.logHScrollOffset)
	}
}

func TestState_HScrollOffset_LeftKeyDecrements(t *testing.T) {
	s := State{
		logHScrollOffset: 30,
		maxHScrollOffset: 50,
	}
	leftMsg := tea.KeyMsg{Type: tea.KeyLeft}
	s, _ = s.Update(leftMsg, nil)
	if s.logHScrollOffset != 20 {
		t.Fatalf("expected logHScrollOffset=20 after left key, got %d", s.logHScrollOffset)
	}
	s, _ = s.Update(leftMsg, nil)
	if s.logHScrollOffset != 10 {
		t.Fatalf("expected logHScrollOffset=10 after two left keys, got %d", s.logHScrollOffset)
	}
}

func TestState_UpdateMaxHScrollOffset_VerySmallWindow(t *testing.T) {
	s := State{maxHScrollOffset: 50, logHScrollOffset: 30}
	s = s.UpdateMaxHScrollOffset(10, 10)
	if s.maxHScrollOffset != 0 {
		t.Fatalf("expected maxHScrollOffset=0 for very small window, got %d", s.maxHScrollOffset)
	}
	if s.logHScrollOffset != 0 {
		t.Fatalf("expected logHScrollOffset clamped to 0, got %d", s.logHScrollOffset)
	}
}

func TestState_HScrollOffset_ReachesMaxThenStops(t *testing.T) {
	s := State{
		logHScrollOffset: 45,
		maxHScrollOffset: 50,
	}
	rightMsg := tea.KeyMsg{Type: tea.KeyRight}
	s, _ = s.Update(rightMsg, nil)
	if s.logHScrollOffset != 50 {
		t.Fatalf("expected logHScrollOffset=50 (reached max), got %d", s.logHScrollOffset)
	}
	s, _ = s.Update(rightMsg, nil)
	if s.logHScrollOffset != 50 {
		t.Fatalf("expected logHScrollOffset stays at 50, got %d", s.logHScrollOffset)
	}
	s, _ = s.Update(rightMsg, nil)
	if s.logHScrollOffset != 50 {
		t.Fatalf("expected logHScrollOffset still 50 after extra right presses, got %d", s.logHScrollOffset)
	}
}

func TestState_RingBufferBasicOperation(t *testing.T) {
	s := NewState()
	for i := 0; i < common.LogsCap+5; i++ {
		s = s.AppendLog("info", "log entry")
	}
	if s.logCount != common.LogsCap {
		t.Fatalf("expected logCount=%d after overflow, got %d", common.LogsCap, s.logCount)
	}
	logs := s.logs()
	if len(logs) != common.LogsCap {
		t.Fatalf("expected logs length=%d, got %d", common.LogsCap, len(logs))
	}
}

func TestState_FilteredLogsLevel(t *testing.T) {
	s := NewState()
	s = s.AppendLog("debug", "debug log")
	s = s.AppendLog("info", "info log")
	s = s.AppendLog("warning", "warning log")
	s = s.AppendLog("error", "error log")
	s.logLevel = 2
	s.updateFilteredLogs()
	if len(s.filteredLogIndices) != 2 {
		t.Fatalf("expected 2 filtered logs at level warning+, got %d", len(s.filteredLogIndices))
	}
}

func TestState_ClearLogs(t *testing.T) {
	s := NewState()
	s = s.AppendLog("info", "log1")
	s = s.AppendLog("info", "log2")
	s = s.ClearLogs()
	if s.logCount != 0 || s.selectedLog != 0 || s.logScrollTop != 0 {
		t.Fatalf("expected cleared state, got count=%d sel=%d scroll=%d", s.logCount, s.selectedLog, s.logScrollTop)
	}
}
