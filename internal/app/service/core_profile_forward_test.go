//go:build linux

package service

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/host"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

func TestForwardedProfileKeepsActivationResult(t *testing.T) {
	for _, warning := range []bool{false, true} {
		name := "without-warning"
		if warning {
			name = "with-warning"
		}
		t.Run(name, func(t *testing.T) {
			owner, uid, _ := prepareManagedProfile(t)
			if warning {
				merge, err := profile.MergePath()
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(merge, []byte("rules: [\n"), 0600))
			}
			local, err := owner.ApplyProfile(context.Background(), uid, false)
			require.NoError(t, err)
			root, err := config.GetConfigDir()
			require.NoError(t, err)
			owner.daemon = true
			require.NoError(t, owner.startControl(root))
			caller := NewCoreService()
			t.Cleanup(caller.Close)
			result, err := caller.ApplyProfile(context.Background(), uid, false)
			require.NoError(t, err)
			require.NotEmpty(t, result.BackupName)
			require.FileExists(t, filepath.Join(root, "config-backups", result.BackupName))
			if warning {
				require.Error(t, result.MergeErr)
				require.Equal(t, local.MergeErr.Error(), result.MergeErr.Error())
			} else {
				require.NoError(t, result.MergeErr)
			}
		})
	}
}

type ownerStartingServices struct {
	*fakeCoreServices
	owner *CoreService
	root  string
}

func (m *ownerStartingServices) Action(ctx context.Context, action string) error {
	if err := m.fakeCoreServices.Action(ctx, action); err != nil {
		return err
	}
	if action == "start" {
		m.owner.mu.Lock()
		defer m.owner.mu.Unlock()
		return m.owner.startControl(m.root)
	}
	return nil
}

func TestForwardedProfileAfterServiceStartKeepsWarning(t *testing.T) {
	owner, uid, _ := prepareManagedProfile(t)
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			w.Write([]byte(`{"version":"v1.2.3"}`))
		} else {
			w.Write([]byte(`{"proxies":{}}`))
		}
	}))
	defer controller.Close()
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer proxy.Close()
	path, err := config.ManagedMihomoConfigPath()
	require.NoError(t, err)
	require.NoError(t, config.WriteMihomoField(path, "external-controller", strings.TrimPrefix(controller.URL, "http://")))
	require.NoError(t, config.WriteMihomoField(path, "mixed-port", proxy.Addr().(*net.TCPAddr).Port))
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.Runtime = "service"; return nil }))
	merge, err := profile.MergePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(merge, []byte("rules: [\n"), 0600))
	owner.daemon = true
	manager := &ownerStartingServices{fakeCoreServices: &fakeCoreServices{state: host.ServiceState{Installed: true}}, owner: owner, root: root}
	owner.services = manager.fakeCoreServices
	caller := NewCoreService()
	t.Cleanup(caller.Close)
	caller.services = manager
	result, err := caller.ApplyProfile(context.Background(), uid, true)
	require.NoError(t, err)
	require.Equal(t, []string{"start"}, manager.actions)
	require.NotEmpty(t, result.BackupName)
	require.Error(t, result.MergeErr)
}

func TestForwardedProfileResponseCompatibility(t *testing.T) {
	for _, entry := range []struct {
		name, body, wantError string
		status                int
	}{
		{"legacy", `{"result":"ok"}`, "", http.StatusOK},
		{"malformed", `{"result":`, "invalid local core profile response", http.StatusOK},
		{"unexpected", `{"result":"unexpected"}`, "invalid local core profile response", http.StatusOK},
		{"failure", `{"error":"activation failed"}`, "activation failed", http.StatusConflict},
	} {
		t.Run(entry.name, func(t *testing.T) {
			newIsolatedService(t)
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(entry.status)
				w.Write([]byte(entry.body))
			}))
			defer server.Close()
			root, err := config.GetConfigDir()
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(filepath.Join(root, "runtime"), 0700))
			data, err := json.Marshal(coreOwner{URL: server.URL, Token: strings.Repeat("a", 64)})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(root, "runtime", "owner.json"), data, 0600))
			caller := NewCoreService()
			t.Cleanup(caller.Close)
			result, err := caller.ApplyProfile(context.Background(), "uid", false)
			if entry.wantError == "" {
				require.NoError(t, err)
				require.Empty(t, result.BackupName)
				require.NoError(t, result.MergeErr)
			} else {
				require.ErrorContains(t, err, entry.wantError)
			}
			require.Equal(t, int32(1), requests.Load(), "owner responses must not trigger local reapplication")
		})
	}
}
