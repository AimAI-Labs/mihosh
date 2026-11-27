package preferences

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestCoreActionCommandsBindControllerAndHandleMissingClient(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	commands := []func(*service.Client) tea.Cmd{
		func(client *service.Client) tea.Cmd { return UpgradeCoreCmd(client, "release") },
		RestartCoreCmd, ReloadConfigsCmd, UpdateGeoDataCmd, FlushDNSCacheCmd, FlushFakeIPCmd,
	}
	for _, action := range commands {
		client := api.NewClient(config.MihomoEndpoint{ExternalController: server.URL}, 1000)
		generation := client.Generation()
		cmd := action(client)
		require.NotNil(t, cmd)
		client.UpdateEndpoint("http://127.0.0.1:0", "")
		got, ok := cmd().(messages.CoreHTTPMsg)
		require.True(t, ok, "action completed without a controller generation")
		require.Equal(t, generation, got.Generation)
		require.IsType(t, messages.CoreActionDoneMsg{}, got.Payload)
		require.Nil(t, action(nil))
	}
	require.EqualValues(t, len(commands), requests.Load())
	state, cmd := (State{}).RunCoreAction("restart")
	require.False(t, state.IsCoreRestarting)
	require.Nil(t, cmd)
}
