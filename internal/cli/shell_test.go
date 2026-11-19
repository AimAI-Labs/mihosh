package cli

import (
	"bytes"
	"encoding/json"
	"testing"

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
