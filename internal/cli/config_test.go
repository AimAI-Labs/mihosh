package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	configpkg "github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIConfigReloadKeepsSuccessAndWarningOutput(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != http.MethodPut || r.URL.Path != "/configs" {
					t.Errorf("unexpected reload request: %s %s", r.Method, r.URL)
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			cfg := configpkg.DefaultConfig
			cfg.Core.Mode, cfg.Core.ExternalController = "external", server.URL
			require.NoError(t, configpkg.Save(&cfg))
			out, err := os.CreateTemp(t.TempDir(), "reload-output")
			require.NoError(t, err)
			saved := os.Stdout
			os.Stdout = out
			t.Cleanup(func() { os.Stdout = saved; _ = out.Close() })
			require.NoError(t, reloadMihomoCore("already-edited.yaml"), "reload failure remains a warning after editing")
			require.EqualValues(t, 1, requests.Load())
			_, err = out.Seek(0, io.SeekStart)
			require.NoError(t, err)
			output, err := io.ReadAll(out)
			require.NoError(t, err)
			if status == http.StatusNoContent {
				require.Contains(t, string(output), i18n.T("cli.config.edit.reload_success"))
			} else {
				require.Contains(t, string(output), i18n.T("cli.config.edit.reload_fail_hint"))
			}
		})
	}
}

