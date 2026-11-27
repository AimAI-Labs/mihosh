package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const sampleChainRawYAML = `mixed-port: 7890
proxies:
  - name: HK-Entry
    type: ss
    server: 1.1.1.1
    port: 443
  - name: SG-Relay
    type: vmess
    server: 2.2.2.2
    port: 443
  - name: US-Exit
    type: trojan
    server: 3.3.3.3
    port: 443
  - name: User-Custom
    type: ss
    server: 4.4.4.4
    port: 443
    dialer-proxy: Existing-Upstream
proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - HK-Entry
      - SG-Relay
      - US-Exit
      - User-Custom
  - name: RELAY-GRP
    type: url-test
    proxies:
      - HK-Entry
rules:
  - MATCH,PROXY
`

func extractProxyDialers(t *testing.T, data []byte) map[string]string {
	t.Helper()
	var doc struct {
		Proxies []struct {
			Name        string `yaml:"name"`
			DialerProxy string `yaml:"dialer-proxy"`
		} `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(data, &doc))
	out := make(map[string]string, len(doc.Proxies))
	for _, p := range doc.Proxies {
		out[p.Name] = p.DialerProxy
	}
	return out
}

func TestApplyChainEditsInjectsDialerProxyAndPreservesUnrelated(t *testing.T) {
	edits := ChainEdits{
		Chains: []ProxyChain{{
			Group:   "PROXY",
			Nodes:   []string{"HK-Entry", "SG-Relay", "US-Exit"},
			Enabled: true,
		}},
	}

	out, err := ApplyChainEditsToYAML([]byte(sampleChainRawYAML), ChainEdits{}, edits, true)
	require.NoError(t, err)

	dialers := extractProxyDialers(t, out)
	require.Equal(t, "", dialers["HK-Entry"])
	require.Equal(t, "HK-Entry", dialers["SG-Relay"])
	require.Equal(t, "SG-Relay", dialers["US-Exit"])
	// Existing user-configured dialer-proxy on unrelated proxy must be preserved.
	require.Equal(t, "Existing-Upstream", dialers["User-Custom"])

	// Reverting the chain in standalone config removes only chain-owned dialer-proxy fields.
	reverted, err := ApplyChainEditsToYAML(out, edits, ChainEdits{}, true)
	require.NoError(t, err)
	dialers = extractProxyDialers(t, reverted)
	require.Equal(t, "", dialers["HK-Entry"])
	require.Equal(t, "", dialers["SG-Relay"])
	require.Equal(t, "", dialers["US-Exit"])
	require.Equal(t, "Existing-Upstream", dialers["User-Custom"])
}

func TestChainEditsAllowGroupAsEntryOnly(t *testing.T) {
	// Proxy group allowed as entry hop (#1).
	okEdits := ChainEdits{
		Chains: []ProxyChain{{
			Group:   "PROXY",
			Nodes:   []string{"RELAY-GRP", "US-Exit"},
			Enabled: true,
		}},
	}
	out, err := ApplyChainEditsToYAML([]byte(sampleChainRawYAML), ChainEdits{}, okEdits, true)
	require.NoError(t, err)
	dialers := extractProxyDialers(t, out)
	require.Equal(t, "RELAY-GRP", dialers["US-Exit"])

	// Proxy group rejected as non-entry hop in strict mode.
	badEdits := ChainEdits{
		Chains: []ProxyChain{{
			Group:   "PROXY",
			Nodes:   []string{"HK-Entry", "RELAY-GRP"},
			Enabled: true,
		}},
	}
	_, err = ApplyChainEditsToYAML([]byte(sampleChainRawYAML), ChainEdits{}, badEdits, true)
	require.ErrorIs(t, err, ErrChainProxyNotFound)
}

func TestChainEditsValidationRejectsInvalidCyclesAndConflicts(t *testing.T) {
	var edits ChainEdits
	// Too few nodes
	require.ErrorIs(t, edits.SetGroupChain(ProxyChain{Group: "PROXY", Nodes: []string{"HK-Entry"}, Enabled: true}), ErrInvalidProxyChain)
	// Duplicate nodes
	require.ErrorIs(t, edits.SetGroupChain(ProxyChain{Group: "PROXY", Nodes: []string{"HK-Entry", "HK-Entry"}, Enabled: true}), ErrInvalidProxyChain)
	// Built-in policy
	require.ErrorIs(t, edits.SetGroupChain(ProxyChain{Group: "PROXY", Nodes: []string{"DIRECT", "US-Exit"}, Enabled: true}), ErrInvalidProxyChain)

	// Valid first chain: HK-Entry -> US-Exit
	require.NoError(t, edits.SetGroupChain(ProxyChain{Group: "G1", Nodes: []string{"HK-Entry", "US-Exit"}, Enabled: true}))
	// Conflicting target on another group: SG-Relay -> US-Exit
	require.ErrorIs(t, edits.SetGroupChain(ProxyChain{Group: "G2", Nodes: []string{"SG-Relay", "US-Exit"}, Enabled: true}), ErrChainConflict)
	// Cycle across groups: US-Exit -> HK-Entry
	require.ErrorIs(t, edits.SetGroupChain(ProxyChain{Group: "G2", Nodes: []string{"US-Exit", "HK-Entry"}, Enabled: true}), ErrChainCycle)
}

func TestChainEditsReplayInGenerateForUID(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	require.NoError(t, WriteRaw("sub-chain", []byte(sampleChainRawYAML)))
	require.NoError(t, WriteChainEdits(ChainEdits{
		Chains: []ProxyChain{{
			Group:   "PROXY",
			Nodes:   []string{"HK-Entry", "US-Exit"},
			Enabled: true,
		}},
	}))

	generated, err := GenerateForUID("sub-chain", "")
	require.NoError(t, err)
	dialers := extractProxyDialers(t, generated.Data)
	require.Equal(t, "HK-Entry", dialers["US-Exit"])
	require.Equal(t, "Existing-Upstream", dialers["User-Custom"])

	// Updating chain via GenerateOverrideConfigWithChains reflects cleared chain before disk write.
	prev, err := ReadChainEdits()
	require.NoError(t, err)
	clearedBytes, err := GenerateOverrideConfigWithChains(Profile{UID: "sub-chain"}, "", nil, false, prev, ChainEdits{})
	require.NoError(t, err)
	dialers = extractProxyDialers(t, clearedBytes)
	require.Equal(t, "", dialers["US-Exit"])
	require.Equal(t, "Existing-Upstream", dialers["User-Custom"])

	// Standalone config path (uid == "") also works with GenerateOverrideConfigWithChains.
	standalonePath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(standalonePath, generated.Data, 0600))
	standaloneCleared, err := GenerateOverrideConfigWithChains(Profile{}, standalonePath, nil, false, prev, ChainEdits{})
	require.NoError(t, err)
	dialers = extractProxyDialers(t, standaloneCleared)
	require.Equal(t, "", dialers["US-Exit"])
	require.Equal(t, "Existing-Upstream", dialers["User-Custom"])
}
