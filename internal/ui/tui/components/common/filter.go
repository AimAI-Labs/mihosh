package common

import "strings"

// FuzzyMatch checks if all characters of pattern appear in text sequentially, case-insensitive.
func FuzzyMatch(pattern, text string) bool {
	if pattern == "" {
		return true
	}
	patternRunes := []rune(strings.ToLower(pattern))
	textRunes := []rune(strings.ToLower(text))

	pIdx := 0
	for i := 0; i < len(textRunes) && pIdx < len(patternRunes); i++ {
		if textRunes[i] == patternRunes[pIdx] {
			pIdx++
		}
	}
	return pIdx == len(patternRunes)
}
