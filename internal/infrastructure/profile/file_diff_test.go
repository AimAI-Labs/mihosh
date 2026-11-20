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
		{"punctuation", "name: 'test'\n", "name: test\n", DiffIgnorePunctuation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NotEmpty(t, DiffFiles(tc.old, tc.new, 0))
			require.Empty(t, DiffFiles(tc.old, tc.new, tc.option))
			all := DiffIgnoreIndent | DiffIgnoreWhitespace | DiffIgnoreFormatCharacters | DiffIgnorePunctuation
			require.NotEmpty(t, DiffFiles(tc.old, tc.new, all&^tc.option))
		})
	}
	all := DiffIgnoreIndent | DiffIgnoreWhitespace | DiffIgnoreFormatCharacters
	rows := DiffFiles("    mode: rule\n    name: 'test'\n", "  mode: rule\n  name: test\n", all)
	require.Len(t, rows, 4)
	require.Equal(t, FileDiffRow{Before: "      mode: rule", After: "    mode: rule"}, rows[1])
	require.Equal(t, FileDiffRow{Before: "-     name: 'test'", After: "+   name: test", Changed: true}, rows[2])
}

func TestDiffPreservesUnicodeValuesInGeneratedPreview(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	source := []byte("proxies:\n  - name: \"🚀 Café\"\n")
	generated, err := GenerateSourcePreview(source)
	require.NoError(t, err)
	require.NoError(t, generated.MergeErr)
	require.Equal(t, string(source), string(generated.Data))
	require.Empty(t, DiffFiles(string(source), "proxies:\n  - name: \"\\U0001f680 Caf\\u00e9\"\n", 0))
	require.NotEmpty(t, DiffFiles(string(source), "proxies:\n  - name: \"🌙 Café\"\n", 0))
}

func TestReadableUnicodeLeavesLiteralEscapesAndControls(t *testing.T) {
	source := []byte(`# \U0001F680
plain: '\U0001F680'
escaped: "\\U0001F680"
tagged: !!str "\U0001F680\u000a"
`)
	want := []byte(`# \U0001F680
plain: '\U0001F680'
escaped: "\\U0001F680"
tagged: !!str "🚀\u000a"
`)
	require.Equal(t, want, readableYAMLUnicode(source))
}
