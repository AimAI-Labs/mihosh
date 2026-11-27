package model

import (
	"reflect"
	"testing"
)

func TestThemeNamesAndValidation(t *testing.T) {
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
	got := ThemeNames()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ThemeNames() = %v, want %v", got, want)
	}

	// Mutating the returned slice must not corrupt the catalog.
	got[0] = "corrupted"
	if !IsValidTheme("tokyo-night") {
		t.Fatal("mutating ThemeNames() slice corrupted catalog")
	}

	for _, name := range want {
		if !IsValidTheme(name) {
			t.Errorf("IsValidTheme(%q) = false, want true", name)
		}
	}
	for _, invalid := range []string{"", "unknown", "Tokyo-Night"} {
		if IsValidTheme(invalid) {
			t.Errorf("IsValidTheme(%q) = true, want false", invalid)
		}
	}
}
