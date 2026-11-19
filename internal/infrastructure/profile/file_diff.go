package profile

import (
	"fmt"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

type FileDiffRow struct {
	Before  string
	After   string
	Changed bool
}

func DiffFiles(before, after string) []FileDiffRow {
	a := strings.Split(before, "\n")
	b := strings.Split(after, "\n")
	matcher := difflib.NewMatcher(a, b)
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
