package placeholder

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func initI18n(t *testing.T, lang string) {
	t.Helper()
	i18n.Init()
	i18n.SetLanguageOverride(lang)
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
}

// compact 去掉所有空白，便于忽略换行位置比较文案。
func compact(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func assertContainsText(t *testing.T, view, text string) {
	t.Helper()
	if !strings.Contains(compact(view), compact(text)) {
		t.Fatalf("placeholder is missing %q\n%s", text, view)
	}
}

func TestSpriteRowsShareOneWidth(t *testing.T) {
	want := len(sprite[0])
	for i, row := range sprite {
		if len(row) != want {
			t.Fatalf("sprite row %d width = %d, want %d", i, len(row), want)
		}
	}
}

func TestRenderFillsRequestedArea(t *testing.T) {
	initI18n(t, "en-US")

	for _, size := range [][2]int{{80, 24}, {40, 12}, {30, 8}, {20, 5}, {120, 40}} {
		width, height := size[0], size[1]
		lines := strings.Split(Render(width, height, 0), "\n")
		if len(lines) != height {
			t.Fatalf("%dx%d: got %d lines, want %d", width, height, len(lines), height)
		}
		for i, line := range lines {
			if w := lipgloss.Width(line); w > width {
				t.Fatalf("%dx%d: line %d width = %d, want <= %d", width, height, i, w, width)
			}
		}
	}
}

func TestRenderShowsTitleReasonAndHint(t *testing.T) {
	for _, lang := range []string{"en-US", "zh-CN"} {
		initI18n(t, lang)
		view := ansi.Strip(Render(80, 24, 0))
		assertContainsText(t, view, i18n.T("placeholder.title"))
		assertContainsText(t, view, i18n.T("placeholder.reason"))
		assertContainsText(t, view, i18n.T("placeholder.hint"))
	}
}

func TestRenderAnimatesSpark(t *testing.T) {
	initI18n(t, "en-US")

	frames := make(map[string]struct{})
	for frame := 0; frame < len(sparkFrames); frame++ {
		view := ansi.Strip(Render(80, 24, frame))
		if !strings.Contains(view, "▀") {
			t.Fatalf("frame %d lost the pixel sprite", frame)
		}
		frames[view] = struct{}{}
	}
	if len(frames) != len(sparkFrames) {
		t.Fatalf("expected %d distinct frames, got %d", len(sparkFrames), len(frames))
	}
}

func TestRenderDropsSpriteWhenSmall(t *testing.T) {
	initI18n(t, "en-US")

	wide := ansi.Strip(Render(80, 24, 0))
	if !strings.Contains(wide, "▀") {
		t.Fatal("large placeholder should draw the pixel sprite")
	}

	narrow := ansi.Strip(Render(20, 6, 0))
	if strings.Contains(narrow, "▀") || strings.Contains(narrow, "▄") {
		t.Fatal("narrow placeholder should drop the pixel sprite")
	}
	assertContainsText(t, narrow, i18n.T("placeholder.title"))
}

func TestStateAdvanceLoopsFrames(t *testing.T) {
	s := State{}
	for i := 1; i <= len(sparkFrames); i++ {
		s = s.Advance()
		if s.Frame() != i%len(sparkFrames) {
			t.Fatalf("after %d advances frame = %d", i, s.Frame())
		}
	}
}
