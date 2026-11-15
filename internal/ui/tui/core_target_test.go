package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func targetTestModel(t *testing.T) (Model, string) {
	t.Helper()
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	cfg := config.DefaultConfig
	cfg.Core.Mode = "managed"
	require.NoError(t, config.Save(&cfg))
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("external-controller: 127.0.0.1:9090\nsecret: old\nmixed-port: 7890\n"), 0600))
	m := NewModel(api.NewClient(config.ResolveMihomoEndpoint(), 50), "", 50)
	t.Cleanup(m.Close)
	m.currentPage = layout.PageSub
	m.connectionEpoch = 7
	m.connectionReady = true
	m.chartData.AddMemoryData(1234)
	m.nodesState.Groups = map[string]model.Group{"old": {}}
	m.logsState = m.logsState.AppendLog("info", "old controller")
	return m, path
}

func TestTargetChangeInvalidatesOldHealthAndPageData(t *testing.T) {
	for _, entry := range []struct {
		name string
		msg  tea.Msg
	}{
		{"settings-loaded", messages.MihomoConfigMsg{Config: &model.MihomoConfig{ExternalController: "127.0.0.1:9091", Secret: "new", MixedPort: 7891}}},
		{"settings-saved", messages.MihomoConfigSavedMsg{WriteOK: true}},
		{"settings-reload-failed", messages.MihomoConfigSavedMsg{WriteOK: true, Err: errors.New("old endpoint closed")}},
		{"connect", messages.CoreOperationMsg{Action: "connect"}},
		{"ports", messages.CoreOperationMsg{Action: "ports"}},
		{"profile", messages.CoreOperationMsg{Action: "profile"}},
		{"owner", messages.CoreOwnerChangedMsg{Action: "restart"}},
	} {
		t.Run(entry.name, func(t *testing.T) {
			m, path := targetTestModel(t)
			client, ws, chart := m.client, m.wsClient, m.chartData
			generation := ws.Generation()
			require.NoError(t, os.WriteFile(path, []byte("external-controller: 127.0.0.1:9091\nsecret: new\nmixed-port: 7891\n"), 0600))
			updated, cmd := m.Update(entry.msg)
			next := updated.(Model)
			require.Greater(t, next.connectionEpoch, m.connectionEpoch)
			require.NotNil(t, cmd)
			require.False(t, next.connectionReady)
			require.Same(t, client, next.client)
			require.Same(t, ws, next.wsClient)
			require.Greater(t, ws.Generation(), generation)
			require.Equal(t, "127.0.0.1:9091", client.EndpointSnapshot().ExternalController)
			require.Equal(t, "new", client.EndpointSnapshot().Secret)
			require.Same(t, chart, next.chartData)
			require.Empty(t, next.chartData.MemoryHistory)
			require.Empty(t, next.nodesState.Groups)
			require.Empty(t, next.logsState.Logs())
			updated, cmd = next.Update(messages.CoreConnectionMsg{Epoch: m.connectionEpoch, Err: errors.New("old health failed")})
			require.Equal(t, layout.PageSub, updated.(Model).currentPage)
			require.Nil(t, cmd)
		})
	}
}

func TestUnchangedSettingsDoNotInvalidateTarget(t *testing.T) {
	m, _ := targetTestModel(t)
	generation := m.wsClient.Generation()
	updated, cmd := m.Update(messages.MihomoConfigMsg{Config: &model.MihomoConfig{ExternalController: "127.0.0.1:9090", Secret: "old", MixedPort: 7891}})
	next := updated.(Model)
	require.Equal(t, m.connectionEpoch, next.connectionEpoch)
	require.Equal(t, generation, next.wsClient.Generation())
	require.True(t, next.connectionReady)
	require.NotEmpty(t, next.chartData.MemoryHistory)
	require.NotEmpty(t, next.nodesState.Groups)
	require.NotEmpty(t, next.logsState.Logs())
	require.Nil(t, cmd)
}

func TestRejectedTargetChangesLeaveConnectionAlone(t *testing.T) {
	for _, msg := range []tea.Msg{
		messages.MihomoConfigSavedMsg{Err: errors.New("write failed")},
		messages.CoreOperationMsg{Action: "connect", Err: errors.New("connect failed")},
		messages.CoreOperationMsg{Action: "connect", ID: 123},
	} {
		m, _ := targetTestModel(t)
		updated, _ := m.Update(msg)
		next := updated.(Model)
		require.Equal(t, m.connectionEpoch, next.connectionEpoch)
		require.True(t, next.connectionReady)
		require.NotEmpty(t, next.chartData.MemoryHistory)
	}
}

func TestStoppedOwnerRejectsQueuedHealthyResult(t *testing.T) {
	m, _ := targetTestModel(t)
	updated, _ := m.Update(messages.CoreOwnerChangedMsg{Action: "stop"})
	next := updated.(Model)
	require.Greater(t, next.connectionEpoch, m.connectionEpoch)
	require.True(t, next.connectionChecked)
	require.False(t, next.connectionReady)
	updated, cmd := next.Update(messages.CoreConnectionMsg{Epoch: m.connectionEpoch, Version: "old"})
	require.False(t, updated.(Model).connectionReady)
	require.Nil(t, cmd)
}

func TestStoppedManagedStatusClearsReadyConnection(t *testing.T) {
	m, _ := targetTestModel(t)
	updated, cmd := m.Update(messages.CoreStatusMsg{
		Status: m.coreState.Status,
	})
	// Mode != "managed" in empty status leaves connection alone
	require.True(t, updated.(Model).connectionReady)
	require.Nil(t, cmd)

	status := m.coreState.Status
	status.Mode = "managed"
	status.Running = false
	updated, cmd = updated.(Model).Update(messages.CoreStatusMsg{Status: status})
	next := updated.(Model)
	require.Greater(t, next.connectionEpoch, m.connectionEpoch)
	require.True(t, next.connectionChecked)
	require.False(t, next.connectionReady)
	require.Nil(t, cmd)
}
