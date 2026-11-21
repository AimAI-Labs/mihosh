package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

const testChainConfigYAML = `mixed-port: 7890
external-controller: 127.0.0.1:9090
proxies:
  - name: Entry-HK
    type: ss
    server: 1.1.1.1
    port: 443
  - name: Exit-US
    type: trojan
    server: 2.2.2.2
    port: 443
  - name: Untouched-JP
    type: ss
    server: 3.3.3.3
    port: 443
    dialer-proxy: Existing-Relay
proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - Entry-HK
      - Exit-US
rules:
  - MATCH,PROXY
`

func TestApplyProxyChainLocalExternalReloadsAndSelectsExitNode(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", cfgDir)

	mihomoCfgPath := filepath.Join(cfgDir, "external-config.yaml")
	require.NoError(t, os.WriteFile(mihomoCfgPath, []byte(testChainConfigYAML), 0600))

	var mu sync.Mutex
	var reloadCount int
	var selectedGroup, selectedProxy string
	var failReload bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/configs":
			if failReload {
				http.Error(w, "bad config", http.StatusBadRequest)
				return
			}
			reloadCount++
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/proxies/"):
			selectedGroup = strings.TrimPrefix(r.URL.Path, "/proxies/")
			var body struct {
				Name string `json:"name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			selectedProxy = body.Name
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := api.NewClient(config.MihomoEndpoint{
		ExternalController: strings.TrimPrefix(srv.URL, "http://"),
	}, 2000)

	svc := NewConfigService()
	svc.localExternalPath = func() (string, error) { return mihomoCfgPath, nil }

	// 1. Apply chain Entry-HK -> Exit-US on PROXY.
	res := svc.ApplyProxyChain(context.Background(), nil, client, profile.ProxyChain{
		Group:   "PROXY",
		Nodes:   []string{"Entry-HK", "Exit-US"},
		Enabled: true,
	})
	require.NoError(t, res.Err)
	require.True(t, res.WriteOK)
	require.True(t, res.Applied)

	mu.Lock()
	require.Equal(t, 1, reloadCount)
	require.Equal(t, "PROXY", selectedGroup)
	require.Equal(t, "Exit-US", selectedProxy)
	mu.Unlock()

	written, err := os.ReadFile(mihomoCfgPath)
	require.NoError(t, err)
	require.Contains(t, string(written), "dialer-proxy: Entry-HK")
	require.Contains(t, string(written), "dialer-proxy: Existing-Relay")

	savedChains, err := svc.ReadProxyChains()
	require.NoError(t, err)
	chain, ok := savedChains.GroupChain("PROXY")
	require.True(t, ok)
	require.Equal(t, []string{"Entry-HK", "Exit-US"}, chain.Nodes)

	// 2. Rollback when reload fails: previous config & chain-edits.yaml must remain intact.
	mu.Lock()
	failReload = true
	mu.Unlock()

	failRes := svc.ApplyProxyChain(context.Background(), nil, client, profile.ProxyChain{
		Group:   "PROXY",
		Nodes:   nil,
		Enabled: false,
	})
	require.Error(t, failRes.Err)
	require.True(t, failRes.Restored)

	afterRollback, err := os.ReadFile(mihomoCfgPath)
	require.NoError(t, err)
	require.Contains(t, string(afterRollback), "dialer-proxy: Entry-HK")
	savedChains, err = svc.ReadProxyChains()
	require.NoError(t, err)
	_, ok = savedChains.GroupChain("PROXY")
	require.True(t, ok)

	// 3. Clearing the chain succeeds when reload succeeds, preserving unrelated dialer-proxy.
	mu.Lock()
	failReload = false
	mu.Unlock()

	clearRes := svc.ApplyProxyChain(context.Background(), nil, client, profile.ProxyChain{
		Group:   "PROXY",
		Nodes:   nil,
		Enabled: false,
	})
	require.NoError(t, clearRes.Err)
	require.True(t, clearRes.Applied)

	afterClear, err := os.ReadFile(mihomoCfgPath)
	require.NoError(t, err)
	require.NotContains(t, string(afterClear), "dialer-proxy: Entry-HK")
	require.Contains(t, string(afterClear), "dialer-proxy: Existing-Relay")
	savedChains, err = svc.ReadProxyChains()
	require.NoError(t, err)
	_, ok = savedChains.GroupChain("PROXY")
	require.False(t, ok)
}
