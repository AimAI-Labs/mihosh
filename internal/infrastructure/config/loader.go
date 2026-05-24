package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

// ErrConfigNotFound 配置文件不存在
var ErrConfigNotFound = errors.New("配置文件不存在")

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

// GetMihomoConfigPath 获取 mihomo 配置文件的默认路径
func GetMihomoConfigPath() (string, error) {
	if configFile := searchKnownMihomoDirectories(); configFile != "" {
		return configFile, nil
	}

	if runtime.GOOS != "windows" {
		if configFile, err := GetMihomoConfigPathFromProcess(); err == nil {
			return configFile, nil
		}
	}

	return "", buildMihomoConfigPathNotFoundError()
}

func GetMihomoConfigPathFromProcess() (string, error) {
	output, err := systemctlStatusRunner()
	if err != nil {
		return "", fmt.Errorf("无法获取 mihomo 服务状态: %w", err)
	}

	cmdLine := parseSystemctlStatus(string(output))
	if cmdLine == "" {
		return "", errors.New("无法从 systemctl status 输出中解析 mihomo 命令行")
	}

	configDir := extractConfigDirFromCommandLine(cmdLine)
	if configDir == "" {
		return "", errors.New("无法从命令行中提取配置目录（未找到 -d 标志）")
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

func searchKnownMihomoDirectories() string {
	home, err := os.UserHomeDir()
	if err == nil {
		knownDirs := []string{
			filepath.Join(home, ".config", "mihomo"),
			filepath.Join(home, ".config", "clash"),
			filepath.Join(home, ".mihomo"),
			filepath.Join(home, ".clash"),
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
	return errors.New(
		"未找到 mihomo 配置文件，请手动指定。\n" +
			"降级处理：\n" +
			"1. 运行 `sudo systemctl status mihomo`\n" +
			"2. 查找 `-d` 后面的目录参数\n" +
			"3. 在该目录下确认 `config.yaml` 或 `config.yml`\n" +
			"4. 使用 `mihosh config edit --path <配置文件或目录>` 手动指定",
	)
}

// Load 加载配置文件
func Load() (*Config, error) {
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
	viper.SetDefault("api_address", DefaultConfig.APIAddress)
	viper.SetDefault("secret", DefaultConfig.Secret)
	viper.SetDefault("test_url", DefaultConfig.TestURL)
	viper.SetDefault("timeout", DefaultConfig.Timeout)
	viper.SetDefault("proxy_address", DefaultConfig.ProxyAddress)
	viper.SetDefault("language", DefaultConfig.Language)
	viper.SetDefault("auto_refresh_interval", DefaultConfig.AutoRefreshInterval)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
