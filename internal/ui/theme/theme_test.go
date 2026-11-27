package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestBuiltinThemesComplete(t *testing.T) {
	names := Names()
	if len(names) != len(builtinThemes) {
		t.Fatalf("Names() count = %d, builtinThemes count = %d (catalog and palette map diverged)", len(names), len(builtinThemes))
	}
	for _, name := range names {
		th, ok := builtinThemes[name]
		if !ok {
			t.Errorf("theme %q in Names() but not in builtinThemes", name)
		}
		if th.Name != name ||
			th.Foreground == "" || th.Muted == "" || th.Dim == "" || th.Bright == "" ||
			th.Background == "" || th.Surface == "" || th.Overlay == "" || th.Selected == "" ||
			th.Primary == "" || th.Secondary == "" ||
			th.Success == "" || th.Warning == "" || th.Danger == "" || th.Info == "" ||
			th.Orange == "" || th.Purple == "" || th.Border == "" || th.Active == "" {
			t.Errorf("theme %q has empty or mismatched fields", name)
		}
	}
}

func TestCurrentNameDefault(t *testing.T) {
	if CurrentName() != "tokyo-night" {
		t.Fatalf("expected default tokyo-night, got %q", CurrentName())
	}
}

func TestSetThemeValid(t *testing.T) {
	defer SetTheme("tokyo-night")
	if !SetTheme("catppuccin") {
		t.Fatal("SetTheme(catppuccin) returned false")
	}
	if CurrentName() != "catppuccin" {
		t.Fatalf("expected catppuccin, got %q", CurrentName())
	}
	th := Current()
	if th.Name != "catppuccin" {
		t.Fatalf("Current().Name = %q, want catppuccin", th.Name)
	}
}

func TestSetThemeInvalid(t *testing.T) {
	before := CurrentName()
	if SetTheme("nonexistent") {
		t.Fatal("SetTheme(nonexistent) returned true")
	}
	if CurrentName() != before {
		t.Fatalf("current theme changed after invalid set: %q -> %q", before, CurrentName())
	}
}

func TestIsValid(t *testing.T) {
	for _, name := range Names() {
		if !IsValid(name) {
			t.Errorf("IsValid(%q) = false, want true", name)
		}
	}
	if IsValid("nonexistent") {
		t.Error("IsValid(nonexistent) = true, want false")
	}
}

func TestNamesStableOrder(t *testing.T) {
	want := []string{
		"tokyo-night",
		"dark",
		"light",
		"solarized-dark",
		"solarized-light",
		"colorblind-dark",
		"colorblind-light",
		"terminal",
		"catppuccin",
		"catppuccin-latte",
		"gruvbox",
		"gruvbox-light",
		"nord",
		"dracula",
		"kanagawa",
		"one-dark",
		"rose-pine",
		"everforest",
	}
	got := Names()
	if len(got) != len(want) {
		t.Fatalf("Names() len = %d, want %d", len(got), len(want))
	}
	for i, n := range want {
		if got[i] != n {
			t.Fatalf("Names()[%d] = %q, want %q", i, got[i], n)
		}
	}
}

func TestGetReturnsPaletteWithoutMutatingCurrent(t *testing.T) {
	defer SetTheme("tokyo-night")
	SetTheme("tokyo-night")

	got, ok := Get("solarized-dark")
	if !ok {
		t.Fatal("Get(solarized-dark) returned ok=false")
	}
	if got.Name != "solarized-dark" || got.Background != lipgloss.Color("#002B36") {
		t.Fatalf("unexpected palette from Get(solarized-dark): %+v", got)
	}
	if CurrentName() != "tokyo-night" {
		t.Fatalf("Get() mutated current theme to %q", CurrentName())
	}

	if _, ok := Get("nonexistent"); ok {
		t.Fatal("Get(nonexistent) returned ok=true")
	}
}

func TestTerminalThemeUsesNativeANSIColors(t *testing.T) {
	th, ok := Get("terminal")
	if !ok {
		t.Fatal("Get(terminal) returned ok=false")
	}
	if Resolve(th.Primary) != th.Primary || Resolve(th.Background) != th.Background {
		t.Fatalf("expected terminal ANSI colors to pass through Resolve unchanged")
	}
}

func TestThemeReturnsColorValues(t *testing.T) {
	defer SetTheme("tokyo-night")
	SetTheme("dracula")
	th := Current()
	if string(th.Primary) == "" {
		t.Error("Current().Primary is empty")
	}
	if th.Primary != lipgloss.Color("#BD93F9") {
		t.Errorf("Theme().Primary = %q, want #BD93F9", th.Primary)
	}
}

func TestResolveMatchesLipglossProfilesAndHandlesInvalidColors(t *testing.T) {
	prev := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(prev)

	for _, profile := range []termenv.Profile{
		termenv.TrueColor,
		termenv.ANSI256,
		termenv.ANSI,
		termenv.Ascii,
	} {
		lipgloss.SetColorProfile(profile)
		for _, hex := range []lipgloss.Color{"#7AA2F7", "#BD93F9", "#F7768E", "#9ECE6A"} {
			want := lipgloss.NewStyle().Foreground(hex).Render("mihosh")
			got := lipgloss.NewStyle().Foreground(Resolve(hex)).Render("mihosh")
			if got != want {
				t.Fatalf("profile=%v color=%s render mismatch: got %q, want %q", profile, hex, got, want)
			}
		}
	}

	for _, raw := range []lipgloss.Color{"", "12", "#zzzzzz", "#12"} {
		if got := Resolve(raw); got != raw {
			t.Fatalf("Resolve(%q) = %#v, want original input", raw, got)
		}
	}
}
