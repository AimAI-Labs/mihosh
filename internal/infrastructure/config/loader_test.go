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

// stubEmptyProcessList 隔离真实进程扫描，避免测试受本机运行中的 mihomo 影响。
func stubEmptyProcessList(t *testing.T) {
	t.Helper()
	originalRunner := processListRunner
	processListRunner = func() ([]byte, error) {
		return []byte{}, nil
	}
	t.Cleanup(func() {
		processListRunner = originalRunner
	})
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
	stubEmptyProcessList(t)

	path, err := GetMihomoConfigPath()
	require.Error(t, err)
	assert.Empty(t, path)
}

func TestGetMihomoConfigPathForWriteReturnsDefaultWhenNotExists(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", filepath.Join(tempHome, "AppData", "Roaming"))
	}

	originalRunner := systemctlStatusRunner
	systemctlStatusRunner = func() ([]byte, error) {
		return []byte("mihomo.service could not be found"), nil
	}
	t.Cleanup(func() {
		systemctlStatusRunner = originalRunner
	})
	stubEmptyProcessList(t)

	path, err := GetMihomoConfigPathForWrite()
	require.NoError(t, err)
	assert.NotEmpty(t, path)
	assert.Contains(t, path, "config.yaml")
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

// TestGetMihomoConfigPathFromProcessFindsManuallyStartedInstance 验证无 systemd 服务时，
// 通过进程命令行发现手动启动的 mihomo（-d 目录 + -f 自定义配置文件）。
func TestGetMihomoConfigPathFromProcessFindsManuallyStartedInstance(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "COCODUCK.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("mixed-port: 7890\n"), 0644))

	originalSystemctl := systemctlStatusRunner
	systemctlStatusRunner = func() ([]byte, error) {
		return []byte("mihomo.service could not be found"), nil
	}
	t.Cleanup(func() {
		systemctlStatusRunner = originalSystemctl
	})

	originalProcessList := processListRunner
	processListRunner = func() ([]byte, error) {
		return []byte(" 123 /usr/local/bin/mihomo -d " + dir + " -f " + configPath + "\n"), nil
	}
	t.Cleanup(func() {
		processListRunner = originalProcessList
	})

	path, err := GetMihomoConfigPathFromProcess()
	require.NoError(t, err)
	assert.Equal(t, configPath, path)
}

// TestParseMihomoConfigFromProcessListPrefersConfigFile 验证 -f 优先于 -d 目录下的默认文件。
func TestParseMihomoConfigFromProcessListPrefersConfigFile(t *testing.T) {
	dir := t.TempDir()
	defaultConfig := filepath.Join(dir, "config.yaml")
	customConfig := filepath.Join(dir, "COCODUCK.yaml")
	require.NoError(t, os.WriteFile(defaultConfig, []byte("mixed-port: 7890\n"), 0644))
	require.NoError(t, os.WriteFile(customConfig, []byte("mixed-port: 7891\n"), 0644))

	output := " 123 /usr/local/bin/mihomo -d " + dir + " -f " + customConfig + "\n"
	assert.Equal(t, customConfig, parseMihomoConfigFromProcessList(output))
}

// TestParseMihomoConfigFromProcessListUsesDirectory 验证只有 -d 时使用目录下的 config.yaml。
func TestParseMihomoConfigFromProcessListUsesDirectory(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("mixed-port: 7890\n"), 0644))

	output := " 123 /usr/local/bin/mihomo -d " + dir + "\n"
	assert.Equal(t, configPath, parseMihomoConfigFromProcessList(output))
}

// TestParseMihomoConfigFromProcessListSupportsEqualsSyntax 验证 -f=path 写法。
func TestParseMihomoConfigFromProcessListSupportsEqualsSyntax(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "custom.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("mixed-port: 7890\n"), 0644))

	output := " 123 /usr/local/bin/mihomo -f=" + configPath + "\n"
	assert.Equal(t, configPath, parseMihomoConfigFromProcessList(output))
}

// TestParseMihomoConfigFromProcessListIgnoresUnrelatedProcesses 防止误匹配编辑器、
// mihosh 自身或 grep 等包含 "mihomo" 字样的进程。
func TestParseMihomoConfigFromProcessListIgnoresUnrelatedProcesses(t *testing.T) {
	output := " 1 /usr/bin/vim /home/u/.config/mihomo/config.yaml\n" +
		" 2 /home/u/mihosh\n" +
		" 3 /usr/bin/grep mihomo\n" +
		" 4 /usr/bin/zsh -c ./mihomo -d /tmp"
	assert.Empty(t, parseMihomoConfigFromProcessList(output))
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
