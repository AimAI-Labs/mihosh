package model

import "slices"

var builtinThemeNames = []string{
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
