package config

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

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
	return runMihomoDiscovery("systemctl", "status", "mihomo")
}

// processListRunner 列出系统进程（pid + 完整命令行），用于发现手动启动的 mihomo。
// -eww 保证长命令行不被终端宽度截断。
var processListRunner = func() ([]byte, error) {
	return runMihomoDiscovery("ps", "-eww", "-o", "pid=,args=")
}

func runMihomoDiscovery(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 200 * time.Millisecond
	return cmd.CombinedOutput()
}

// GetConfigDir 获取配置目录
func GetConfigDir() (string, error) {
	if root := os.Getenv("MIHOSH_CONFIG_DIR"); root != "" {
		if !filepath.IsAbs(root) {
			return "", errors.New("MIHOSH_CONFIG_DIR must be an absolute path")
		}
		return filepath.Clean(root), nil
	}
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
	cfg, _ := Load()
	return resolveMihomoConfigPath(cfg)
}

func resolveMihomoConfigPath(cfg *Config) (string, error) {
	if cfg != nil {
		if cfg.Core.Mode == "managed" {
			return ManagedMihomoConfigPath()
		}
		if cfg.Core.ExternalController != "" {
			return "", ErrExternalConfig
		}
	}
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
	} else if errors.Is(err, ErrExternalConfig) {
		return "", err
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

// GetMihomoConfigPathFromProcess 从运行中的 mihomo 进程定位配置文件。
// 先查 systemd 服务（系统服务场景），再扫描进程命令行（手动启动场景）。
func GetMihomoConfigPathFromProcess() (string, error) {
	if configFile := mihomoConfigPathFromSystemd(); configFile != "" {
		return configFile, nil
	}
	if configFile := mihomoConfigPathFromRunningProcess(); configFile != "" {
		return configFile, nil
	}
	return "", buildMihomoConfigPathNotFoundError()
}

func mihomoConfigPathFromSystemd() string {
	output, err := systemctlStatusRunner()
	if err != nil {
		return ""
	}

	cmdLine := parseSystemctlStatus(string(output))
	if cmdLine == "" {
		return ""
	}

	configDir := extractConfigDirFromCommandLine(cmdLine)
	if configDir == "" {
		return ""
	}

	if configFile := findConfigFileInDirectory(configDir); configFile != "" {
		return configFile
	}
	return searchFallbackDirectories()
}

func mihomoConfigPathFromRunningProcess() string {
	output, err := processListRunner()
	if err != nil {
		return ""
	}
	return parseMihomoConfigFromProcessList(string(output))
}

// parseMihomoConfigFromProcessList 从 ps 输出中查找 mihomo 进程的启动参数。
// 优先采用 -f 指定的配置文件；未指定时用 -d 目录下的 config.yaml。
// 手动启动（非 systemd）的 mihomo 只能通过命令行发现。
func parseMihomoConfigFromProcessList(output string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		pid, executable := fields[0], fields[1]
		if !strings.Contains(filepath.Base(executable), "mihomo") {
			continue
		}

		configFile, configDir := parseMihomoProcessArgs(fields[2:])
		if configFile != "" {
			if !filepath.IsAbs(configFile) {
				if cwd, err := os.Readlink(filepath.Join("/proc", pid, "cwd")); err == nil {
					configFile = filepath.Join(cwd, configFile)
				}
			}
			if info, err := os.Stat(configFile); err == nil && !info.IsDir() {
				return configFile
			}
		}
		if configDir != "" {
			if configFile := findConfigFileInDirectory(configDir); configFile != "" {
				return configFile
			}
		}
	}
	return ""
}

// parseMihomoProcessArgs 解析 mihomo 的 -f/-d 参数，兼容 -f path 与 -f=path 两种写法。
func parseMihomoProcessArgs(args []string) (configFile, configDir string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--f":
			if i+1 < len(args) {
				configFile = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "-f=") || strings.HasPrefix(arg, "--f="):
			configFile = arg[strings.Index(arg, "=")+1:]
		case arg == "-d" || arg == "--d":
			if i+1 < len(args) {
				configDir = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "-d=") || strings.HasPrefix(arg, "--d="):
			configDir = arg[strings.Index(arg, "=")+1:]
		}
	}
	return configFile, configDir
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
	root, err := GetConfigDir()
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(filepath.Join(root, profileRestoreDir)); !os.IsNotExist(err) {
		unlock, err := lockAppConfig()
		if err != nil {
			return nil, err
		}
		defer unlock()
	}
	return loadLocked()
}

func loadOrDefaultLocked() (*Config, error) {
	cfg, err := loadLocked()
	if errors.Is(err, ErrConfigNotFound) {
		defaults := DefaultConfig
		return &defaults, nil
	}
	return cfg, err
}

func loadLocked() (*Config, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	configFile := filepath.Join(configDir, "config.yaml")

	// 配置文件不存在时返回错误，由调用方决定是否触发初始化引导
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, ErrConfigNotFound
	}

	reader := viper.New()
	reader.SetConfigFile(configFile)
	reader.SetConfigType("yaml")

	// 预注册默认值，防止 Unmarshal 将 YAML 缺失字段重置为零值
	reader.SetDefault("test_url", DefaultConfig.TestURL)
	reader.SetDefault("timeout", DefaultConfig.Timeout)
	reader.SetDefault("language", DefaultConfig.Language)
	reader.SetDefault("auto_refresh_interval", DefaultConfig.AutoRefreshInterval)
	reader.SetDefault("theme", DefaultConfig.Theme)
	reader.SetDefault("border_style", DefaultConfig.BorderStyle)
	// 订阅：subs 默认空列表（旧配置文件无此键时保持空），active_sub 默认空（未启用）
	reader.SetDefault("subs", []profile.Profile{})
	reader.SetDefault("active_sub", "")

	if err := reader.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := reader.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	restoreSubscriptionUsage(reader, &cfg)
	return &cfg, nil
}
