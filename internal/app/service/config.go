package service

import (
	"fmt"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
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
	default:
		return fmt.Errorf("未知的配置项: %s (可用: api_address, secret, test_url, timeout, proxy_address)", key)
	}

	return config.Save(cfg)
}
