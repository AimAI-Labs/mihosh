package model

import "slices"

var builtinThemeNames = []string{
	"tokyo-night",
	"catppuccin",
	"gruvbox",
	"nord",
	"dracula",
	"kanagawa",
	"solarized-light",
}

// ThemeNames returns all supported built-in theme names in stable display order.
func ThemeNames() []string {
	out := make([]string, len(builtinThemeNames))
	copy(out, builtinThemeNames)
	return out
}

// IsValidTheme reports whether name is a recognized built-in theme identifier.
func IsValidTheme(name string) bool {
	return slices.Contains(builtinThemeNames, name)
}
