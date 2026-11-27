package config

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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

// ps only selects candidate PIDs. Read each candidate's original argv separately.
var processListRunner = func() ([]byte, error) {
	return runMihomoDiscovery("ps", "-Aww", "-o", "pid=,args=")
}

var processArgumentsReader = readProcessArguments

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
// Linux 优先查找 systemd 服务，Linux 和 macOS 都可从运行进程发现配置，
// 找不到时再搜索已知目录。
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
	// 优先从运行中的 mihomo 进程解析实际配置路径。
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
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
	var hasServiceCommand bool
	if runtime.GOOS == "linux" {
		var configFile string
		configFile, hasServiceCommand = mihomoConfigPathFromSystemd()
		if configFile != "" {
			return configFile, nil
		}
	}
	if configFile := mihomoConfigPathFromRunningProcess(); configFile != "" {
		return configFile, nil
	}
	if hasServiceCommand {
		if configFile := searchFallbackDirectories(); configFile != "" {
			return configFile, nil
		}
	}
	return "", buildMihomoConfigPathNotFoundError()
}

func mihomoConfigPathFromSystemd() (string, bool) {
	output, err := systemctlStatusRunner()
	if err != nil {
		return "", false
	}
	status := string(output)
	var args []string
	var cwd string
	for _, line := range strings.Split(status, "\n") {
		mainPID, ok := strings.CutPrefix(strings.TrimSpace(line), "Main PID:")
		if !ok {
			continue
		}
		fields := strings.Fields(mainPID)
		if len(fields) > 0 {
			if pid, err := strconv.Atoi(fields[0]); err == nil && pid > 0 {
				args, cwd = processArgumentsReader(pid)
			}
		}
		break
	}
	if len(args) == 0 || !isMihomoExecutable(args[0]) {
		args, cwd = splitSystemdCommand(parseSystemctlStatus(status)), ""
	}
	if len(args) == 0 {
		return "", false
	}
	configFile, configDir := parseMihomoProcessArgs(args[1:])
	if configFile == "" && configDir == "" {
		configDir = filepath.Dir(args[0])
	}
	return resolveProcessConfigPath(configFile, configDir, cwd), true
}

func mihomoConfigPathFromRunningProcess() string {
	output, err := processListRunner()
	if err != nil {
		return ""
	}
	return parseMihomoConfigFromProcessList(string(output))
}

// parseMihomoConfigFromProcessList reads original argv for candidate Mihomo PIDs.
func parseMihomoConfigFromProcessList(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		end := strings.IndexAny(line, " \t")
		if end < 0 || !strings.Contains(line[end:], "mihomo") {
			continue
		}
		pid, err := strconv.Atoi(line[:end])
		if err != nil || pid <= 0 {
			continue
		}
		args, cwd := processArgumentsReader(pid)
		if len(args) == 0 || !isMihomoExecutable(args[0]) {
			continue
		}
		file, dir := parseMihomoProcessArgs(args[1:])
		if configFile := resolveProcessConfigPath(file, dir, cwd); configFile != "" {
			return configFile
		}
	}
	return ""
}

func resolveProcessConfigPath(file, dir, cwd string) string {
	if file = resolveProcessPath(file, cwd); file != "" {
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			return file
		}
	}
	if dir = resolveProcessPath(dir, cwd); dir != "" {
		return findConfigFileInDirectory(dir)
	}
	return ""
}

func resolveProcessPath(path, cwd string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if !filepath.IsAbs(cwd) {
		return ""
	}
	return filepath.Join(cwd, path)
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

		treeLine := strings.TrimLeft(trimmed, "│├└─ \t")
		if treeLine == trimmed {
			continue
		}
		end := strings.IndexAny(treeLine, " \t")
		if end < 0 {
			continue
		}
		if _, err := strconv.Atoi(treeLine[:end]); err == nil {
			cmd := strings.TrimSpace(treeLine[end:])
			if strings.Contains(cmd, "mihomo") {
				return cmd
			}
		}
	}
	return ""
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
