package profile

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRuleEditsReplayAfterStandardOverrides(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	merge := []byte(`# keep these overrides
prepend-rules:
  - DOMAIN,first.example,DIRECT
mode: rule
rule-edits:
  - rule: DOMAIN,added.example,DIRECT
    index: 2
  - match: DOMAIN,original.example,DIRECT
    rule: DOMAIN,changed.example,REJECT
  - match: IP-CIDR,192.0.2.0/24,DIRECT,no-resolve
`)
	path, err := MergePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, merge, 0600))
	preview, err := GenerateSourcePreview([]byte("rules:\n  - Domain, original.example,DIRECT\n  - IP-CIDR,192.0.2.0/24,DIRECT,NO-RESOLVE\n  - MATCH,DIRECT\n"))
	require.NoError(t, err)
	var cfg struct{ Rules []string }
	require.NoError(t, yaml.Unmarshal(preview.Data, &cfg))
	require.Equal(t, []string{
		"DOMAIN,first.example,DIRECT", "DOMAIN,added.example,DIRECT", "DOMAIN,changed.example,REJECT", "MATCH,DIRECT",
	}, cfg.Rules)
	require.NotContains(t, string(preview.Data), "rule-edits")

	refreshed, err := GenerateSourcePreview([]byte("rules:\n  - DOMAIN,new-upstream.example,DIRECT\n  - MATCH,DIRECT\n"))
	require.NoError(t, err)
	require.Contains(t, string(refreshed.Data), "DOMAIN,new-upstream.example,DIRECT")
	require.NotContains(t, string(refreshed.Data), "DOMAIN,changed.example,REJECT")
	_, err = ApplyRuleEditToYAML(refreshed.Data, RuleEdit{Match: "DOMAIN,missing.example,DIRECT"})
	require.ErrorIs(t, err, ErrRuleEditTarget)
}

func TestNormalizeLegacyRulesKeepsDeltasAcrossSubscriptionRefresh(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	legacy := []byte("# keep\nmode: rule\nprepend-rules: ['DOMAIN,first.test,DIRECT']\nrule-edits: [{match: 'DOMAIN,old.test,DIRECT', rule: 'MATCH,REJECT'}]\n")
	mergePath, err := MergePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(mergePath, legacy, 0600))

	normalized, err := NormalizeLegacyMerge(legacy, "", nil)
	require.NoError(t, err)
	require.NoError(t, WriteMerge(normalized))
	require.Contains(t, string(normalized), "# keep")
	require.NotContains(t, string(normalized), "rules:")
	require.NotContains(t, string(normalized), "rule-edits:")

	preview, err := GenerateSourcePreview([]byte("rules: ['DOMAIN,old.test,DIRECT', 'DOMAIN,new.test,DIRECT', 'MATCH,DIRECT']\n"))
	require.NoError(t, err)
	require.NoError(t, preview.MergeErr)
	var doc struct{ Rules []string }
	require.NoError(t, yaml.Unmarshal(preview.Data, &doc))
	require.Equal(t, []string{"DOMAIN,first.test,DIRECT", "MATCH,REJECT", "DOMAIN,new.test,DIRECT", "MATCH,DIRECT"}, doc.Rules)
}

func TestWriteMergeRejectsNonNativeFieldsAndInvalidLists(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	original := []byte("mode: rule\n")
	require.NoError(t, WriteMerge(original))
	for _, raw := range []string{
		"prepend-rules: []", "rule-edits: []", "unknown: true", "rules: {}",
		"proxies: [invalid]", "mode: rule\nmode: global", "mode: rule\n---\nmode: global",
	} {
		require.Error(t, WriteMerge([]byte(raw)), raw)
		stored, err := ReadMerge()
		require.NoError(t, err)
		require.Equal(t, original, stored)
	}
}
