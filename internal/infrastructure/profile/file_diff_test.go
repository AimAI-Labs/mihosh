package profile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiffFormattingOptionsAreIndependent(t *testing.T) {
	for _, tc := range []struct {
		name     string
		old, new string
		option   FileDiffOptions
	}{
		{"indent", "  port: 7890\n", "    port: 7890\n", DiffIgnoreIndent},
		{"whitespace", "  port:  7890 \n", "  port: 7890\n", DiffIgnoreWhitespace},
		{"format", "\ufeffmode: ru\u200ble\n", "mode: rule\n", DiffIgnoreFormatCharacters},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NotEmpty(t, DiffFiles(tc.old, tc.new, 0))
			require.Empty(t, DiffFiles(tc.old, tc.new, tc.option))
			all := DiffIgnoreIndent | DiffIgnoreWhitespace | DiffIgnoreFormatCharacters
			require.NotEmpty(t, DiffFiles(tc.old, tc.new, all&^tc.option))
		})
	}
	all := DiffIgnoreIndent | DiffIgnoreWhitespace | DiffIgnoreFormatCharacters
	rows := DiffFiles("    mode: rule\n    name: 'test'\n", "  mode: rule\n  name: test\n", all)
	require.Len(t, rows, 4)
	require.Equal(t, FileDiffRow{Before: "      mode: rule", After: "    mode: rule"}, rows[1])
	require.Equal(t, FileDiffRow{Before: "-     name: 'test'", After: "+   name: test", Changed: true}, rows[2])
}
