package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/stretchr/testify/require"
)

func TestActivateAndProxyOutputs(t *testing.T) {
	i18n.Init()
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	var out bytes.Buffer
	activate := newActivateCommand()
	activate.SetOut(&out)
	activate.SetArgs([]string{"zsh", "--output", "json"})
	require.NoError(t, activate.Execute())
	var activation struct{ Shell, Script string }
	require.NoError(t, json.Unmarshal(out.Bytes(), &activation))
	require.Equal(t, "zsh", activation.Shell)
	require.Contains(t, activation.Script, "mihosh() {")

	out.Reset()
	proxy := newProxyCommand()
	proxy.SetOut(&out)
	proxy.SetArgs([]string{"off", "--output", "json"})
	require.NoError(t, proxy.Execute())
	var result struct {
		Enabled bool
		Script  string
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	require.False(t, result.Enabled)
	require.Contains(t, result.Script, "unset HTTPS_PROXY\n")
}

func TestWriteProxyProbeResultsIncludesExitMetadata(t *testing.T) {
	i18n.Init()
	i18n.SetLanguageOverride("en-US")
	t.Cleanup(func() { i18n.SetLanguageOverride("") })
	results := []service.ProxyProbeResult{
		{Source: "ip.sb", IP: "2001:db8::1", Country: "Japan", City: "Tokyo", ASN: 64496, Org: "Example Network", LatencyMS: 20},
		{Source: "ipify", IP: "192.0.2.1", LatencyMS: 40},
	}
	var out bytes.Buffer
	cmd := newProxyCommand()
	cmd.SetOut(&out)
	for _, format := range []outputFormat{outputFormatPlain, outputFormatTable} {
		out.Reset()
		require.NoError(t, writeProxyProbeResults(cmd, results, format))
		require.Regexp(t, `ip\.sb\s+2001:db8::1\s+Japan\s+Tokyo\s+AS64496\s+Example Network\s+20 ms`, out.String())
		require.Regexp(t, `ipify\s+192\.0\.2\.1\s+40 ms`, out.String())
		require.Regexp(t, `^Source\s+IP\s+Country\s+City\s+ASN\s+Organization\s+Latency\s+Error\n`, out.String())
	}

	out.Reset()
	require.NoError(t, writeProxyProbeResults(cmd, results, outputFormatJSON))
	require.JSONEq(t, `[
		{"source":"ip.sb","ip":"2001:db8::1","country":"Japan","city":"Tokyo","asn":64496,"org":"Example Network","latency_ms":20},
		{"source":"ipify","ip":"192.0.2.1","latency_ms":40}
	]`, out.String())
}
