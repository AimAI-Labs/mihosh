package utils

import (
	"testing"
)

func TestHexToHSL(t *testing.T) {
	tests := []struct {
		name  string
		hex   string
		wantH float64
		wantS float64
		wantL float64
	}{
		{
			name:  "black",
			hex:   "#000000",
			wantH: 0,
			wantS: 0,
			wantL: 0,
		},
		{
			name:  "white",
			hex:   "#FFFFFF",
			wantH: 0,
			wantS: 0,
			wantL: 1.0,
		},
		{
			name:  "red",
			hex:   "#FF0000",
			wantH: 0,
			wantS: 1.0,
			wantL: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HexToHSL(tt.hex)
			if err != nil {
				t.Errorf("HexToHSL() error = %v", err)
				return
			}
			if got.H != tt.wantH || got.S != tt.wantS || got.L != tt.wantL {
				t.Errorf("HexToHSL() = (%v, %v, %v), want (%v, %v, %v)", got.H, got.S, got.L, tt.wantH, tt.wantS, tt.wantL)
			}
		})
	}
}

func TestHSLToHex(t *testing.T) {
	tests := []struct {
		name string
		hsl  HSL
		want string
	}{
		{
			name: "black",
			hsl:  HSL{0, 0, 0},
			want: "#000000",
		},
		{
			name: "white",
			hsl:  HSL{0, 0, 1.0},
			want: "#FFFFFF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HSLToHex(tt.hsl)
			if got != tt.want {
				t.Errorf("HSLToHex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLighterColor(t *testing.T) {
	tests := []struct {
		name       string
		hex        string
		adjustment float64
		want       string
	}{
		{
			name:       "lighter blue",
			hex:        "#00BFFF",
			adjustment: 0.2,
			want:       "#4DD9FF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LighterColor(tt.hex, tt.adjustment)
			if err != nil {
				t.Errorf("LighterColor() error = %v", err)
				return
			}
			if len(got) != 7 {
				t.Errorf("LighterColor() = %v, want 7 chars hex", got)
			}
		})
	}
}

func TestDarkerColor(t *testing.T) {
	tests := []struct {
		name       string
		hex        string
		adjustment float64
	}{
		{
			name:       "darker blue",
			hex:        "#00BFFF",
			adjustment: 0.2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DarkerColor(tt.hex, tt.adjustment)
			if err != nil {
				t.Errorf("DarkerColor() error = %v", err)
				return
			}
			if len(got) != 7 {
				t.Errorf("DarkerColor() = %v, want 7 chars hex", got)
			}
		})
	}
}

func TestContrastRatio(t *testing.T) {
	tests := []struct {
		name string
		hex1 string
		hex2 string
		min  float64
	}{
		{
			name: "black and white",
			hex1: "#000000",
			hex2: "#FFFFFF",
			min:  21.0,
		},
		{
			name: "same color",
			hex1: "#FFFFFF",
			hex2: "#FFFFFF",
			min:  1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ContrastRatio(tt.hex1, tt.hex2)
			if err != nil {
				t.Errorf("ContrastRatio() error = %v", err)
				return
			}
			if got < tt.min {
				t.Errorf("ContrastRatio() = %v, want >= %v", got, tt.min)
			}
		})
	}
}

func TestMeetsWCAGAA(t *testing.T) {
	tests := []struct {
		name       string
		foreground string
		background string
		minRatio   float64
		want       bool
	}{
		{
			name:       "white on black meets AA",
			foreground: "#FFFFFF",
			background: "#000000",
			minRatio:   4.5,
			want:       true,
		},
		{
			name:       "gray on white does not meet AA",
			foreground: "#AAAAAA",
			background: "#FFFFFF",
			minRatio:   4.5,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MeetsWCAGAA(tt.foreground, tt.background, tt.minRatio)
			if err != nil {
				t.Errorf("MeetsWCAGAA() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("MeetsWCAGAA() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateColorVariants(t *testing.T) {
	baseHex := "#00BFFF"
	lighterAdj := 0.2
	darkerAdj := 0.2

	lighter, darker, err := GenerateColorVariants(baseHex, lighterAdj, darkerAdj)
	if err != nil {
		t.Errorf("GenerateColorVariants() error = %v", err)
		return
	}

	if lighter == darker {
		t.Errorf("GenerateColorVariants() lighter=%v and darker=%v should be different", lighter, darker)
	}

	if lighter == baseHex || darker == baseHex {
		t.Errorf("GenerateColorVariants() variants should be different from base color")
	}
}

func TestColorStringsEqual(t *testing.T) {
	tests := []struct {
		name string
		c1   string
		c2   string
		want bool
	}{
		{
			name: "same colors",
			c1:   "#00BFFF",
			c2:   "#00BFFF",
			want: true,
		},
		{
			name: "same colors different case",
			c1:   "#00BFFF",
			c2:   "#00bfFF",
			want: true,
		},
		{
			name: "different colors",
			c1:   "#00BFFF",
			c2:   "#FF0000",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColorStringsEqual(tt.c1, tt.c2)
			if got != tt.want {
				t.Errorf("ColorStringsEqual(%v, %v) = %v, want %v", tt.c1, tt.c2, got, tt.want)
			}
		})
	}
}

func TestRelativeLuminance(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		min  float64
		max  float64
	}{
		{
			name: "black",
			hex:  "#000000",
			min:  0.0,
			max:  0.0,
		},
		{
			name: "white",
			hex:  "#FFFFFF",
			min:  1.0,
			max:  1.0,
		},
		{
			name: "red",
			hex:  "#FF0000",
			min:  0.2,
			max:  0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RelativeLuminance(tt.hex)
			if err != nil {
				t.Errorf("RelativeLuminance() error = %v", err)
				return
			}
			if got < tt.min || got > tt.max {
				t.Errorf("RelativeLuminance() = %v, want between %v and %v", got, tt.min, tt.max)
			}
		})
	}
}

func TestHexToRGBToHex(t *testing.T) {
	testColors := []string{
		"#000000",
		"#FFFFFF",
		"#FF0000",
		"#00FF00",
		"#0000FF",
		"#00BFFF",
		"#9B59B6",
	}

	for _, original := range testColors {
		hsl, err := HexToHSL(original)
		if err != nil {
			t.Errorf("HexToHSL(%s) error = %v", original, err)
			continue
		}

		converted := HSLToHex(hsl)

		// Verify conversion produces valid hex
		if len(converted) != 7 || converted[0] != '#' {
			t.Errorf("HSLToHex produced invalid hex: %s", converted)
		}

		// Verify RGB components are in valid range
		r, g, b, err := hexToRGB(converted)
		if err != nil {
			t.Errorf("hexToRGB(%s) error = %v", converted, err)
			continue
		}
		if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 {
			t.Errorf("RGB values out of range for %s: (%d,%d,%d)", converted, r, g, b)
		}
	}
}
