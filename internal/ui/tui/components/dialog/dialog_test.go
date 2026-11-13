package dialog_test

import (
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestPlacementMatchesBounds(t *testing.T) {
	const content = "╭──╮\n│中│\n╰──╯"
	for _, test := range []struct {
		name          string
		dialog        dialog.Dialog
		left, top     int
		width, height int
	}{
		{"centered odd gap", dialog.New(content, 13, 8), 4, 2, 13, 8},
		{"below header", dialog.New(content, 12, 9).Below(4), 4, 4, 12, 9},
		{"bottom adjustment", dialog.New(content, 12, 5).Below(4), 4, 2, 12, 5},
		{"full page", dialog.New(content, 4, 3).Below(0), 0, 0, 4, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			left, top, right, bottom := test.dialog.Bounds()
			if left != test.left || top != test.top || right != left+4 || bottom != top+3 {
				t.Fatalf("bounds = (%d,%d,%d,%d)", left, top, right, bottom)
			}
			base := strings.TrimSuffix(strings.Repeat(strings.Repeat("x", test.width)+"\n", test.height), "\n")
			for _, view := range []string{test.dialog.View(), test.dialog.Overlay(base, false)} {
				lines := strings.Split(ansi.Strip(view), "\n")
				if len(lines) != test.height {
					t.Fatalf("height = %d, want %d", len(lines), test.height)
				}
				for i, want := range strings.Split(content, "\n") {
					if got := ansi.Cut(lines[top+i], left, right); got != want {
						t.Fatalf("dialog row %d = %q, want %q", i, got, want)
					}
				}
			}
		})
	}
}

func TestOverlayPreservesBackgroundAndANSI(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	content := "\x1b[31m中AB\x1b[0m"
	base := "abcdefghijkl\nabcdefghijkl\nabcdefghijkl"
	d := dialog.New(content, 12, 3)
	plain := d.Overlay(base, false)
	dimmed := d.Overlay(base, true)
	if want := "abcdefghijkl\nabcd中ABijkl\nabcdefghijkl"; ansi.Strip(plain) != want || ansi.Strip(dimmed) != want {
		t.Fatal("overlay changed surrounding content or ANSI display width")
	}
	if !strings.Contains(dimmed, content) || !strings.Contains(dimmed, "\x1b[2m") {
		t.Fatal("dialog colors or background dimming were lost")
	}
	if strings.Contains(plain, "\x1b[2m") {
		t.Fatal("background was dimmed without request")
	}
	if got := dialog.New("AB", 6, 3).Overlay("x", false); got != "x\n  AB\n" {
		t.Fatalf("short background was not padded: %q", got)
	}
}

func TestClippedDialogFitsSmallViewport(t *testing.T) {
	content := "\x1b[31m中文ABCDE\x1b[0m\nsecond\nthird"
	for _, size := range [][2]int{{1, 1}, {3, 2}, {5, 1}} {
		width, height := size[0], size[1]
		d := dialog.New(content, width, height).Clipped()
		left, top, right, bottom := d.Bounds()
		if left != 0 || top != 0 || right != width || bottom != height {
			t.Fatalf("clipped bounds = (%d,%d,%d,%d)", left, top, right, bottom)
		}
		for _, view := range []string{d.View(), d.Overlay("background\nbackground\nbackground", true)} {
			if lipgloss.Height(view) != height {
				t.Fatal("clipped dialog exceeds viewport height")
			}
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > width {
					t.Fatalf("clipped dialog exceeds viewport width: %q", line)
				}
			}
		}
	}
}

func TestInvalidViewport(t *testing.T) {
	for _, size := range [][2]int{{0, 10}, {10, 0}, {-1, 5}, {5, -1}} {
		d := dialog.New("content", size[0], size[1])
		if d.View() != "" || d.Overlay("base", true) != "" {
			t.Fatal("invalid viewport should render nothing")
		}
		if left, top, right, bottom := d.Bounds(); left != 0 || top != 0 || right != 0 || bottom != 0 {
			t.Fatal("invalid viewport should have empty bounds")
		}
	}
}
