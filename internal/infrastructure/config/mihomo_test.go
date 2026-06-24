package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadWriteMihomoYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Test 1: Write to non-existent file
	err := WriteMihomoField(configPath, "allow-lan", true)
	if err != nil {
		t.Fatalf("Failed to write to new file: %v", err)
	}
	res, err := ReadMihomoYAML(configPath)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	if res["allow-lan"] != true {
		t.Errorf("Expected allow-lan=true, got %v", res["allow-lan"])
	}

	// Test 2: Write different type
	err = WriteMihomoField(configPath, "mixed-port", 7890)
	if err != nil {
		t.Fatalf("Failed to write int: %v", err)
	}

	// Test 3: Write float type (generic type test)
	err = WriteMihomoField(configPath, "test-float", 1.5)
	if err != nil {
		t.Fatalf("Failed to write float: %v", err)
	}

	// Test 4: Preserve comments
	initialYAML := []byte("# Comment\nexternal-controller: '127.0.0.1:9090'\nallow-lan: false\n")
	if err := os.WriteFile(configPath, initialYAML, 0644); err != nil {
		t.Fatalf("Failed to setup comment test: %v", err)
	}
	if err := WriteMihomoField(configPath, "allow-lan", true); err != nil {
		t.Fatalf("Failed to write existing field: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read back: %v", err)
	}
	if !strings.Contains(string(data), "# Comment") {
		t.Errorf("Comments were not preserved")
	}
	if !strings.Contains(string(data), "allow-lan: true") {
		t.Errorf("Value was not updated")
	}
}

// TestResolveMixedPortFromYAMLTypes 验证 resolveMixedPort 兼容 YAML 解析的多种数值类型。
func TestResolveMixedPortFromYAMLTypes(t *testing.T) {
	cases := []struct {
		name string
		data map[string]interface{}
		want int
	}{
		{name: "int", data: map[string]interface{}{"mixed-port": 7890}, want: 7890},
		{name: "int64", data: map[string]interface{}{"mixed-port": int64(7891)}, want: 7891},
		{name: "float64", data: map[string]interface{}{"mixed-port": float64(7892)}, want: 7892},
		{name: "missing falls back", data: map[string]interface{}{}, want: 7890},
		{name: "zero falls back", data: map[string]interface{}{"mixed-port": 0}, want: 7890},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveMixedPort(c.data, 7890); got != c.want {
				t.Errorf("resolveMixedPort() = %d, want %d", got, c.want)
			}
		})
	}
}

// TestMixedPortToProxyURL 验证代理地址派生格式。
func TestMixedPortToProxyURL(t *testing.T) {
	if got := MixedPortToProxyURL(7890); got != "http://127.0.0.1:7890" {
		t.Errorf("MixedPortToProxyURL(7890) = %q, want http://127.0.0.1:7890", got)
	}
}

// TestResolveMihomoEndpointParsesYAML 验证从临时 YAML 文件解析 endpoint 各字段。
// 通过临时设置 HOME + 注入已知 mihomo 目录，让 GetMihomoConfigPath 命中该文件。
func TestResolveMihomoEndpointParsesYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", "")

	mihomoDir := filepath.Join(home, ".config", "mihomo")
	require.NoError(t, os.MkdirAll(mihomoDir, 0755))
	yamlContent := []byte(strings.Join([]string{
		"external-controller: 192.168.1.10:9091",
		"secret: topsecret",
		"mixed-port: 7897",
		"allow-lan: true",
		"log-level: info",
	}, "\n"))
	configPath := filepath.Join(mihomoDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, yamlContent, 0644))

	got := ResolveMihomoEndpoint()
	assert.Equal(t, "192.168.1.10:9091", got.ExternalController)
	assert.Equal(t, "topsecret", got.Secret)
	assert.Equal(t, 7897, got.MixedPort)
}

// TestResolveMihomoEndpointFallsBackToDefaults 当自动发现失败时回退默认值。
func TestResolveMihomoEndpointFallsBackToDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("APPDATA", "")

	originalRunner := systemctlStatusRunner
	systemctlStatusRunner = func() ([]byte, error) {
		return []byte("mihomo.service could not be found"), nil
	}
	t.Cleanup(func() { systemctlStatusRunner = originalRunner })

	got := ResolveMihomoEndpoint()
	assert.Equal(t, DefaultMihomoEndpoint, got, "auto discovery failure should fall back to defaults")
	assert.NotEmpty(t, got.ExternalController, "default endpoint must be usable")
	assert.Greater(t, got.MixedPort, 0, "default mixed-port must be valid")
}

// TestResolveMihomoEndpointPartialFields 当部分字段缺失时保留默认值。
func TestResolveMihomoEndpointPartialFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", "")

	mihomoDir := filepath.Join(home, ".config", "mihomo")
	require.NoError(t, os.MkdirAll(mihomoDir, 0755))
	// 仅写 external-controller，secret/mixed-port 应保留默认
	configPath := filepath.Join(mihomoDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("external-controller: 10.0.0.1:9090\n"), 0644))

	got := ResolveMihomoEndpoint()
	assert.Equal(t, "10.0.0.1:9090", got.ExternalController)
	assert.Equal(t, "", got.Secret, "missing secret should be empty (default)")
	assert.Equal(t, DefaultMihomoEndpoint.MixedPort, got.MixedPort, "missing mixed-port should fall back to default")
}
