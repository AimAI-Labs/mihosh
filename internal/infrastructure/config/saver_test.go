package config

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSavePersistsTestURL 验证 test_url 经 Save→磁盘后保持完整。
// （取代已删除的 TestSavePersistsProxyAddress：proxy_address 已迁移至 mihomo 配置文件。）
func TestSavePersistsTestURL(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
	})
	viper.Reset()

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cfg := &Config{
		TestURL: "http://www.gstatic.com/generate_204",
		Timeout: 5000,
	}

	err := Save(cfg)
	require.NoError(t, err, "Save() returned error")

	configFile := filepath.Join(tempHome, ".mihosh", "config.yaml")
	reader := viper.New()
	reader.SetConfigFile(configFile)
	reader.SetConfigType("yaml")

	err = reader.ReadInConfig()
	require.NoError(t, err, "ReadInConfig() returned error")

	got := reader.GetString("test_url")
	assert.Equal(t, cfg.TestURL, got, "test_url not persisted")
}

func TestSavePersistsLanguage(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
	})
	viper.Reset()

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cfg := &Config{
		TestURL: "http://www.gstatic.com/generate_204",
		Timeout: 5000,
		Language: "en-US",
	}

	err := Save(cfg)
	require.NoError(t, err, "Save() returned error")

	configFile := filepath.Join(tempHome, ".mihosh", "config.yaml")
	reader := viper.New()
	reader.SetConfigFile(configFile)
	reader.SetConfigType("yaml")

	err = reader.ReadInConfig()
	require.NoError(t, err, "ReadInConfig() returned error")

	got := reader.GetString("language")
	assert.Equal(t, cfg.Language, got, "language not persisted")
}

func TestSavePersistsAutoRefreshInterval(t *testing.T) {
	t.Cleanup(func() {
		viper.Reset()
	})
	viper.Reset()

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cfg := &Config{
		TestURL:             "http://www.gstatic.com/generate_204",
		Timeout:             5000,
		Language:            "en-US",
		AutoRefreshInterval: 9,
	}

	err := Save(cfg)
	require.NoError(t, err, "Save() returned error")

	configFile := filepath.Join(tempHome, ".mihosh", "config.yaml")
	reader := viper.New()
	reader.SetConfigFile(configFile)
	reader.SetConfigType("yaml")

	err = reader.ReadInConfig()
	require.NoError(t, err, "ReadInConfig() returned error")

	got := reader.GetInt("auto_refresh_interval")
	assert.Equal(t, cfg.AutoRefreshInterval, got, "auto_refresh_interval not persisted")
}
