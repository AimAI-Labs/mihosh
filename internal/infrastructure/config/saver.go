package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Save 保存配置文件
func Save(cfg *Config) error {
	mu.Lock()
	defer mu.Unlock()
	unlock, err := lockAppConfig()
	if err != nil {
		return err
	}
	defer unlock()
	return saveLocked(cfg)
}

func saveLocked(cfg *Config) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	// 确保配置目录存在
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	configFile := filepath.Join(configDir, "config.yaml")
	writer := viper.New()
	writer.SetConfigFile(configFile)
	writer.SetConfigType("yaml")
	if _, err := os.Stat(configFile); err == nil {
		if err = writer.ReadInConfig(); err != nil {
			return err
		}
	}

	writer.Set("test_url", cfg.TestURL)
	writer.Set("timeout", cfg.Timeout)
	writer.Set("language", cfg.Language)
	writer.Set("auto_refresh_interval", cfg.AutoRefreshInterval)
	writer.Set("theme", cfg.Theme)
	// 订阅管理字段：subs 为订阅元数据列表，active_sub 为当前激活 UID
	if cfg.Subs == nil {
		writer.Set("subs", []interface{}{})
	} else {
		writer.Set("subs", cfg.Subs)
	}
	writer.Set("active_sub", cfg.ActiveSub)
	writer.Set("core", cfg.Core)

	writer.SetConfigFile(configFile)
	writer.SetConfigType("yaml")
	file, err := os.CreateTemp(configDir, ".mihosh-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = writer.WriteConfigTo(file); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), configFile)
}
