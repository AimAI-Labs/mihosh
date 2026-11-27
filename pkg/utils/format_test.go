package utils

import "testing"

func TestMaskSecretLimitsVisibleCharacters(t *testing.T) {
	for _, tc := range []struct{ secret, want string }{
		{"", ""},
		{"1234567", "****"},
		{"12345678", "****"},
		{"abcdefghi", "ab****hi"},
		{"甲乙丙丁戊己庚辛壬", "甲乙****辛壬"},
	} {
		if got := MaskSecret(tc.secret); got != tc.want {
			t.Errorf("MaskSecret() = %q, want %q", got, tc.want)
		}
	}
}
