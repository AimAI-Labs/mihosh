package service

import (
	"fmt"
	"strings"
	"sync"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
)

// ConfigService 配置管理服务
type ConfigService struct {
	mergeMu           sync.Mutex
	mihomoConfigPath  func() (string, error)
	localExternalPath func() (string, error)
	writeMihomoField  func(string, string, interface{}) error
	restartMihomo     func() error
}

// MihomoConfigSaveResult separates persisted data from runtime application.
type MihomoConfigSaveResult struct {
	WriteOK bool
	Applied bool
	Err     error
}

// NewConfigService 创建配置服务
func NewConfigService() *ConfigService {
	return &ConfigService{
		mihomoConfigPath:  config.GetMihomoConfigPathForWrite,
		localExternalPath: config.GetMihomoConfigPath,
		writeMihomoField:  config.WriteMihomoField,
		restartMihomo:     config.RestartMihomoService,
	}
}

// LoadConfig 加载配置
func (s *ConfigService) LoadConfig() (*config.Config, error) {
	return config.Load()
}

// SaveConfig 保存配置
func (s *ConfigService) SaveConfig(cfg *config.Config) error {
	return config.Save(cfg)
}

// SetConfigValue 设置单个配置项。
// 连接信息（external-controller/secret/mixed-port）已迁移至 mihomo 配置文件，
// 不再通过本方法管理；请用 TUI 的 Mihomo 标签页或 config edit 编辑 mihomo 配置。
func (s *ConfigService) SetConfigValue(key, value string) error {
	return config.Update(func(cfg *config.Config) error {
		switch key {
		case "test_url", "test-url":
			cfg.TestURL = value
		case "timeout":
			var timeout int
			if _, err := fmt.Sscanf(value, "%d", &timeout); err != nil {
				return fmt.Errorf("timeout 必须是数字: %v", err)
			}
			cfg.Timeout = timeout
		case "language":
			if value != "auto" && value != "zh-CN" && value != "en-US" {
				return fmt.Errorf("language 必须是 auto, zh-CN 或 en-US")
			}
			cfg.Language = value
		case "auto_refresh_interval", "auto-refresh-interval":
			var interval int
			if _, err := fmt.Sscanf(value, "%d", &interval); err != nil {
				return fmt.Errorf("auto_refresh_interval 必须是数字: %v", err)
			}
			if interval < 0 {
				return fmt.Errorf("auto_refresh_interval 不能小于 0")
			}
			cfg.AutoRefreshInterval = interval
		case "theme":
			if !theme.IsValid(value) {
				return fmt.Errorf("theme 必须是: %s", strings.Join(theme.Names(), ", "))
			}
			cfg.Theme = value
		case "border_style", "border-style":
			if value != "rounded" && value != "square" {
				return fmt.Errorf("border-style must be rounded or square")
			}
			cfg.BorderStyle = value
		default:
			return fmt.Errorf("未知的配置项: %s (可用: test_url, timeout, language, auto_refresh_interval, theme, border_style)", key)
		}

		return nil
	})
}

// ReadMihomoConfig reads the managed fields from disk, retaining endpoint/default fallbacks.
func (s *ConfigService) ReadMihomoConfig() *model.MihomoConfig {
	endpoint := config.ResolveMihomoEndpoint()
	mihomoCfg := &model.MihomoConfig{
		ExternalController: endpoint.ExternalController,
		Secret:             endpoint.Secret,
		MixedPort:          endpoint.MixedPort,
	}

	// 从 YAML 读取 allow-lan / log-level（ResolveMihomoEndpoint 未解析的字段）
	if path, err := config.GetMihomoConfigPath(); err == nil {
		if data, err := config.ReadMihomoYAML(path); err == nil {
			if v, ok := data["allow-lan"].(bool); ok {
				mihomoCfg.AllowLan = v
			}
			if v, ok := data["log-level"].(string); ok {
				mihomoCfg.LogLevel = v
			}
		}
	}

	return mihomoCfg
}

// SaveMihomoConfigField writes YAML, then hot-reloads via API before any service restart fallback.
func (s *ConfigService) SaveMihomoConfigField(client *api.Client, key string, value interface{}) MihomoConfigSaveResult {
	path, err := s.mihomoConfigPath()
	if err != nil {
		return MihomoConfigSaveResult{Err: err}
	}
	if err := s.writeMihomoField(path, key, value); err != nil {
		return MihomoConfigSaveResult{Err: fmt.Errorf("write yaml failed: %w", err)}
	}
	return s.applyMihomoConfig(client, path, true)
}

// ReloadMihomoConfig applies an already-written file using the CLI's API-only policy.
func (s *ConfigService) ReloadMihomoConfig(client *api.Client, path string) MihomoConfigSaveResult {
	return s.applyMihomoConfig(client, path, false)
}

func (s *ConfigService) isManagedCore() bool {
	cfg, err := config.Load()
	return err == nil && cfg.Core.Mode == "managed"
}

func (s *ConfigService) applyMihomoConfig(client *api.Client, path string, allowServiceFallback bool) MihomoConfigSaveResult {
	result := MihomoConfigSaveResult{WriteOK: true}
	// An offline save retains the existing file-only success semantics without invoking sudo.
	if client == nil {
		return result
	}
	reloadErr := client.ReloadConfig(path)
	if reloadErr == nil {
		result.Applied = true
		return result
	}
	if allowServiceFallback && !s.isManagedCore() && s.restartMihomo() == nil {
		result.Applied = true
		return result
	}
	result.Err = reloadErr
	if allowServiceFallback && !s.isManagedCore() {
		result.Err = fmt.Errorf("API reload and systemctl restart both failed: %w", reloadErr)
	}
	return result
}
