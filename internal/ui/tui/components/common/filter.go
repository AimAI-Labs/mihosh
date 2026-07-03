package common

import "strings"

// FuzzyMatch checks if all characters of pattern appear in text sequentially, case-insensitive.
func FuzzyMatch(pattern, text string) bool {
	if pattern == "" {
		return true
	}
	pattern = strings.ToLower(pattern)
	text = strings.ToLower(text)

	pIdx := 0
	for i := 0; i < len(text) && pIdx < len(pattern); i++ {
		if text[i] == pattern[pIdx] {
			pIdx++
		}
	}
	return pIdx == len(pattern)
}
