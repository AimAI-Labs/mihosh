package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/stretchr/testify/require"
)

func TestControllerCommandsWorkBeforeCreatingMihoshConfig(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", home)
	t.Setenv("PATH", t.TempDir()) // Use only the isolated Mihomo file during discovery.

	var reloads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/configs" && r.Method == http.MethodPut:
			reloads.Add(1)
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/configs":
			_, _ = io.WriteString(w, `{"mode":"rule"}`)
		case r.URL.Path == "/connections":
			_, _ = io.WriteString(w, `{"connections":[]}`)
		case r.URL.Path == "/memory":
			_, _ = io.WriteString(w, `{"inuse":42}`)
		case r.URL.Path == "/proxies":
			_, _ = io.WriteString(w, `{"proxies":{"group":{"name":"group","type":"Selector","all":["proxy"],"now":"proxy"}}}`)
		case r.URL.Path == "/proxies/group" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/delay"):
			_, _ = io.WriteString(w, `{"delay":42}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	path, err := config.GetDefaultMihomoConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("external-controller: "+server.URL+"\n"), 0600))
	output, err := os.CreateTemp(t.TempDir(), "output")
	require.NoError(t, err)
	original := os.Stdout
	os.Stdout = output
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		os.Stdout = original
		_ = output.Close()
	})

	for _, args := range [][]string{
		{"status"}, {"list"}, {"select", "group", "proxy"}, {"mode"}, {"connections"},
		{"test", "node", "proxy"}, {"test", "group", "group"}, {"test-group", "group"}, {"config", "show"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			rootCmd.SetArgs(args)
			var stderr bytes.Buffer
			require.Equal(t, exitCodeOK, executeRootCommand(rootCmd, &stderr), stderr.String())
		})
	}
	require.NoError(t, reloadMihomoCore(path))
	require.EqualValues(t, 1, reloads.Load())

	cfgPath := filepath.Join(root, "config.yaml")
	_, err = os.Stat(cfgPath)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, os.WriteFile(cfgPath, []byte("language: ["), 0600))
	rootCmd.SetArgs([]string{"status"})
	var stderr bytes.Buffer
	require.Equal(t, exitCodeConfig, executeRootCommand(rootCmd, &stderr), stderr.String())
}
