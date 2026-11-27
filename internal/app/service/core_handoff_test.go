//go:build linux

package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/stretchr/testify/require"
)

type handoffServices struct {
	*fakeCoreServices
	owner     *CoreService
	failStart bool
	failStop  bool
}

func (m *handoffServices) Action(ctx context.Context, action string) error {
	if action == "stop" && m.failStop {
		return errors.New("unit not loaded")
	}
	if err := m.fakeCoreServices.Action(ctx, action); err != nil {
		return err
	}
	switch action {
	case "start", "restart":
		if m.failStart {
			return errors.New("background start rejected")
		}
		if err := m.owner.Start(ctx); err != nil {
			return err
		}
		m.state.Running = true
		return nil
	case "stop":
		m.state.Running = false
		return m.owner.StopContext(ctx)
	}
	return nil
}

func serviceHandoffFixture(t *testing.T) (*CoreService, *handoffServices, *api.Client) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("fake kernel requires Python")
	}
	s, network, root := networkService(t)
	require.NoError(t, config.Update(func(cfg *config.Config) error { cfg.Core.Version = "v1.2.3"; return nil }))
	require.NoError(t, s.Configure())
	freePort := func() int {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { require.NoError(t, listener.Close()) }()
		return listener.Addr().(*net.TCPAddr).Port
	}
	controller, mixed := freePort(), freePort()
	require.NoError(t, s.SetPorts(controller, mixed))
	binary, err := coreinfra.BinaryPath(root, "v1.2.3", "linux")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(binary), 0700))
	// Local fake API and proxy listeners exercise process ownership and readiness.
	script := fmt.Sprintf(`#!%s
import http.server, socket, sys
if '-t' in sys.argv: sys.exit(0)
proxy = socket.socket()
proxy.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
proxy.bind(('127.0.0.1', %d))
proxy.listen(128)
class API(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'{"version":"v1.2.3","proxies":{}}')
    def log_message(self, *args): pass
http.server.HTTPServer(('127.0.0.1', %d), API).serve_forever()
`, python, mixed, controller)
	require.NoError(t, os.WriteFile(binary, []byte(script), 0700))
	owner := NewCoreService()
	owner.daemon, owner.network = true, network
	t.Cleanup(owner.Close)
	manager := &handoffServices{fakeCoreServices: &fakeCoreServices{}, owner: owner}
	owner.services = manager.fakeCoreServices
	s.services = manager
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, s.Start(ctx))
	return s, manager, api.NewClient(config.ResolveMihomoEndpoint(), 500)
}

func TestServiceHandoffKeepsProxyAfterClientClose(t *testing.T) {
	for _, action := range []string{"install", "start", "restart", "enable", "boot"} {
		t.Run(action, func(t *testing.T) {
			s, manager, client := serviceHandoffFixture(t)
			if action != "install" {
				manager.state.Installed = true
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			require.NoError(t, s.ServiceAction(ctx, action))
			require.Nil(t, s.process)
			require.NotNil(t, manager.owner.process)
			cfg, err := config.Load()
			require.NoError(t, err)
			require.Equal(t, "service", cfg.Core.Runtime)
			if action == "enable" || action == "boot" {
				require.True(t, manager.state.Enabled)
			}
			s.Close()
			_, err = client.GetVersion()
			require.NoError(t, err, "closing the frontend must leave the background core running")
			endpoint := config.ResolveMihomoEndpoint()
			proxy, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", endpoint.MixedPort), time.Second)
			require.NoError(t, err)
			require.NoError(t, proxy.Close())
		})
	}
}

func TestServiceHandoffFailurePreservesSession(t *testing.T) {
	for _, failInstall := range []bool{false, true} {
		t.Run(fmt.Sprint(failInstall), func(t *testing.T) {
			s, manager, client := serviceHandoffFixture(t)
			original := s.process
			manager.fail, manager.failStart = failInstall, !failInstall
			err := s.ServiceAction(context.Background(), "install")
			require.Error(t, err)
			require.NotNil(t, s.process)
			if failInstall {
				require.Same(t, original, s.process, "failed installation must not stop the session")
			}
			_, err = client.GetVersion()
			require.NoError(t, err)
			cfg, err := config.Load()
			require.NoError(t, err)
			require.NotEqual(t, "service", cfg.Core.Runtime)
		})
	}
}

func TestServiceActionWithoutInstallPreservesRunningSessionAndConfig(t *testing.T) {
	s, _, client := serviceHandoffFixture(t)
	original := s.process
	for _, action := range []string{"start", "restart", "enable", "boot", "stop", "disable", "uninstall"} {
		t.Run(action, func(t *testing.T) {
			err := s.ServiceAction(context.Background(), action)
			require.ErrorContains(t, err, "not installed")
			require.Same(t, original, s.process, "uninstalled service action must never stop the running session")
			_, err = client.GetVersion()
			require.NoError(t, err)
			cfg, err := config.Load()
			require.NoError(t, err)
			require.NotEqual(t, "service", cfg.Core.Runtime)
		})
	}
}

func TestServiceHandoffRecoveryRestoresSessionEvenIfStopErrors(t *testing.T) {
	s, manager, client := serviceHandoffFixture(t)
	manager.state.Installed = true
	manager.failStart = true
	manager.failStop = true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := s.ServiceAction(ctx, "start")
	require.Error(t, err)
	require.NotNil(t, s.process, "failed handoff with inactive service must still recover session core")
	_, err = client.GetVersion()
	require.NoError(t, err)
	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotEqual(t, "service", cfg.Core.Runtime)
}

func TestServiceHandoffCanSwitchToExternal(t *testing.T) {
	for _, tc := range []struct {
		name, controller, secret string
		mixedPort                int
		apply                    func(*CoreService) error
		failStop                 bool
	}{
		{name: "default discovery", apply: (*CoreService).UseExternal},
		{
			name: "explicit controller", controller: "https://external.example:9090",
			secret: "fixture-secret", mixedPort: 7890,
			apply: func(s *CoreService) error {
				return s.ConnectExternal("https://external.example:9090/", "fixture-secret", 7890)
			},
		},
		{name: "stop failure", apply: (*CoreService).UseExternal, failStop: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, manager, _ := serviceHandoffFixture(t)
			manager.state.Installed = true
			require.NoError(t, s.ServiceAction(context.Background(), "start"))
			require.True(t, manager.state.Running)
			manager.failStop = tc.failStop

			err := tc.apply(s)
			cfg, loadErr := config.Load()
			require.NoError(t, loadErr)
			if tc.failStop {
				require.ErrorContains(t, err, "unit not loaded")
				require.Equal(t, "managed", cfg.Core.Mode)
				require.True(t, manager.state.Running)
				return
			}
			require.NoError(t, err)
			require.False(t, manager.state.Running)
			require.Nil(t, manager.owner.process)
			require.Equal(t, "external", cfg.Core.Mode)
			require.Equal(t, tc.controller, cfg.Core.ExternalController)
			require.Equal(t, tc.secret, cfg.Core.ExternalSecret)
			require.Equal(t, tc.mixedPort, cfg.Core.ExternalMixedPort)
		})
	}
}
