package common

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

func TestRenderBorderedPanelKeepsEveryLineAtRequestedWidth(t *testing.T) {
	const width = 48

	panel := RenderBorderedPanel(
		"JSON 详情",
		"short\n"+strings.Repeat("x", width),
		width,
		TokyoMuted(),
		TokyoBlue(),
	)

	for i, line := range strings.Split(panel, "\n") {
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("line %d width = %d, want %d: %q", i, got, width, line)
		}
	}
}

func TestRenderBorderedPanelKeepsRightBorderAfterTruncatingStyledContent(t *testing.T) {
	const width = 32

	longStyledLine := lipgloss.NewStyle().
		Foreground(TokyoGreen()).
		Render(strings.Repeat("x", width*2))
	panel := RenderBorderedPanel("JSON 详情", longStyledLine, width, TokyoMuted(), TokyoBlue())

	lines := strings.Split(panel, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 panel lines, got %d: %q", len(lines), panel)
	}

	for i, line := range lines {
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("line %d width = %d, want %d: %q", i, got, width, line)
		}
	}

	middleLine := ansiRE.ReplaceAllString(lines[1], "")
	if !strings.HasSuffix(middleLine, "│") {
		t.Fatalf("expected middle line to end with right border, got %q", middleLine)
	}
}

func TestTruncateDisplayDoesNotLeavePartialANSISequences(t *testing.T) {
	styled := "\x1b[32m" + strings.Repeat("x", 40) + "\x1b[0m"

	got := ansiRE.ReplaceAllString(TruncateDisplay(styled, 10), "")
	if strings.Contains(got, "[") || strings.Contains(got, ";") {
		t.Fatalf("expected ANSI-aware truncation, got visible text %q", got)
	}
	if !strings.Contains(TruncateDisplay(styled, 10), "\x1b[0m") {
		t.Fatalf("expected truncated styled text to reset ANSI styles")
	}
}
