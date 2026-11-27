package logs

import (
	"strings"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func init() {
	i18n.Init()
}

func TestRenderLogEntry_HOffsetZero(t *testing.T) {
	log := model.LogEntry{
		Type:      "info",
		Payload:   "This is a test log message",
		Timestamp: time.Now(),
	}
	result := RenderLogEntry(log, false, 80, 0)
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
	result := RenderLogEntry(log, false, 80, 10)
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
	result := RenderLogEntry(log, false, 80, 100)
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
	result := RenderLogEntry(log, true, 80, 0)
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
	result := RenderLogEntry(log, false, 60, 0)
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
	result := RenderLogEntry(log, false, 20, 0)
	if result == "" {
		t.Fatal("expected non-empty result even with narrow maxWidth")
	}
}

func TestRenderLogList_EmptyLogs(t *testing.T) {
	i18n.SetLanguageOverride("zh-CN")
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
	result := RenderLogEntry(log, false, 80, 0)
	if !strings.Contains(result, "url=encoded") {
		t.Fatalf("expected URL-decoded content, got %q", result)
	}
}

func TestResolveLogLevelTabStyle_ActiveKeepsContinuousBackgroundAroundIndicator(t *testing.T) {
	style := resolveLogLevelTabStyle("info", true)
	if style.Background != common.TokyoSelected() {
		t.Fatalf("expected active background %q, got %q", common.TokyoSelected(), style.Background)
	}
	if style.Foreground != common.TokyoCyan() {
		t.Fatalf("expected active foreground %q, got %q", common.TokyoCyan(), style.Foreground)
	}
	if style.Indicator != common.TokyoBlue() {
		t.Fatalf("expected active info indicator %q, got %q", common.TokyoBlue(), style.Indicator)
	}
	if !style.Bold {
		t.Fatal("expected active tab to be bold")
	}
}

func TestResolveLogLevelTabStyle_InactiveUsesCommonInactiveColors(t *testing.T) {
	style := resolveLogLevelTabStyle("debug", false)
	if style.Background != common.TokyoSelected() {
		t.Fatalf("expected inactive background %q, got %q", common.TokyoSelected(), style.Background)
	}
	if style.Foreground != common.TokyoMuted() {
		t.Fatalf("expected inactive foreground %q, got %q", common.TokyoMuted(), style.Foreground)
	}
	if style.Indicator != logLevelColors()["debug"] {
		t.Fatalf("expected indicator foreground %q, got %q", logLevelColors()["debug"], style.Indicator)
	}
	if style.Bold {
		t.Fatal("expected inactive tab not to be bold")
	}
}

func TestClickedLevelMatchesVisibleButtons(t *testing.T) {
	const width = 80
	const selected = 1
	x := 1
	for i, level := range logLevels {
		end := x + lipgloss.Width(renderLogLevelTab(level, i == selected))
		for ; x < end; x++ {
			if got := ClickedLevel(x, width, selected); got != i {
				t.Errorf("column %d: expected %s (%d), got %d", x, level, i, got)
			}
		}
	}
	for _, outside := range []int{0, x, width - 1} {
		if got := ClickedLevel(outside, width, selected); got != -1 {
			t.Errorf("column %d is outside buttons, got %d", outside, got)
		}
	}
	if got := ClickedLevel(9, 10, selected); got != -1 {
		t.Errorf("right border of narrow level bar is outside buttons, got %d", got)
	}
}

func TestLogsIdleHidesSearchBarAndSearchModeReplacesLevelBar(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("") })

	for _, lang := range []string{"zh-CN", "en-US"} {
		i18n.SetLanguageOverride(lang)
		s := NewState(nil).AppendLog("info", "alpha entry").AppendLog("warning", "beta entry")

		for _, width := range []int{32, 80, 120} {
			idleView := ansi.Strip(RenderLogsPage(s.ToPageState(width, 24)))
			if strings.Contains(idleView, i18n.T("logs.search")) {
				t.Fatalf("%s width=%d idle logs page must not render persistent search label:\n%s", lang, width, idleView)
			}
			for i, line := range strings.Split(idleView, "\n") {
				if lipgloss.Width(line) > width {
					t.Fatalf("%s width=%d idle line %d overflowed: %d > %d", lang, width, i, lipgloss.Width(line), width)
				}
			}

			sSearch := s
			sSearch.logFilterMode = true
			sSearch.logFilter = "alpha"
			sSearch.filterEngine = common.SearchRegex
			sSearch.updateFilteredLogs()

			searchView := ansi.Strip(RenderLogsPage(sSearch.ToPageState(width, 24)))
			lines := strings.Split(searchView, "\n")
			if lipgloss.Width(lines[1]) != width {
				t.Fatalf("%s width=%d search bar width=%d, want %d", lang, width, lipgloss.Width(lines[1]), width)
			}
			if !strings.Contains(lines[1], i18n.T("logs.search")) || (width >= 80 && !strings.Contains(lines[1], "[RE]")) {
				t.Fatalf("%s width=%d expected search bar with regex badge: %q", lang, width, lines[1])
			}
			if strings.Contains(lines[1], "debug") || strings.Contains(lines[1], i18n.T("logs.pause_btn")) {
				t.Fatalf("%s width=%d expected level/pause buttons replaced during search: %q", lang, width, lines[1])
			}
		}
	}
}
