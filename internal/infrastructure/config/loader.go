package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/spf13/viper"
)

var mu sync.Mutex

type errConfigNotFound struct{}

func (errConfigNotFound) Error() string { return i18n.T("config.loader.err_not_found") }

// ErrConfigNotFound 配置文件不存在
var ErrConfigNotFound = errConfigNotFound{}

var systemctlStatusRunner = func() ([]byte, error) {
	return exec.Command("systemctl", "status", "mihomo").CombinedOutput()
}

// GetConfigDir 获取配置目录
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, ".mihosh")
	return configDir, nil
}

// GetMihomoConfigPath 获取 mihomo 配置文件的默认路径。
// Linux 下优先从 systemctl status 解析 -d 参数（运行时实际生效的配置），
// 再降级搜索已知目录；其他平台直接搜索已知目录。
func GetMihomoConfigPath() (string, error) {
	// 优先：从运行中的 mihomo 进程解析 -d 参数（最权威的实际配置路径）
	if runtime.GOOS != "windows" {
		if configFile, err := GetMihomoConfigPathFromProcess(); err == nil {
			return configFile, nil
		}
	}

	// 降级：搜索已知目录（Clash Verge 等桌面客户端场景）
	if configFile := searchKnownMihomoDirectories(); configFile != "" {
		return configFile, nil
	}

	return "", buildMihomoConfigPathNotFoundError()
}

// GetMihomoConfigPathForWrite 获取用于写入的 mihomo 配置文件路径。
// 优先返回自动发现到的现有配置文件路径；若未发现，则返回默认创建路径（如 Windows 下 %APPDATA%\mihomo\config.yaml 或类 Unix 下 ~/.config/mihomo/config.yaml）。
func GetMihomoConfigPathForWrite() (string, error) {
	if configFile, err := GetMihomoConfigPath(); err == nil {
		return configFile, nil
	}
	return GetDefaultMihomoConfigPath()
}

// GetDefaultMihomoConfigPath 获取默认的 mihomo 配置文件路径
func GetDefaultMihomoConfigPath() (string, error) {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "mihomo", "config.yaml"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "mihomo", "config.yaml"), nil
}

func GetMihomoConfigPathFromProcess() (string, error) {
	output, err := systemctlStatusRunner()
	if err != nil {
		return "", fmt.Errorf(i18n.T("config.loader.err_systemctl_status"), err)
	}

	cmdLine := parseSystemctlStatus(string(output))
	if cmdLine == "" {
		return "", errors.New(i18n.T("config.loader.err_parse_cmd"))
	}

	configDir := extractConfigDirFromCommandLine(cmdLine)
	if configDir == "" {
		return "", errors.New(i18n.T("config.loader.err_extract_dir"))
	}

	if configFile := findConfigFileInDirectory(configDir); configFile != "" {
		return configFile, nil
	}

	if configFile := searchFallbackDirectories(); configFile != "" {
		return configFile, nil
	}

	return "", buildMihomoConfigPathNotFoundError()
}

func parseSystemctlStatus(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "ExecStart=") {
			parts := strings.SplitN(trimmed, "ExecStart=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}

		if !strings.Contains(trimmed, "/mihomo") || !strings.Contains(trimmed, "-d ") {
			continue
		}

		for _, field := range strings.Fields(trimmed) {
			if strings.Contains(field, "/mihomo") {
				idx := strings.Index(trimmed, field)
				if idx >= 0 {
					return strings.TrimSpace(trimmed[idx:])
				}
			}
		}
	}
	return ""
}

func extractConfigDirFromCommandLine(cmdLine string) string {
	parts := strings.Fields(cmdLine)
	for i, part := range parts {
		if part == "-d" && i+1 < len(parts) {
			return strings.Trim(parts[i+1], "\"")
		}
		if strings.HasPrefix(part, "-d") {
			return strings.TrimPrefix(part, "-d")
		}
	}
	execPath := parts[0]
	if absPath, err := filepath.Abs(execPath); err == nil {
		return filepath.Dir(absPath)
	}
	return filepath.Dir(execPath)
}

