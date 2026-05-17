package logs

import (
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/charmbracelet/lipgloss"
)

func TestRenderLogsPage_LongLogLine_NoTruncationOverflow(t *testing.T) {
	longPayload := strings.Repeat("A", 500)
	logs := []model.LogEntry{
		{Type: "info", Payload: longPayload, Timestamp: time.Now()},
	}

	state := PageState{
		Logs:               logs,
		FilteredLogIndices: []int{0},
		LogLevel:           0,
		FilterText:         "",
		FilterMode:        false,
		SelectedLog:        0,
		ScrollTop:          0,
		HScrollOffset:      0,
		Width:             80,
		Height:            24,
	}

	result := RenderLogsPage(state)
	lines := strings.Split(result, "\n")

	hasLongLine := false
	for _, line := range lines {
		if len(line) > 200 {
			hasLongLine = true
			break
		}
	}
	if hasLongLine {
		t.Fatal("expected no extremely long lines in output that could overflow TUI")
	}
}

func TestRenderLogsPage_WithHorizontalScroll(t *testing.T) {
	longPayload := strings.Repeat("X", 200)
	logs := []model.LogEntry{
		{Type: "info", Payload: longPayload, Timestamp: time.Now()},
	}

	state := PageState{
		Logs:               logs,
		FilteredLogIndices: []int{0},
		LogLevel:           0,
		FilterText:         "",
		FilterMode:        false,
		SelectedLog:        0,
		ScrollTop:          0,
		HScrollOffset:      50,
		Width:             80,
		Height:            24,
	}

	result := RenderLogsPage(state)
	if result == "" {
		t.Fatal("expected non-empty result with hScrollOffset")
	}
}

func TestRenderLogEntry_VeryLongLine_HScrollOffsetNearMax(t *testing.T) {
	veryLongPayload := strings.Repeat("B", 1000)
	log := model.LogEntry{
		Type:      "info",
		Payload:   veryLongPayload,
		Timestamp: time.Now(),
	}

	result := renderLogEntry(log, false, 80, 495)
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if len(line) > 100 {
			t.Fatalf("line too long (%d chars): %.100q...", len(line), line)
		}
	}
}

func TestRenderLogEntry_UnicodeCJKScroll(t *testing.T) {
	cjkPayload := "中文日志内容测试这是一段很长的中文日志用于测试水平滚动时对双字节字符的处理是否正确"
	log := model.LogEntry{
		Type:      "info",
		Payload:   cjkPayload,
		Timestamp: time.Now(),
	}

	for hOffset := 0; hOffset <= 60; hOffset += 10 {
		result := renderLogEntry(log, false, 80, hOffset)
		if result == "" {
			t.Fatalf("empty result at hOffset=%d", hOffset)
		}
		lines := strings.Split(result, "\n")
		for _, line := range lines {
			if lipgloss.Width(line) > 100 {
				t.Fatalf("line width too large (%d) at hOffset=%d: %.80q", lipgloss.Width(line), hOffset, line)
			}
		}
	}
}

func TestRenderLogEntry_SpecialChars(t *testing.T) {
	specialPayload := "URL=https://example.com/path?param=value&other=12345 Content-Type: application/json"
	log := model.LogEntry{
		Type:      "info",
		Payload:   specialPayload,
		Timestamp: time.Now(),
	}

	result := renderLogEntry(log, false, 80, 0)
	if result == "" {
		t.Fatal("expected non-empty result with special chars")
	}
	if !strings.ContainsAny(result, "URL") {
		t.Fatalf("expected URL visible in result, got %q", result)
	}
}

func TestRenderLogsPage_VariousWindowSizes_ContentWidth(t *testing.T) {
	longPayload := strings.Repeat("L", 300)
	logs := []model.LogEntry{
		{Type: "info", Payload: longPayload, Timestamp: time.Now()},
		{Type: "debug", Payload: "short", Timestamp: time.Now()},
	}

	testWidths := []int{80, 100, 120}
	for _, width := range testWidths {
		state := PageState{
			Logs:               logs,
			FilteredLogIndices: []int{0, 1},
			LogLevel:           0,
			FilterText:         "",
			FilterMode:        false,
			SelectedLog:        0,
			ScrollTop:          0,
			HScrollOffset:      0,
			Width:             width,
			Height:            24,
		}

		result := RenderLogsPage(state)
		totalWidth := 0
		for _, line := range strings.Split(result, "\n") {
			w := lipgloss.Width(line)
			if w > totalWidth {
				totalWidth = w
			}
		}
		if totalWidth > width {
			t.Fatalf("output width %d exceeds terminal width %d", totalWidth, width)
		}
	}
}
