package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
)

// ErrInvalidConfigValue is returned when a configuration key or value fails validation.
var ErrInvalidConfigValue = errors.New("invalid configuration value")

// ConfigService 配置管理服务
type ConfigService struct {
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

// SetConfigValue 设置单个配置项。
// 连接信息（external-controller/secret/mixed-port）已迁移至 mihomo 配置文件，
// 不再通过本方法管理；请用 TUI 的 Mihomo 标签页或 config edit 编辑 mihomo 配置。
func (s *ConfigService) SetConfigValue(key, value string) error {
	return config.Update(func(cfg *config.Config) error {
		switch key {
		case "test_url", "test-url":
			cfg.TestURL = value
		case "timeout":
			timeout, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return fmt.Errorf("%w: timeout 必须是数字: %v", ErrInvalidConfigValue, err)
			}
			if timeout <= 0 {
				return fmt.Errorf("%w: timeout 必须大于 0", ErrInvalidConfigValue)
			}
			cfg.Timeout = timeout
		case "language":
			if value != "auto" && value != "zh-CN" && value != "en-US" {
				return fmt.Errorf("%w: language 必须是 auto, zh-CN 或 en-US", ErrInvalidConfigValue)
			}
			cfg.Language = value
		case "auto_refresh_interval", "auto-refresh-interval":
			interval, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return fmt.Errorf("%w: auto_refresh_interval 必须是数字: %v", ErrInvalidConfigValue, err)
			}
			if interval < 0 {
				return fmt.Errorf("%w: auto_refresh_interval 不能小于 0", ErrInvalidConfigValue)
			}
			cfg.AutoRefreshInterval = interval
		case "theme":
			if !model.IsValidTheme(value) {
				return fmt.Errorf("%w: theme 必须是: %s", ErrInvalidConfigValue, strings.Join(model.ThemeNames(), ", "))
			}
			cfg.Theme = value
		case "border_style", "border-style":
			if value != "rounded" && value != "square" {
				return fmt.Errorf("%w: border-style must be rounded or square", ErrInvalidConfigValue)
			}
			cfg.BorderStyle = value
		default:
			return fmt.Errorf("%w: 未知的配置项: %s (可用: test_url, timeout, language, auto_refresh_interval, theme, border_style)", ErrInvalidConfigValue, key)
		}

		return nil
	})
}

// ReadMihomoConfig reads the managed fields from disk, retaining endpoint/default fallbacks.
func (s *ConfigService) ReadMihomoConfig() (*model.MihomoConfig, MihomoTarget) {
	cfg, _ := config.Load()
	target := config.ResolveMihomoTarget(cfg)
	endpoint := target.Endpoint
	return &model.MihomoConfig{
		ExternalController: endpoint.ExternalController,
		Secret:             endpoint.Secret,
		MixedPort:          endpoint.MixedPort,
		AllowLan:           target.AllowLan,
		LogLevel:           target.LogLevel,
	}, target
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
	managed := s.isManagedCore()
	if allowServiceFallback && !managed {
		restartErr := s.restartMihomo()
		if restartErr == nil {
			result.Applied = true
			return result
		}
		result.Err = fmt.Errorf("API reload and systemctl restart both failed: %w", errors.Join(reloadErr, restartErr))
		return result
	}
	result.Err = reloadErr
	return result
}
