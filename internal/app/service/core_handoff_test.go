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
}

func (m *handoffServices) Action(ctx context.Context, action string) error {
	if err := m.fakeCoreServices.Action(ctx, action); err != nil {
		return err
	}
	switch action {
	case "start", "restart":
		if m.failStart {
			return errors.New("background start rejected")
		}
		return m.owner.Start(ctx)
	case "stop":
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
		defer listener.Close()
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
			proxy.Close()
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
