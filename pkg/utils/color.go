package utils

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type HSL struct {
	H float64
	S float64
	L float64
}

func HexToHSL(hex string) (HSL, error) {
	r, g, b, err := hexToRGB(hex)
	if err != nil {
		return HSL{}, err
	}

	rNorm := float64(r) / 255.0
	gNorm := float64(g) / 255.0
	bNorm := float64(b) / 255.0

	max := math.Max(rNorm, math.Max(gNorm, bNorm))
	min := math.Min(rNorm, math.Min(gNorm, bNorm))
	delta := max - min

	l := (max + min) / 2.0
	s := 0.0
	h := 0.0

	if delta != 0 {
		s = l / 0.5
		if s < 1 {
			s = delta / (2.0 - max - min)
		} else {
			s = delta / (max + min)
		}

		switch max {
		case rNorm:
			h = (gNorm - bNorm) / delta
			if gNorm < bNorm {
				h += 6.0
			}
		case gNorm:
			h = (bNorm-rNorm)/delta + 2.0
		case bNorm:
			h = (rNorm-gNorm)/delta + 4.0
		}
		h /= 6.0
	}

	return HSL{H: h * 360.0, S: s, L: l}, nil
}

func HSLToHex(hsl HSL) string {
	h := hsl.H / 360.0
	s := hsl.S
	l := hsl.L

	var r, g, b float64

	if s == 0 {
		r, g, b = l, l, l
	} else {
		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q

		r = hueToRGB(p, q, h+1.0/3.0)
		g = hueToRGB(p, q, h)
		b = hueToRGB(p, q, h-1.0/3.0)
	}

	return rgbToHex(int(r*255), int(g*255), int(b*255))
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1.0
	}
	if t > 1 {
		t -= 1.0
	}
	if t < 1.0/6.0 {
		return p + (q-p)*6.0*t
	}
	if t < 1.0/2.0 {
		return q
	}
	if t < 2.0/3.0 {
		return p + (q-p)*(2.0/3.0-t)*6.0
	}
	return p
}

func hexToRGB(hex string) (int, int, int, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string(hex[0]) + string(hex[0]) + string(hex[1]) + string(hex[1]) + string(hex[2]) + string(hex[2])
	}
	if len(hex) != 6 {
		return 0, 0, 0, nil
	}

	r := 0
	g := 0
	b := 0
	_, err := parseHexComponent(hex[0:2], &r)
	if err != nil {
		return 0, 0, 0, err
	}
	_, err = parseHexComponent(hex[2:4], &g)
	if err != nil {
		return 0, 0, 0, err
	}
	_, err = parseHexComponent(hex[4:6], &b)
	if err != nil {
		return 0, 0, 0, err
	}

	return r, g, b, nil
}

func parseHexComponent(s string, v *int) (int, error) {
	var val int
	_, err := hexToInt(s[0], &val)
	if err != nil {
		return 0, err
	}
	*v = val * 16
	_, err = hexToInt(s[1], &val)
	if err != nil {
		return 0, err
	}
	*v += val
	return *v, nil
}

func hexToInt(c byte, v *int) (int, error) {
	switch {
	case c >= '0' && c <= '9':
		*v = int(c - '0')
	case c >= 'A' && c <= 'F':
		*v = int(c - 'A' + 10)
	case c >= 'a' && c <= 'f':
		*v = int(c - 'a' + 10)
	default:
		*v = 0
	}
	return *v, nil
}

func rgbToHex(r, g, b int) string {
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func LighterColor(hex string, adjustment float64) (string, error) {
	hsl, err := HexToHSL(hex)
	if err != nil {
		return hex, err
	}
	hsl.L = math.Min(1.0, hsl.L+adjustment)
	return HSLToHex(hsl), nil
}

func DarkerColor(hex string, adjustment float64) (string, error) {
	hsl, err := HexToHSL(hex)
	if err != nil {
		return hex, err
	}
	hsl.L = math.Max(0.0, hsl.L-adjustment)
	return HSLToHex(hsl), nil
}

func ContrastRatio(hex1, hex2 string) (float64, error) {
	l1, err := RelativeLuminance(hex1)
	if err != nil {
		return 0, err
	}
	l2, err := RelativeLuminance(hex2)
	if err != nil {
		return 0, err
	}

	var lighter, darker float64
	if l1 > l2 {
		lighter = l1
		darker = l2
	} else {
		lighter = l2
		darker = l1
	}

	return (lighter + 0.05) / (darker + 0.05), nil
}

func RelativeLuminance(hex string) (float64, error) {
	r, g, b, err := hexToRGB(hex)
	if err != nil {
		return 0, err
	}

	rNorm := sRGBToLinear(float64(r) / 255.0)
	gNorm := sRGBToLinear(float64(g) / 255.0)
	bNorm := sRGBToLinear(float64(b) / 255.0)

	return 0.2126*rNorm + 0.7152*gNorm + 0.0722*bNorm, nil
}

func sRGBToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func MeetsWCAGAA(foreground, background string, minRatio float64) (bool, error) {
	ratio, err := ContrastRatio(foreground, background)
	if err != nil {
		return false, err
	}
	return ratio >= minRatio, nil
}

func GenerateColorVariants(baseHex string, lighterAdj, darkerAdj float64) (lighter, darker string, err error) {
	lighter, err = LighterColor(baseHex, lighterAdj)
	if err != nil {
		return "", "", err
	}
	darker, err = DarkerColor(baseHex, darkerAdj)
	if err != nil {
		return "", "", err
	}
	return lighter, darker, nil
}

func ColorStringsEqual(c1, c2 string) bool {
	return strings.EqualFold(c1, c2)
}

func GetDelayColor(delay int) lipgloss.Color {
	switch {
	case delay == 0:
		return lipgloss.Color("#565f89")
	case delay < 100:
		return lipgloss.Color("#9ECE6A")
	case delay < 300:
		return lipgloss.Color("#E0AF68")
	default:
		return lipgloss.Color("#F7768E")
	}
}
