package service

import (
	"fmt"
	"log"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/ui/theme"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
	tea "github.com/charmbracelet/bubbletea"
)

// ConfigService 配置管理服务
type ConfigService struct{}

// NewConfigService 创建配置服务
func NewConfigService() *ConfigService {
	return &ConfigService{}
}

// LoadConfig 加载配置
func (s *ConfigService) LoadConfig() (*config.Config, error) {
	return config.Load()
}

// SaveConfig 保存配置
func (s *ConfigService) SaveConfig(cfg *config.Config) error {
	return config.Save(cfg)
}

// InitConfig 初始化配置（交互式）
func (s *ConfigService) InitConfig() error {
	fmt.Println("欢迎使用 Mihosh!")
	fmt.Println("首次使用，需要进行配置初始化")
	fmt.Println()

	cfg := config.DefaultConfig

	fmt.Printf("请输入 Mihomo API 地址 [默认: %s]: ", config.DefaultConfig.APIAddress)
	var input string
	fmt.Scanln(&input)
	if input != "" {
		cfg.APIAddress = input
	}

	fmt.Printf("请输入 API 密钥 (Secret) [可选]: ")
	fmt.Scanln(&input)
	if input != "" {
		cfg.Secret = input
	}

	fmt.Printf("请输入测速 URL [默认: %s]: ", config.DefaultConfig.TestURL)
	fmt.Scanln(&input)
	if input != "" {
		cfg.TestURL = input
	}

	if err := config.Save(&cfg); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("✓ 配置初始化完成!")
	return nil
}

// SetConfigValue 设置单个配置项
func (s *ConfigService) SetConfigValue(key, value string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch key {
	case "api_address", "api-address":
		cfg.APIAddress = value
	case "secret":
		cfg.Secret = value
	case "test_url", "test-url":
		cfg.TestURL = value
	case "timeout":
		var timeout int
		if _, err := fmt.Sscanf(value, "%d", &timeout); err != nil {
			return fmt.Errorf("timeout 必须是数字: %v", err)
		}
		cfg.Timeout = timeout
	case "proxy_address", "proxy-address":
		cfg.ProxyAddress = value
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
	default:
		return fmt.Errorf("未知的配置项: %s (可用: api_address, secret, test_url, timeout, proxy_address, language, auto_refresh_interval, theme)", key)
	}

	return config.Save(cfg)
}

// FetchMihomoConfig fetches the live config from Mihomo API and supplements with YAML if needed
func (s *ConfigService) FetchMihomoConfig(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return messages.MihomoConfigMsg{Err: fmt.Errorf("API client is nil")}
		}
		resp, err := client.GetConfigs()
		if err != nil {
			return messages.MihomoConfigMsg{Err: err}
		}

		mihomoCfg := &model.MihomoConfig{
			ExternalController: resp.ExternalController,
			Secret:             resp.Secret,
			MixedPort:          resp.MixedPort,
			AllowLan:           resp.AllowLan,
			LogLevel:           resp.LogLevel,
		}

		// Fallback for secret if empty (API might omit it)
		if mihomoCfg.Secret == "" {
			if path, err := config.GetMihomoConfigPath(); err == nil {
				if yamlData, err := config.ReadMihomoYAML(path); err == nil {
					if secret, ok := yamlData["secret"].(string); ok {
						mihomoCfg.Secret = secret
					}
				} else {
					log.Printf("Fallback: failed to read mihomo yaml: %v", err)
				}
			} else {
				log.Printf("Fallback: failed to get mihomo config path: %v", err)
			}
		}
		return messages.MihomoConfigMsg{Config: mihomoCfg}
	}
}

// SaveMihomoConfigField writes to YAML and reloads the API
func (s *ConfigService) SaveMihomoConfigField(client *api.Client, key string, value interface{}) tea.Cmd {
	return func() tea.Msg {
		path, err := config.GetMihomoConfigPath()
		if err != nil {
			return messages.MihomoConfigSavedMsg{Err: err}
		}

		if err := config.WriteMihomoField(path, key, value); err != nil {
			return messages.MihomoConfigSavedMsg{Err: fmt.Errorf("write yaml failed: %w", err)}
		}

		if client != nil {
			if err := client.ReloadConfig(path); err != nil {
				return messages.MihomoConfigSavedMsg{Err: fmt.Errorf("reload failed: %w", err)}
			}
		}
		
		// If external-controller or secret changed, we also need to update mihosh's own config.
		// The update loop will handle this when it receives the saved msg if we sync here,
		// or we can just update our local config directly since we are the config service.
		if key == "external-controller" {
			if strVal, ok := value.(string); ok {
				if err := s.SetConfigValue("api_address", strVal); err != nil {
					return messages.MihomoConfigSavedMsg{Err: fmt.Errorf("update local api_address failed: %w", err)}
				}
			} else {
				return messages.MihomoConfigSavedMsg{Err: fmt.Errorf("external-controller must be a string")}
			}
		} else if key == "secret" {
			if strVal, ok := value.(string); ok {
				if err := s.SetConfigValue("secret", strVal); err != nil {
					return messages.MihomoConfigSavedMsg{Err: fmt.Errorf("update local secret failed: %w", err)}
				}
			} else {
				return messages.MihomoConfigSavedMsg{Err: fmt.Errorf("secret must be a string")}
			}
		}

		return messages.MihomoConfigSavedMsg{}
	}
}
