package config

import (
	"io"
	"os"
	"path/filepath"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/fsutil"
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

	return writeConfigFile(cfg, configDir, filepath.Join(configDir, "config.yaml"))
}

func writeConfigFile(cfg *Config, configDir, destination string) error {
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
	writer.Set("border_style", cfg.BorderStyle)
	writer.Set("shell_proxy", cfg.ShellProxy)
	if cfg.FileDiffOptions != nil {
		writer.Set("file_diff_options", *cfg.FileDiffOptions)
	} else if writer.IsSet("file_diff_options") {
		// A typed nil writes YAML null instead of falling back to the old value.
		writer.Set("file_diff_options", cfg.FileDiffOptions)
	}
	// 订阅管理字段：subs 为订阅元数据列表，active_sub 为当前激活 UID
	if cfg.Subs == nil {
		writer.Set("subs", []interface{}{})
	} else {
		writer.Set("subs", cfg.Subs)
	}
	// Older clients replace subs on save, but round-trip unknown top-level keys.
	if records := subscriptionUsageRecords(cfg.Subs); len(records) > 0 || writer.IsSet(subscriptionUsageKey) {
		writer.Set(subscriptionUsageKey, records)
	}
	writer.Set("active_sub", cfg.ActiveSub)
	writer.Set("core", cfg.Core)
	if cfg.Sync != (SyncConfig{}) || writer.IsSet("sync") {
		writer.Set("sync", cfg.Sync)
	}

	return writeConfigAtomically(destination, 0600, writer.WriteConfigTo)
}

func writeConfigAtomically(path string, mode os.FileMode, write func(io.Writer) error) error {
	return fsutil.WriteStreamAtomic(path, fsutil.WriteOptions{
		Perm:          mode,
		DirPerm:       0755,
		EnsureDir:     true,
		FollowSymlink: true,
		TempPattern:   ".mihosh-config-*",
	}, write)
}
