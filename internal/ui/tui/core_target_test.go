package tui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/bootstrap"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func targetTestModel(t *testing.T) (Model, string) {
	t.Helper()
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	cfg := config.DefaultConfig
	cfg.Core.Mode = "managed"
	cfg.Timeout = 50
	require.NoError(t, config.Save(&cfg))
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("external-controller: 127.0.0.1:9090\nsecret: old\nmixed-port: 7890\n"), 0600))
	m := NewModel(bootstrap.NewApp())
	t.Cleanup(m.Close)
	m.currentPage = layout.PageSub
	m.connectionEpoch = 7
	m.connectionReady = true
	m.chartData.AddMemoryData(1234)
	m.nodesState.Groups = map[string]model.Group{"old": {}}
	m.logsState = m.logsState.AppendLog("info", "old controller")
	return m, path
}

func resolvedTestTarget(t *testing.T, m Model) (Model, tea.Cmd) {
	t.Helper()
	require.NotNil(t, m.targetPending)
	cfg, err, target := bootstrap.DiscoverTarget()
	require.NoError(t, err)
	next, cmd := m.Update(messages.CoreTargetResolvedMsg{Request: m.targetRequest, Target: target, Config: cfg})
	return next.(Model), cmd
}

func TestTargetChangeInvalidatesOldHealthAndPageData(t *testing.T) {
	for _, entry := range []struct {
		name string
		msg  tea.Msg
	}{
		{"settings-loaded", messages.MihomoConfigMsg{Target: &config.MihomoTarget{Endpoint: config.MihomoEndpoint{ExternalController: "127.0.0.1:9091", Secret: "new", MixedPort: 7891}}}},
		{"settings-saved", messages.MihomoConfigSavedMsg{WriteOK: true}},
		{"settings-reload-failed", messages.MihomoConfigSavedMsg{WriteOK: true, Err: errors.New("old endpoint closed")}},
		{"override-restarted", messages.MihomoOverridesSavedMsg{WriteOK: true, Applied: true, Restarted: true}},
		{"merge-restarted", messages.SubMergeAppliedMsg{Restarted: true}},
		{"activation-restarted", messages.SubActivatedMsg{Restarted: true}},
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
			if entry.name != "settings-loaded" {
				require.Equal(t, m.connectionEpoch, next.connectionEpoch, "clients should update only after discovery")
				require.NotNil(t, cmd)
				next, cmd = resolvedTestTarget(t, next)
			}
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

func TestOverrideResultsKeepLiveTargetUntilManualRefresh(t *testing.T) {
	i18n.Init()
	for _, entry := range []struct {
		name    string
		msg     tea.Msg
		pending bool
		notice  string
	}{
		{"partial", messages.MihomoOverridesSavedMsg{WriteOK: true, Applied: true, EndpointChanged: true, NeedsRestart: true},
			true, "settings.toast.mihomo_override_needs_restart_partial"},
		{"remote", messages.MihomoOverridesSavedMsg{WriteOK: true, Applied: true, EndpointChanged: true, Remote: true, RemoteNotPersisted: true},
			false, "settings.toast.mihomo_override_remote_applied"},
		{"rollback", messages.MihomoOverridesSavedMsg{EndpointChanged: true, Restored: true, Err: errors.New("reload rejected")},
			false, ""},
		{"merge-partial", messages.SubMergeAppliedMsg{NeedsRestart: true},
			true, "settings.toast.mihomo_override_needs_restart_partial"},
	} {
		t.Run(entry.name, func(t *testing.T) {
			m, path := targetTestModel(t)
			wsGeneration := m.wsClient.Generation()
			require.NoError(t, os.WriteFile(path, []byte("external-controller: 127.0.0.1:9091\nsecret: new\nmixed-port: 7891\n"), 0600))
			updated, cmd := m.Update(entry.msg)
			next := updated.(Model)
			if entry.pending {
				require.NotNil(t, cmd)
				next, _ = resolvedTestTarget(t, next)
			}
			require.Equal(t, m.connectionEpoch, next.connectionEpoch)
			require.Equal(t, wsGeneration, next.wsClient.Generation())
			require.Equal(t, "old", next.client.EndpointSnapshot().Secret)
			require.True(t, next.connectionReady)
			if entry.notice == "" {
				require.Equal(t, messages.NoticeError, next.notice.Message.Level)
				require.Contains(t, next.notice.Message.Detail, i18n.T("settings.toast.mihomo_override_restored"))
			} else {
				require.Equal(t, i18n.T(entry.notice), next.notice.Message.Text)
			}
			if entry.pending {
				next.currentPage = layout.PageCore
				refreshed, cmd := next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
				after := refreshed.(Model)
				require.NotNil(t, cmd)
				after, cmd = resolvedTestTarget(t, after)
				require.NotNil(t, cmd)
				require.Greater(t, after.connectionEpoch, next.connectionEpoch)
				require.Greater(t, after.wsClient.Generation(), wsGeneration)
				require.Equal(t, "new", after.client.EndpointSnapshot().Secret)
			}
		})
	}
}

func TestMergeEditorRefreshReachesMihomoForm(t *testing.T) {
	i18n.Init()
	m, _ := targetTestModel(t)
	m.width, m.height = 120, 34
	require.NoError(t, profile.WriteMerge([]byte("mixed-port: 8021\nlog-level: debug\n")))
	updated, cmd := m.Update(messages.MergeEditFinishedMsg{})
	require.NotNil(t, cmd)
	var read messages.MihomoOverridesMsg
	for _, child := range mergeEditSideEffects(t, cmd) {
		if msg, ok := child().(messages.MihomoOverridesMsg); ok {
			read = msg
		}
	}
	require.NoError(t, read.Err)
	require.Equal(t, "8021", read.Overrides["mixed-port"])
	updated, _ = updated.Update(read)
	next := updated.(Model)
	next.coreState, _ = next.coreState.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	require.Contains(t, next.coreState.Render(120, 34), "8021")
	require.Equal(t, m.connectionEpoch, next.connectionEpoch)
}

func TestMergeEditorAppliesActiveSubscriptionToRemoteCore(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	i18n.Init()
	payload := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/configs" {
			http.NotFound(w, r)
			return
		}
		var body struct{ Payload string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload <- body.Payload
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	cfg := config.DefaultConfig
	cfg.Core.Mode, cfg.Core.ExternalController = "external", server.Listener.Addr().String()
	require.NoError(t, config.Save(&cfg))
	m := newTestModel(api.NewClient(config.ResolveMihomoEndpoint(), 500), "", 500)
	t.Cleanup(m.Close)
	subscription, err := m.profileSvc.AddProfile("local", profile.SubSource{Kind: profile.SourceLocal, Path: "unused"})
	require.NoError(t, err)
	require.NoError(t, profile.WriteRaw(subscription.UID, []byte("log-level: info\nrules:\n  - MATCH,DIRECT\n")))
	require.NoError(t, m.profileSvc.SetActiveSub(subscription.UID))
	m.currentPage = layout.PageSub
	m.subState = m.subState.ApplySubs([]profile.Profile{subscription}, subscription.UID)
	require.NoError(t, profile.WriteMerge([]byte("allow-lan: false\n")))

	updated, cmd := m.Update(messages.MergeEditFinishedMsg{ClearedOverrides: []string{"log-level"}})
	var applied messages.SubMergeAppliedMsg
	for _, child := range mergeEditSideEffects(t, cmd) {
		if msg, ok := child().(messages.SubMergeAppliedMsg); ok {
			applied = msg
		}
	}
	require.NoError(t, applied.Err)
	require.Equal(t, subscription.UID, applied.UID)
	require.True(t, applied.Remote)
	select {
	case data := <-payload:
		require.Contains(t, data, "allow-lan: false")
		require.Contains(t, data, "log-level: info")
		require.Contains(t, data, "MATCH,DIRECT")
	default:
		t.Fatal("remote controller received no configuration")
	}

	updated, cmd = updated.Update(applied)
	require.Equal(t, i18n.T("settings.toast.mihomo_override_remote_applied"), updated.(Model).notice.Message.Text)
	require.NotNil(t, cmd)
	require.Equal(t, m.connectionEpoch, updated.(Model).connectionEpoch)
}

func mergeEditSideEffects(t *testing.T, cmd tea.Cmd) tea.BatchMsg {
	t.Helper()
	require.NotNil(t, cmd)
	batch, ok := cmd().(tea.BatchMsg)
	require.True(t, ok)
	require.Len(t, batch, 2)
	// The first command keeps the editor's work together, alongside the notice expiry.
	sideEffects, ok := batch[0]().(tea.BatchMsg)
	require.True(t, ok)
	return sideEffects
}

func TestUnchangedSettingsDoNotInvalidateTarget(t *testing.T) {
	m, _ := targetTestModel(t)
	generation := m.wsClient.Generation()
	updated, cmd := m.Update(messages.MihomoConfigMsg{Target: &config.MihomoTarget{Endpoint: config.MihomoEndpoint{ExternalController: "127.0.0.1:9090", Secret: "old", MixedPort: 7891}}})
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
	updated, pending := m.Update(messages.CoreOwnerChangedMsg{Action: "stop"})
	require.NotNil(t, pending)
	next, _ := resolvedTestTarget(t, updated.(Model))
	require.Greater(t, next.connectionEpoch, m.connectionEpoch)
	require.True(t, next.connectionChecked)
	require.False(t, next.connectionReady)
	updated, cmd := next.Update(messages.CoreConnectionMsg{Epoch: m.connectionEpoch, Version: "old"})
	require.False(t, updated.(Model).connectionReady)
	require.Nil(t, cmd)
}

func TestManagedStartupBindsCredentialsCreatedByFallback(t *testing.T) {
	m, path := targetTestModel(t)
	oldWS := m.wsClient.Generation()
	require.NoError(t, os.WriteFile(path, []byte("external-controller: 127.0.0.1:9090\nsecret: fallback\nmixed-port: 7890\n"), 0600))
	updated, cmd := m.Update(messages.ManagedCoreStartedMsg{Epoch: m.connectionEpoch})
	require.NotNil(t, cmd)
	resolved, ok := cmd().(messages.CoreTargetResolvedMsg)
	require.True(t, ok)
	updated, cmd = updated.Update(resolved)
	next := updated.(Model)
	require.Equal(t, "fallback", next.client.EndpointSnapshot().Secret)
	require.Greater(t, next.wsClient.Generation(), oldWS)
	require.Greater(t, next.connectionEpoch, m.connectionEpoch)
	require.NotNil(t, cmd)
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
	require.NotNil(t, cmd)
	next, cmd := resolvedTestTarget(t, updated.(Model))
	require.Greater(t, next.connectionEpoch, m.connectionEpoch)
	require.True(t, next.connectionChecked)
	require.False(t, next.connectionReady)
	require.Nil(t, cmd)
}

func TestSlowControllerDiscoveryLeavesInputResponsiveAndCannotWinLate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process discovery is used on Unix")
	}
	m, oldPath := targetTestModel(t)
	shim := t.TempDir()
	started, released := filepath.Join(shim, "started"), filepath.Join(shim, "release")
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MIHOSH_DISCOVERY_TEST_START", started)
	t.Setenv("MIHOSH_DISCOVERY_TEST_RELEASE", released)
	t.Setenv("MIHOSH_DISCOVERY_TEST_CONFIG", oldPath)
	t.Cleanup(func() { _ = os.WriteFile(released, nil, 0600) })
	require.NoError(t, os.WriteFile(filepath.Join(shim, "systemctl"), []byte("#!/bin/sh\nexit 1\n"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(shim, "ps"), []byte(`#!/bin/sh
: > "$MIHOSH_DISCOVERY_TEST_START"
while [ ! -f "$MIHOSH_DISCOVERY_TEST_RELEASE" ]; do sleep 0.01; done
printf '123 mihomo -f %s\n' "$MIHOSH_DISCOVERY_TEST_CONFIG"
`), 0755))
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.Mode = "external"; return nil }))

	updated, discover := m.Update(messages.ManagedCoreStartedMsg{Epoch: m.connectionEpoch})
	pending := updated.(Model)
	require.NotNil(t, pending.targetPending)
	older := make(chan tea.Msg, 1)
	go func() { older <- discover() }()
	require.Eventually(t, func() bool { _, err := os.Stat(started); return err == nil }, time.Second, 2*time.Millisecond)
	updated, _ = pending.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	pending = updated.(Model)
	require.True(t, pending.showHelp)
	require.Equal(t, "old", pending.client.EndpointSnapshot().Secret)

	require.NoError(t, config.Update(func(cfg *config.Config) error {
		cfg.Core.ExternalController = "127.0.0.1:9191"
		cfg.Core.ExternalSecret = "new"
		return nil
	}))
	updated, discover = pending.Update(messages.ManagedCoreStartedMsg{Epoch: pending.connectionEpoch})
	newer := discover()
	updated, _ = updated.Update(newer)
	current := updated.(Model)
	require.Equal(t, "new", current.client.EndpointSnapshot().Secret)
	require.Greater(t, current.wsClient.Generation(), uint64(0))
	require.NoError(t, os.WriteFile(released, nil, 0600))
	select {
	case result := <-older:
		updated, cmd := current.Update(result)
		require.Nil(t, cmd)
		require.Equal(t, "new", updated.(Model).client.EndpointSnapshot().Secret)
		require.Equal(t, current.connectionEpoch, updated.(Model).connectionEpoch)
	case <-time.After(2 * time.Second):
		t.Fatal("old discovery did not return after release")
	}
}