// isolateHostMihomoDiscovery 用空的 ps/systemctl 屏蔽宿主机环境，
// 避免自动发现命中开发机上真实运行的 mihomo（Windows 不走进程发现）。
func isolateHostMihomoDiscovery(t *testing.T) {
	t.Helper()
	binDir := t.TempDir()
	for _, name := range []string{"ps", "systemctl", "sudo"} {
		require.NoError(t, os.WriteFile(filepath.Join(binDir, name), []byte("#!/bin/sh\nexit 1\n"), 0755))
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRenderConfigShow(t *testing.T) {
	// 将 mihomo 配置自动发现隔离到临时目录，避免读取真实环境。
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("APPDATA", "")
	isolateHostMihomoDiscovery(t)

	cfg := &configpkg.Config{
		TestURL:             "http://www.gstatic.com/generate_204",
		Timeout:             5000,
		AutoRefreshInterval: 5,
	}

	tests := []struct {
		name     string
		format   outputFormat
		contains []string
	}{
		{
			name:   "JSON format",
			format: outputFormatJSON,
			contains: []string{
				`"test_url": "http://www.gstatic.com/generate_204"`,
				`"auto_refresh_interval": 5`,
				`"config_file":`,
				`config.yaml`,
				`"mihomo"`,
				`"external_controller": "127.0.0.1:9090"`,
				`"proxy_url": "http://127.0.0.1:7890"`,
			},
		},
		{
			name:   "Table format",
			format: outputFormatTable,
			contains: []string{
				"KEY",
				"VALUE",
				"TEST_URL",
				"AUTO_REFRESH_INTERVAL",
				"EXTERNAL_CONTROLLER",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := renderConfigShow(&out, cfg, "C:\\mihosh\\config.yaml", tt.format)
			assert.NoError(t, err)

			output := out.String()
			for _, c := range tt.contains {
				assert.Contains(t, output, c)
			}
		})
	}
}

func TestValidateConfigFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mihosh-test-*")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	tests := []struct {
		name     string
		content  string
		ext      string
		expected bool
	}{
		{
			name:     "Valid YAML",
			content:  "api_address: http://localhost:9090\nsecret: token",
			ext:      ".yaml",
			expected: true,
		},
		{
			name:     "Invalid YAML",
			content:  "api_address: : invalid",
			ext:      ".yaml",
			expected: false,
		},
		{
			name:     "Valid JSON",
			content:  `{"api_address": "http://localhost:9090"}`,
			ext:      ".json",
			expected: true,
		},
		{
			name:     "Invalid JSON",
			content:  `{"api_address": "http://localhost:9090"`,
			ext:      ".json",
			expected: false,
		},
		{
			name:     "Valid TOML",
			content:  `api_address = "http://localhost:9090"`,
			ext:      ".toml",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tempDir, "config"+tt.ext)
			err := os.WriteFile(path, []byte(tt.content), 0644)
			assert.NoError(t, err)

			err = validateConfigFile(path)
			if tt.expected {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestDetectEditor(t *testing.T) {
	// This test depends on the environment, so we just check if it returns something or empty
	// We can't easily mock LookPath here without refactoring the code to use an interface or a function variable
	editor := detectEditor()
	// Just ensure it doesn't crash
	t.Logf("Detected editor: %s", editor)
}

func TestCheckConfigFileSize(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mihosh-test-*")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	path := filepath.Join(tempDir, "large-config.yaml")

	// Create a file slightly larger than 1MB
	largeData := make([]byte, 1024*1024+1)
	err = os.WriteFile(path, largeData, 0644)
	assert.NoError(t, err)

	err = checkConfigFileSize(path)
	assert.Error(t, err)

	// Create a small file
	smallPath := filepath.Join(tempDir, "small-config.yaml")
	err = os.WriteFile(smallPath, []byte("api_address: http://localhost:9090"), 0644)
	assert.NoError(t, err)

	err = checkConfigFileSize(smallPath)
	assert.NoError(t, err)
}

func TestGetMihomoConfigPath(t *testing.T) {
	// This test is tricky because it depends on user home and OS
	// But we can verify it returns an error if nothing is found
	// or we can mock parts of it.
	path, err := configpkg.GetMihomoConfigPath()
	if err != nil {
		assert.Error(t, err)
	} else {
		assert.NotEmpty(t, path)
	}
}

func TestResolveMihomoConfigTargetAcceptsDirectory(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")
	err := os.WriteFile(configPath, []byte("mixed-port: 7890\n"), 0644)
	assert.NoError(t, err)

	resolved, err := resolveMihomoConfigTarget(tempDir)
	assert.NoError(t, err)
	assert.Equal(t, configPath, resolved)
}

func TestResolveMihomoConfigTargetRejectsDirectoryWithoutDefaultConfig(t *testing.T) {
	tempDir := t.TempDir()

	resolved, err := resolveMihomoConfigTarget(tempDir)
	assert.Error(t, err)
	assert.Empty(t, resolved)
}

func TestEditConfigFileWithEditorUsesExternalEditorAndValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte("mixed-port: 7890\n"), 0644)
	require.NoError(t, err)

	var gotEditor string
	var gotPath string

	originalRunner := runEditorFn
	runEditorFn = func(editor, target string) error {
		gotEditor = editor
		gotPath = target
		return os.WriteFile(target, []byte("mixed-port: 7891\nmode: rule\n"), 0644)
	}
	t.Cleanup(func() {
		runEditorFn = originalRunner
	})

	err = editConfigFileWithEditor(path, "code")
	require.NoError(t, err)
	assert.Equal(t, "code", gotEditor)
	assert.Equal(t, path, gotPath)
}

func TestRunMihomoConfigEditUsesExternalEditorFlow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte("mixed-port: 7890\n"), 0644)
	require.NoError(t, err)

	originalPath := configEditPath
	originalEditor := configEditEditor
	originalRunner := runEditorFn
	configEditPath = path
	configEditEditor = "code"

	called := false
	runEditorFn = func(editor, target string) error {
		called = true
		return nil
	}

	t.Cleanup(func() {
		configEditPath = originalPath
		configEditEditor = originalEditor
		runEditorFn = originalRunner
	})

	err = runMihomoConfigEdit()
	require.NoError(t, err)
	assert.True(t, called)
}

func TestConfigEditRunsEditorWithQuotedPathAndArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture requires Unix")
	}
	dir := t.TempDir()
	editor := filepath.Join(dir, "sample editor")
	script := "#!/bin/sh\n[ \"$1\" = '--wait' ] || exit 1\nprintf 'mode: rule\\n' >> \"$2\"\n"
	require.NoError(t, os.WriteFile(editor, []byte(script), 0700))
	path := filepath.Join(dir, "edited config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("mixed-port: 7890\n"), 0600))

	require.NoError(t, editConfigFileWithEditor(path, "\""+editor+"\" --wait"))
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(content), "mode: rule")
}
