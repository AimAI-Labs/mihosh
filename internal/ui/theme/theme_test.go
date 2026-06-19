package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestBuiltinThemesComplete(t *testing.T) {
	for _, name := range Names() {
		th, ok := builtinThemes[name]
		if !ok {
			t.Errorf("theme %q in Names() but not in builtinThemes", name)
		}
		if th.Name == "" || th.Foreground == "" || th.Background == "" || th.Primary == "" {
			t.Errorf("theme %q has empty required fields", name)
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
	want := []string{"tokyo-night", "catppuccin", "gruvbox", "nord", "dracula"}
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
