package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	configpkg "github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderConfigShow(t *testing.T) {
	cfg := &configpkg.Config{
		APIAddress:          "http://127.0.0.1:9090",
		Secret:              "secret-token",
		TestURL:             "http://www.gstatic.com/generate_204",
		Timeout:             5000,
		ProxyAddress:        "http://127.0.0.1:7890",
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
				`"api_address": "http://127.0.0.1:9090"`,
				`"secret": "sec****ken"`,
				`"auto_refresh_interval": 5`,
				`"config_file":`,
				`config.yaml`,
			},
		},
		{
			name:   "Table format",
			format: outputFormatTable,
			contains: []string{
				"KEY",
				"VALUE",
				"API_ADDRESS",
				"http://127.0.0.1:9090",
				"AUTO_REFRESH_INTERVAL",
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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

	path := filepath.Join(tempDir, "large-config.yaml")

	// Create a file slightly larger than 1MB
	largeData := make([]byte, 1024*1024+1)
	err = os.WriteFile(path, largeData, 0644)
	assert.NoError(t, err)

	err = checkConfigFileSize(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "配置文件过大")

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
		assert.Contains(t, err.Error(), "未找到 mihomo 配置文件")
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
	assert.Contains(t, err.Error(), "config.yaml")
	assert.Contains(t, err.Error(), "config.yml")
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
