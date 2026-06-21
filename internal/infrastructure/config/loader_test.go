package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSystemctlStatusParsesCGroupProcessLine(t *testing.T) {
	output := `● mihomo.service - mihomo Daemon, Another Clash Kernel.
     Loaded: loaded (/etc/systemd/system/mihomo.service; enabled; preset: enabled)
     Active: active (running) since Sun 2026-02-08 12:33:48 CST; 1 month 23 days ago
   Main PID: 902928 (mihomo)
      Tasks: 11 (limit: 4291)
     Memory: 51.3M (peak: 100.0M swap: 4.1M swap peak: 10.6M)
        CPU: 24min 16.624s
     CGroup: /system.slice/mihomo.service
             └─902928 /home/ubuntu/Apps/local/mihomo/mihomo -d /home/ubuntu/Apps/local/mihomo
`

	assert.Equal(
		t,
		"/home/ubuntu/Apps/local/mihomo/mihomo -d /home/ubuntu/Apps/local/mihomo",
		parseSystemctlStatus(output),
	)
}

func TestGetMihomoConfigPathReturnsFallbackHintWhenAutoDiscoveryFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("APPDATA", "")

	originalRunner := systemctlStatusRunner
	systemctlStatusRunner = func() ([]byte, error) {
		return []byte("mihomo.service could not be found"), nil
	}
	t.Cleanup(func() {
		systemctlStatusRunner = originalRunner
	})

	path, err := GetMihomoConfigPath()
	require.Error(t, err)
	assert.Empty(t, path)
	assert.Contains(t, err.Error(), "sudo systemctl status mihomo")
	assert.Contains(t, err.Error(), "-d")
	assert.Contains(t, err.Error(), "config.yaml")
	assert.Contains(t, err.Error(), "config.yml")
}

func TestGetMihomoConfigPathFromProcessFindsConfigInSystemctlDirectory(t *testing.T) {
	homeDir := t.TempDir()
	serviceDir := t.TempDir()
	configPath := serviceDir + string(os.PathSeparator) + "config.yaml"

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("APPDATA", "")

	err := os.WriteFile(configPath, []byte("mixed-port: 7890\n"), 0644)
	require.NoError(t, err)

	originalRunner := systemctlStatusRunner
	systemctlStatusRunner = func() ([]byte, error) {
		return []byte("             └─902928 /home/ubuntu/Apps/local/mihomo/mihomo -d " + serviceDir), nil
	}
	t.Cleanup(func() {
		systemctlStatusRunner = originalRunner
	})

	path, err := GetMihomoConfigPathFromProcess()
	require.NoError(t, err)
	assert.Equal(t, configPath, path)
}

// TestSearchKnownMihomoDirectoriesFindsClashVerge 验证 Clash Verge Rev（基于 Tauri）
// 的配置目录能被发现。verge-mihomo 启动时以该目录为 -d，其下的 config.yaml 才是
// 热重载真正读取的文件——若此处发现失败，订阅 Activate 写入会落到错误文件上、
// 导致「切换订阅后代理不生效」。
//
// 仅在对应平台运行：Windows 走 %APPDATA%，类 Unix 走 ~/.config，macOS 走 ~/Library。
func TestSearchKnownMihomoDirectoriesFindsClashVerge(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	var appData string
	if runtime.GOOS == "windows" {
		appData = t.TempDir()
		t.Setenv("APPDATA", appData)
	} else {
		t.Setenv("APPDATA", "")
	}

	// 选定一个 Clash Verge identifier，按当前平台拼出其配置根目录。
	const identifier = "io.github.clash-verge-rev.clash-verge-rev"
	var vergeDir string
	switch runtime.GOOS {
	case "windows":
		vergeDir = filepath.Join(appData, identifier)
	case "darwin":
		vergeDir = filepath.Join(home, "Library", "Application Support", identifier)
	default:
		vergeDir = filepath.Join(home, ".config", identifier)
	}
	require.NoError(t, os.MkdirAll(vergeDir, 0755))
	configPath := filepath.Join(vergeDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("mixed-port: 7890\n"), 0644))

	got := searchKnownMihomoDirectories()
	assert.Equal(t, configPath, got, "应发现 Clash Verge 的 config.yaml")
}

// TestSearchKnownMihomoDirectoriesFindsLegacyClashVerge 覆盖历史标识符
// io.github.zzzgydi.clash-verge（上游原版 Clash Verge），与 Rev fork 并列搜索。
func TestSearchKnownMihomoDirectoriesFindsLegacyClashVerge(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	var appData string
	if runtime.GOOS == "windows" {
		appData = t.TempDir()
		t.Setenv("APPDATA", appData)
	} else {
		t.Setenv("APPDATA", "")
	}

	const identifier = "io.github.zzzgydi.clash-verge"
	var vergeDir string
	switch runtime.GOOS {
	case "windows":
		vergeDir = filepath.Join(appData, identifier)
	case "darwin":
		vergeDir = filepath.Join(home, "Library", "Application Support", identifier)
	default:
		vergeDir = filepath.Join(home, ".config", identifier)
	}
	require.NoError(t, os.MkdirAll(vergeDir, 0755))
	configPath := filepath.Join(vergeDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("mixed-port: 7890\n"), 0644))

	got := searchKnownMihomoDirectories()
	assert.Equal(t, configPath, got, "应发现上游 Clash Verge 的 config.yaml")
}
