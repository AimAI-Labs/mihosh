//go:build linux

package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/host"
	"github.com/stretchr/testify/require"
)

type fakeCoreServices struct {
	state      host.ServiceState
	actions    []string
	fail       bool
	executable string
}

func (m *fakeCoreServices) Install(_ context.Context, executable string) error {
	m.executable = executable
	if m.fail {
		return errors.New("install rejected")
	}
	m.state.Installed = true
	return nil
}
func (m *fakeCoreServices) Action(_ context.Context, action string) error {
	m.actions = append(m.actions, action)
	if m.fail {
		return errors.New("action rejected")
	}
	switch action {
	case "enable", "boot":
		m.state.Enabled = true
	case "disable":
		m.state.Enabled = false
	case "uninstall":
		m.state = host.ServiceState{}
	}
	return nil
}
func (m *fakeCoreServices) Status(context.Context) (host.ServiceState, error) { return m.state, nil }
func TestServiceInstallUsesStableApplicationAndUninstallRestoresSessionMode(t *testing.T) {
	s, _, root := networkService(t)
	m := &fakeCoreServices{}
	s.services = m
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.Mode = "managed"; return nil }))
	require.NoError(t, s.ServiceAction(context.Background(), "install"))
	require.Equal(t, filepath.Join(root, "bin"), filepath.Dir(m.executable))
	_, err := os.Stat(m.executable)
	require.NoError(t, err)
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "service", cfg.Core.Runtime)
	require.NoError(t, s.ServiceAction(context.Background(), "enable"))
	status, err := s.Status()
	require.NoError(t, err)
	require.True(t, status.ServiceInstalled)
	require.True(t, status.AutoStart)
	require.NoError(t, s.ServiceAction(context.Background(), "uninstall"))
	cfg, err = config.Load()
	require.NoError(t, err)
	require.Equal(t, "session", cfg.Core.Runtime)
}
func TestRejectedServiceInstallDoesNotSelectServiceRuntime(t *testing.T) {
	s, _, _ := networkService(t)
	s.services = &fakeCoreServices{fail: true}
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.Mode = "managed"; return nil }))
	require.Error(t, s.ServiceAction(context.Background(), "install"))
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.Core.Runtime)
}
func TestClientCloseLeavesExistingBackgroundOwnerAlone(t *testing.T) {
	s, _, root := networkService(t)
	m := &fakeCoreServices{state: host.ServiceState{Installed: true, Running: true}}
	s.services = m
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.Mode = "managed"; cfg.Core.Runtime = "service"; return nil }))
	path := filepath.Join(root, "runtime", "owner.json")
	require.NoError(t, os.WriteFile(path, []byte("existing owner"), 0600))
	s.Close()
	require.Empty(t, m.actions)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "existing owner", string(data))
}
