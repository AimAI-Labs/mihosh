package common

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

func TestDisplayWidthAndTruncateUnicode(t *testing.T) {
	for _, tt := range []struct {
		name  string
		text  string
		width int
	}{
		{"ascii", "hello", 5},
		{"chinese", "中文", 4},
		{"combining", "e\u0301", 1},
		{"emoji", "👩‍💻", 2},
		{"flag", "🇨🇳", 2},
		{"styled", "\x1b[32m中文\x1b[0m", 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := DisplayWidth(tt.text); got != tt.width {
				t.Fatalf("DisplayWidth(%q) = %d, want %d", tt.text, got, tt.width)
			}
		})
	}
	for _, text := range []string{"中文AB", "👩‍💻AB", "🇨🇳AB"} {
		got := TruncateDisplay(text, 3)
		if DisplayWidth(got) != 3 || !strings.HasSuffix(got, "~") {
			t.Fatalf("TruncateDisplay(%q, 3) = %q, want width 3 ending in ~", text, got)
		}
	}
}

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
