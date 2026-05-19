package logs

import (
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
)

func TestRenderLogEntry_HOffsetZero(t *testing.T) {
	log := model.LogEntry{
		Type:      "info",
		Payload:   "This is a test log message",
		Timestamp: time.Now(),
	}
	result := renderLogEntry(log, false, 80, 0)
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result, "This is a test log message") {
		t.Fatalf("expected log content in result, got %q", result)
	}
}

func TestRenderLogEntry_HOffsetPositive(t *testing.T) {
	log := model.LogEntry{
		Type:      "info",
		Payload:   "This is a test log message that is quite long",
		Timestamp: time.Now(),
	}
	result := renderLogEntry(log, false, 80, 10)
	if result == "" {
		t.Fatal("expected non-empty result with hOffset")
	}
	if strings.Contains(result, "This is a") {
		t.Fatal("expected content to be scrolled right, first chars should not appear")
	}
}

func TestRenderLogEntry_HOffsetExceedsContent(t *testing.T) {
	log := model.LogEntry{
		Type:      "info",
		Payload:   "Short",
		Timestamp: time.Now(),
	}
	result := renderLogEntry(log, false, 80, 100)
	if result == "" {
		t.Fatal("expected empty content but still a rendered row")
	}
}

func TestRenderLogEntry_Selected(t *testing.T) {
	log := model.LogEntry{
		Type:      "info",
		Payload:   "Selected log",
		Timestamp: time.Now(),
	}
	result := renderLogEntry(log, true, 80, 0)
	if !strings.Contains(result, "INFO") {
		t.Fatalf("expected INFO level in selected result, got %q", result)
	}
	if !strings.Contains(result, "Selected log") {
		t.Fatalf("expected selected log content, got %q", result)
	}
}

func TestRenderLogEntry_CJKContent(t *testing.T) {
	log := model.LogEntry{
		Type:      "info",
		Payload:   "测试日志内容很长需要滚动查看",
		Timestamp: time.Now(),
	}
	result := renderLogEntry(log, false, 60, 0)
	if result == "" {
		t.Fatal("expected non-empty result for CJK content")
	}
}

func TestRenderLogEntry_MaxWidthNarrow(t *testing.T) {
	log := model.LogEntry{
		Type:      "info",
		Payload:   "A very long log message that should be truncated",
		Timestamp: time.Now(),
	}
	result := renderLogEntry(log, false, 20, 0)
	if result == "" {
		t.Fatal("expected non-empty result even with narrow maxWidth")
	}
}

func TestRenderLogList_EmptyLogs(t *testing.T) {
	result := renderLogList(nil, 0, 0, 10, 80, 0)
	if !strings.Contains(result, "暂无日志") {
		t.Fatalf("expected placeholder for empty logs, got %q", result)
	}
}

func TestRenderLogList_SelectedVisible(t *testing.T) {
	logs := []model.LogEntry{
		{Type: "info", Payload: "Log 0", Timestamp: time.Now()},
		{Type: "info", Payload: "Log 1", Timestamp: time.Now()},
		{Type: "info", Payload: "Log 2", Timestamp: time.Now()},
	}
	result := renderLogList(logs, 1, 0, 10, 80, 0)
	if !strings.Contains(result, "Log 1") {
		t.Fatalf("expected selected log visible, got %q", result)
	}
}

func TestRenderLogList_ScrollToSelected(t *testing.T) {
	logs := make([]model.LogEntry, 20)
	for i := 0; i < 20; i++ {
		logs[i] = model.LogEntry{Type: "info", Payload: "Log", Timestamp: time.Now()}
	}
	result := renderLogList(logs, 15, 0, 5, 80, 0)
	if strings.Contains(result, "Log 0") {
		t.Fatal("expected first entries scrolled away when selected is far down")
	}
}

func TestRenderLogEntry_URLEncoded(t *testing.T) {
	log := model.LogEntry{
		Type:      "info",
		Payload:   "url%3Dencoded%20content",
		Timestamp: time.Now(),
	}
	result := renderLogEntry(log, false, 80, 0)
	if !strings.Contains(result, "url=encoded") {
		t.Fatalf("expected URL-decoded content, got %q", result)
	}
}

func TestResolveLogLevelTabStyle_ActiveKeepsContinuousBackgroundAroundIndicator(t *testing.T) {
	style := resolveLogLevelTabStyle("info", true)
	if style.Background != common.CSecondary {
		t.Fatalf("expected active background %q, got %q", common.CSecondary, style.Background)
	}
	if style.Foreground != common.CWhite {
		t.Fatalf("expected active foreground %q, got %q", common.CWhite, style.Foreground)
	}
	if style.Indicator == style.Background {
		t.Fatalf("expected active indicator color to differ from background, got %q", style.Indicator)
	}
	if style.Indicator != common.CWhite {
		t.Fatalf("expected active info indicator fallback %q, got %q", common.CWhite, style.Indicator)
	}
	if !style.Bold {
		t.Fatal("expected active tab to be bold")
	}
}

func TestResolveLogLevelTabStyle_InactiveUsesCommonInactiveColors(t *testing.T) {
	style := resolveLogLevelTabStyle("debug", false)
	if style.Background != common.CHighlight {
		t.Fatalf("expected inactive background %q, got %q", common.CHighlight, style.Background)
	}
	if style.Foreground != common.CMuted {
		t.Fatalf("expected inactive foreground %q, got %q", common.CMuted, style.Foreground)
	}
	if style.Indicator != logLevelColors["debug"] {
		t.Fatalf("expected indicator foreground %q, got %q", logLevelColors["debug"], style.Indicator)
	}
	if style.Bold {
		t.Fatal("expected inactive tab not to be bold")
	}
}
