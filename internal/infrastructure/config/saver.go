package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Save 保存配置文件
func Save(cfg *Config) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	// 确保配置目录存在
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	configFile := filepath.Join(configDir, "config.yaml")

	viper.Set("api_address", cfg.APIAddress)
	viper.Set("secret", cfg.Secret)
	viper.Set("test_url", cfg.TestURL)
	viper.Set("timeout", cfg.Timeout)
	viper.Set("proxy_address", cfg.ProxyAddress)
	viper.Set("language", cfg.Language)
	viper.Set("auto_refresh_interval", cfg.AutoRefreshInterval)
	viper.Set("theme", cfg.Theme)
	// 订阅管理字段：subs 为订阅元数据列表，active_sub 为当前激活 UID
	if cfg.Subs == nil {
		viper.Set("subs", []interface{}{})
	} else {
		viper.Set("subs", cfg.Subs)
	}
	viper.Set("active_sub", cfg.ActiveSub)

	return viper.WriteConfigAs(configFile)
}
