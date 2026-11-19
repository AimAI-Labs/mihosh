package profile

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRuleEditsReplayAfterStandardOverrides(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	merge := []byte("# keep these overrides\nprepend-rules:\n  - DOMAIN,first.example,DIRECT\nmode: rule\n")
	for _, edit := range []RuleEdit{
		{Rule: "DOMAIN,added.example,DIRECT", Index: 2},
		{Match: "DOMAIN,original.example,DIRECT", Rule: "DOMAIN,changed.example,REJECT"},
		{Match: "IP-CIDR,192.0.2.0/24,DIRECT,no-resolve"},
	} {
		var err error
		merge, err = AppendRuleEdit(merge, edit)
		require.NoError(t, err)
	}
	require.Contains(t, string(merge), "# keep these overrides")
	require.NoError(t, WriteMerge(merge))
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