func findConfigFileInDirectory(dir string) string {
	configFile := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(configFile); err == nil {
		return configFile
	}
	configFile = filepath.Join(dir, "config.yml")
	if _, err := os.Stat(configFile); err == nil {
		return configFile
	}
	return ""
}

// clashVergeIdentifiers 为常见的 Clash Verge（基于 Tauri）应用标识符。
// Tauri 把运行期配置目录放在按 identifier 命名的子目录下，verge-mihomo 启动时
// 即用该目录作为 -d（配置根），其下的 config.yaml 才是热重载真正读取的文件。
// 包含现行 fork（clash-verge-rev）与历史上游（zzzgydi/clash-verge）两种 identifier。
var clashVergeIdentifiers = []string{
	"io.github.clash-verge-rev.clash-verge-rev",
	"io.github.zzzgydi.clash-verge",
}

func searchKnownMihomoDirectories() string {
	home, err := os.UserHomeDir()
	if err == nil {
		knownDirs := []string{
			filepath.Join(home, ".config", "mihomo"),
			filepath.Join(home, ".config", "clash"),
			filepath.Join(home, ".mihomo"),
			filepath.Join(home, ".clash"),
		}
		// Clash Verge (Linux/类 Unix)：~/.config/<identifier>
		for _, id := range clashVergeIdentifiers {
			knownDirs = append(knownDirs, filepath.Join(home, ".config", id))
		}
		// Clash Verge (macOS)：~/Library/Application Support/<identifier>
		if runtime.GOOS == "darwin" {
			for _, id := range clashVergeIdentifiers {
				knownDirs = append(knownDirs, filepath.Join(home, "Library", "Application Support", id))
			}
		}
		for _, dir := range knownDirs {
			if configFile := findConfigFileInDirectory(dir); configFile != "" {
				return configFile
			}
		}
	}

	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			knownDirs := []string{
				filepath.Join(appData, "mihomo"),
				filepath.Join(appData, "clash"),
			}
			// Clash Verge (Windows)：%APPDATA%\<identifier>
			for _, id := range clashVergeIdentifiers {
				knownDirs = append(knownDirs, filepath.Join(appData, id))
			}
			for _, dir := range knownDirs {
				if configFile := findConfigFileInDirectory(dir); configFile != "" {
					return configFile
				}
			}
		}
	}

	return ""
}

func searchFallbackDirectories() string {
	home, err := os.UserHomeDir()
	if err == nil {
		fallbackDirs := []string{
			filepath.Join(home, ".config", "mihomo"),
			filepath.Join(home, ".mihomo"),
		}
		for _, dir := range fallbackDirs {
			if configFile := findConfigFileInDirectory(dir); configFile != "" {
				return configFile
			}
		}
	}

	systemFallbackDirs := []string{
		"/etc/mihomo",
		"/usr/local/etc/mihomo",
	}

	for _, dir := range systemFallbackDirs {
		if configFile := findConfigFileInDirectory(dir); configFile != "" {
			return configFile
		}
	}

	return ""
}

func buildMihomoConfigPathNotFoundError() error {
	return errors.New(i18n.T("config.loader.err_manual_hint"))
}

// Load 加载配置文件
func Load() (*Config, error) {
	mu.Lock()
	defer mu.Unlock()

	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	configFile := filepath.Join(configDir, "config.yaml")

	// 配置文件不存在时返回错误，由调用方决定是否触发初始化引导
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, ErrConfigNotFound
	}

	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	// 预注册默认值，防止 Unmarshal 将 YAML 缺失字段重置为零值
	viper.SetDefault("test_url", DefaultConfig.TestURL)
	viper.SetDefault("timeout", DefaultConfig.Timeout)
	viper.SetDefault("language", DefaultConfig.Language)
	viper.SetDefault("auto_refresh_interval", DefaultConfig.AutoRefreshInterval)
	viper.SetDefault("theme", DefaultConfig.Theme)
	// 订阅：subs 默认空列表（旧配置文件无此键时保持空），active_sub 默认空（未启用）
	viper.SetDefault("subs", []profile.Profile{})
	viper.SetDefault("active_sub", "")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
