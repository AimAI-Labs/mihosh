package profile

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/pmezard/go-difflib/difflib"
)

type FileDiffRow struct {
	Before  string
	After   string
	Changed bool
}

type FileDiffOptions uint8

const (
	DiffIgnoreIndent FileDiffOptions = 1 << iota
	DiffIgnoreWhitespace
	DiffIgnoreFormatCharacters
	DiffIgnorePunctuation
)

func DiffFiles(before, after string, options FileDiffOptions) []FileDiffRow {
	a := strings.Split(strings.ReplaceAll(before, "\r\n", "\n"), "\n")
	b := strings.Split(strings.ReplaceAll(after, "\r\n", "\n"), "\n")
	matcher := difflib.NewMatcher(diffKeys(a, options), diffKeys(b, options))
	var rows []FileDiffRow
	for _, group := range matcher.GetGroupedOpCodes(3) {
		first, last := group[0], group[len(group)-1]
		header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", first.I1+1, last.I2-first.I1, first.J1+1, last.J2-first.J1)
		rows = append(rows, FileDiffRow{Before: header, After: header})
		for _, chunk := range group {
			oldLen, newLen := chunk.I2-chunk.I1, chunk.J2-chunk.J1
			for i := 0; i < max(oldLen, newLen); i++ {
				row := FileDiffRow{Changed: chunk.Tag != 'e'}
				if i < oldLen {
					row.Before = "  " + a[chunk.I1+i]
					if row.Changed {
						row.Before = "- " + a[chunk.I1+i]
					}
				}
				if i < newLen {
					row.After = "  " + b[chunk.J1+i]
					if row.Changed {
						row.After = "+ " + b[chunk.J1+i]
					}
				}
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func diffKeys(lines []string, options FileDiffOptions) []string {
	if options == 0 {
		return lines
	}
	keys := make([]string, len(lines))
	for i, line := range lines {
		if options&DiffIgnoreFormatCharacters != 0 {
			line = strings.Map(func(r rune) rune {
				if unicode.Is(unicode.Cf, r) || (unicode.IsControl(r) && !unicode.IsSpace(r)) {
					return -1
				}
				return r
			}, line)
		}
		text := strings.TrimLeft(line, " \t")
		indent := line[:len(line)-len(text)]
		if options&DiffIgnoreIndent != 0 {
			indent = ""
		}
		if options&DiffIgnoreWhitespace != 0 {
			text = strings.Map(func(r rune) rune {
				if unicode.IsSpace(r) {
					return -1
				}
				return r
			}, text)
		}
		if options&DiffIgnorePunctuation != 0 {
			text = strings.Map(func(r rune) rune {
				if unicode.IsPunct(r) || unicode.IsSymbol(r) {
					return -1
				}
				return r
			}, text)
		}
		keys[i] = indent + text
	}
	return keys
}
